// Command zhuwenctl is the factory CLI/TUI entry (handoff §4). CP-01 wires the walking
// skeleton: lexicon ingest, a full fixture-pack build, and pack verification.
package main

import (
	"fmt"
	"os"

	"github.com/parso/zhuwen-factory/internal/assets"
	"github.com/parso/zhuwen-factory/internal/brief"
	"github.com/parso/zhuwen-factory/internal/canon"
	"github.com/parso/zhuwen-factory/internal/fixtures"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/gen"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/minisign"
	"github.com/parso/zhuwen-factory/internal/pack"
	"github.com/parso/zhuwen-factory/internal/pipeline"
	"github.com/parso/zhuwen-factory/internal/repair"
	"github.com/parso/zhuwen-factory/internal/segment"
	"github.com/parso/zhuwen-factory/internal/spike"
	"github.com/parso/zhuwen-factory/internal/workq"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "lexicon":
		err = cmdLexicon(os.Args[2:])
	case "segment":
		err = cmdSegment(os.Args[2:])
	case "spike":
		err = cmdSpike(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "build":
		err = cmdBuild(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `zhuwenctl — Zhuwen content factory (CP-01)

usage:
  zhuwenctl lexicon                 ingest the fixture lexicon and report
  zhuwenctl lexicon ingest --src <dir|file> --out <lexicon.sqlite> --version <v>
                                    ingest operator-supplied HSK-3.0 TSV(s) -> lexicon.sqlite
                                    (real lists are license-gated; see plans/blockers.md B-1)
  zhuwenctl segment eval            report FMM token/type coverage + ambiguity over the canon
  zhuwenctl spike [--n <k>] [--live]
                                    run canon->brief->gen->segment->gate->repair (MC-2);
                                    --live uses the LLM (needs ZHUWEN_LLM_API_KEY), else fixtures
  zhuwenctl run --db <path> [--stage gen] [--resume]
                                    run the resumable SQLite work queue over the canon (MC-3);
                                    --resume recovers a crashed run without double-charging
  zhuwenctl build --out <pack>      run the pipeline and emit a signed .zpack
                    [--pub <file>]  also write the verify pubkey (default <pack>.pub)
                    [--key <file>]  sign with a minisign secret key
                    [--devkey]      sign with the reproducible DEV fixture key
  zhuwenctl verify <pack> --pub <f> verify signature + hashes + I6 + lexicon_version
  zhuwenctl keygen --out <prefix>   write <prefix>.pub / <prefix>.key (minisign)
`)
}

func cmdLexicon(args []string) error {
	if len(args) > 0 && args[0] == "ingest" {
		return cmdLexiconIngest(args[1:])
	}
	lex, err := assets.Lexicon()
	if err != nil {
		return err
	}
	fmt.Printf("lexicon %s: %d words, max id %d\n", lex.Version(), lex.Len(), lex.MaxID())
	return nil
}

func cmdLexiconIngest(args []string) error {
	src := flagValue(args, "--src")
	out := flagValue(args, "--out")
	version := flagValue(args, "--version")
	if src == "" || out == "" || version == "" {
		return fmt.Errorf("lexicon ingest: --src <dir|file>, --out <lexicon.sqlite>, --version <v> all required")
	}
	lex, err := lexicon.IngestDir(src, version)
	if err != nil {
		return err
	}
	if err := lexicon.WriteSQLite(lex, out); err != nil {
		return err
	}
	fmt.Printf("ingested %s: %d words (max id %d) -> %s\n", version, lex.Len(), lex.MaxID(), out)
	return nil
}

func flagValue(args []string, name string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// buildHarness assembles the fixture lexicon, canon registry, A2 band, segmenter, and gate
// checker shared by `segment eval` and `spike`. Everything is deterministic and offline.
func buildHarness() (*lexicon.Lexicon, *canon.Registry, brief.BandSpec, *segment.Segmenter, repair.PipelineChecker, error) {
	fail := func(err error) (*lexicon.Lexicon, *canon.Registry, brief.BandSpec, *segment.Segmenter, repair.PipelineChecker, error) {
		return nil, nil, brief.BandSpec{}, nil, repair.PipelineChecker{}, err
	}
	lex, err := assets.Lexicon()
	if err != nil {
		return fail(err)
	}
	reg, err := assets.Canon()
	if err != nil {
		return fail(err)
	}
	spec, err := pipeline.BuildFixtureBand(lex, assets.FrontierSimps())
	if err != nil {
		return fail(err)
	}
	seg := segment.New(lex.DictEntries(), nil)
	checker := repair.PipelineChecker{
		Seg:      seg,
		Band:     gate.Band{Known: spec.Known, Frontier: spec.Frontier, Grammar: spec.Grammar},
		Detector: grammar.MarkerDetector{},
		MaxID:    lex.MaxID(),
		Cfg:      gate.DefaultConfig(),
	}
	return lex, reg, spec, seg, checker, nil
}

// cmdSegment implements `segment eval`: generate the fixture stories, then report FMM
// coverage + ambiguity hotspots so the jieba-parity decision (MC-2.2) is data-driven.
func cmdSegment(args []string) error {
	if len(args) == 0 || args[0] != "eval" {
		return fmt.Errorf("segment: expected `segment eval`")
	}
	lex, reg, spec, seg, _, err := buildHarness()
	if err != nil {
		return err
	}
	provider := gen.NewFixtureProvider(lex, assets.FillerSimps())
	var texts []string
	for _, e := range reg.All() {
		b := brief.Compile(e, spec)
		story, err := provider.Retell(b)
		if err != nil {
			return err
		}
		texts = append(texts, story.Text)
	}
	rep := seg.Eval(texts, 20)
	fmt.Printf("segment eval (%s, %d stories):\n", lex.Version(), rep.Stories)
	fmt.Printf("  tokens: %d (word %d, proper %d, literal %d)\n",
		rep.TotalTokens, rep.WordTokens, rep.ProperTokens, rep.LiteralTokens)
	fmt.Printf("  distinct in-lexicon types: %d\n", rep.DistinctTypes)
	fmt.Printf("  token coverage: %.4f   literal (unresolved) rate: %.4f\n", rep.TokenCoverage, rep.LiteralRate)
	fmt.Printf("  ambiguity hotspots: %d\n", len(rep.Hotspots))
	for _, h := range rep.Hotspots {
		fmt.Printf("    story %d @rune %d: chose %q, overlaps %q\n", h.StoryIdx, h.Rune, h.Chosen, h.Overlap)
	}
	return nil
}

// cmdSpike implements `spike`: run the content-reality harness and print the metrics.
func cmdSpike(args []string) error {
	n := 5
	if v := flagValue(args, "--n"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}
	live := hasFlag(args, "--live")
	lexPath := flagValue(args, "--lexicon")

	// Band: default fixture harness, or the real HSK band when --lexicon is given.
	var lex *lexicon.Lexicon
	var reg *canon.Registry
	var spec brief.BandSpec
	var checker repair.PipelineChecker
	if lexPath != "" {
		var err error
		if lex, err = lexicon.ReadSQLite(lexPath); err != nil {
			return err
		}
		if reg, err = assets.Canon(); err != nil {
			return err
		}
		knownMax, frontier := 2, 3
		if v := flagValue(args, "--known-max"); v != "" {
			fmt.Sscanf(v, "%d", &knownMax)
		}
		if v := flagValue(args, "--frontier-level"); v != "" {
			fmt.Sscanf(v, "%d", &frontier)
		}
		spec = pipeline.BuildHSKBand(lex, "A2", knownMax, frontier, knownMax)
		seg := segment.New(lex.DictEntries(), nil)
		checker = repair.PipelineChecker{
			Seg:      seg,
			Band:     gate.Band{Known: spec.Known, Frontier: spec.Frontier, Grammar: spec.Grammar},
			Detector: grammar.MarkerDetector{},
			MaxID:    lex.MaxID(),
			Cfg:      gate.DefaultConfig(),
		}
		fmt.Printf("spike: real HSK lexicon %s (%d words); known HSK<=%d (%d), frontier HSK %d (%d)\n",
			lex.Version(), lex.Len(), knownMax, len(spec.Known), frontier, len(spec.Frontier))
	} else {
		var err error
		if lex, reg, spec, _, checker, err = buildHarness(); err != nil {
			return err
		}
	}

	var provider gen.Provider
	var llm *gen.LLMProvider
	if live {
		cfg, ok := gen.LLMConfigFromEnv()
		if !ok {
			return fmt.Errorf("spike --live: no API key (set ZHUWEN_LLM_API_KEY or ~/.deepseek-api-key)")
		}
		llm = gen.NewLLMProvider(cfg, lex)
		provider = llm
		fmt.Printf("spike: LIVE run (model %s @ %s)\n", cfg.Model, cfg.BaseURL)
	} else {
		provider = gen.NewFixtureProvider(lex, assets.FillerSimps())
		fmt.Println("spike: fixture provider (deterministic; harness/mechanics only)")
	}

	sum := spike.Run(reg, spec, provider, checker, n)
	fmt.Printf("entries=%d  pass@0=%d (%.0f%%)  passed=%d  discarded=%d (%.0f%%)  mean-repair-iters=%.2f\n",
		sum.Entries, sum.PassAtIter0, 100*sum.PassRateAtIter0(),
		sum.Passed, sum.Discarded, 100*sum.DiscardRate(),
		sum.MeanRepairIterations())
	if llm != nil {
		perStory := 0
		if sum.Passed > 0 {
			perStory = llm.TokensUsed() / sum.Passed
		}
		fmt.Printf("tokens: %d total, ~%d per shipped story\n", llm.TokensUsed(), perStory)
	}
	if len(sum.FailureCodeHist) > 0 {
		fmt.Println("failure-code histogram:")
		for _, c := range sum.SortedFailureCodes() {
			fmt.Printf("  %-24s %d\n", c, sum.FailureCodeHist[c])
		}
	}
	if hasFlag(args, "--verbose") {
		for _, f := range sum.Fates {
			status := "DISCARDED"
			if f.Passed {
				status = "PASSED"
			}
			fmt.Printf("\n--- %s [%s] iters=%d ---\n", f.Brief.CanonID, status, f.Iterations)
			if len(f.Candidates) > 0 {
				last := f.Candidates[len(f.Candidates)-1].Text
				if len(last) > 220 {
					last = last[:220] + "…"
				}
				fmt.Printf("last candidate: %s\n", last)
			}
			for i, reasons := range f.FailReasons {
				if len(reasons) > 0 {
					fmt.Printf("iter %d fails: %v\n", i, reasons)
				}
			}
		}
	}
	return nil
}

// cmdRun implements `run`: drive the resumable SQLite work queue (MC-3). Each canon entry is a
// `gen` unit; processing calls the fixture provider (deterministic) and caches the result. A
// crash mid-stage (ZHUWEN_CRASH_AFTER=n, used by the kill-9 e2e) leaves the queue recoverable:
// re-running with --resume completes the rest without recomputing or double-charging.
func cmdRun(args []string) error {
	dbPath := flagValue(args, "--db")
	if dbPath == "" {
		return fmt.Errorf("run: --db <path> required")
	}
	stage := flagValue(args, "--stage")
	if stage == "" {
		stage = "gen"
	}
	resume := hasFlag(args, "--resume")

	lex, reg, spec, _, _, err := buildHarness()
	if err != nil {
		return err
	}
	provider := gen.NewFixtureProvider(lex, assets.FillerSimps())
	briefs := map[string]brief.Brief{}
	for _, e := range reg.All() {
		briefs[e.CanonID] = brief.Compile(e, spec)
	}

	q, err := workq.Open(dbPath)
	if err != nil {
		return err
	}
	defer q.Close()

	for ref := range briefs {
		if err := q.Enqueue(stage, ref); err != nil {
			return err
		}
	}
	if resume {
		if n, err := q.ResetStale(); err != nil {
			return err
		} else if n > 0 {
			fmt.Printf("run: recovered %d stale unit(s) from a prior crash\n", n)
		}
	}

	crashAfter := -1
	if v := os.Getenv("ZHUWEN_CRASH_AFTER"); v != "" {
		fmt.Sscanf(v, "%d", &crashAfter)
	}
	hook := func(processed int, ref string) {
		if crashAfter >= 0 && processed >= crashAfter {
			fmt.Printf("run: simulated crash after %d unit(s), mid-stage on %s\n", processed, ref)
			os.Exit(137) // 128 + SIGKILL(9)
		}
	}

	stageFn := func(q *workq.Queue, ref string) (string, int, error) {
		b, ok := briefs[ref]
		if !ok {
			return "", 0, fmt.Errorf("run: no brief for ref %q", ref)
		}
		// Idempotency key per brief+candidate: a retry after a crash records at most one charge
		// (models the upstream API's idempotency-key dedup), so kill -9 cannot double-charge.
		if _, err := q.Charge(stage + ":" + ref); err != nil {
			return "", 0, err
		}
		story, err := provider.Retell(b)
		if err != nil {
			return "", 0, err
		}
		return story.Text, 0, nil
	}

	if err := q.Process(stage, 4, stageFn, hook); err != nil {
		return err
	}
	summary, err := q.Summary(stage)
	if err != nil {
		return err
	}
	fmt.Println("run:", summary)
	return nil
}

// runFixturePipeline builds the CP-01 fixture pack (shared with genfixtures).
func runFixturePipeline() (*pack.Pack, error) {
	p, rejected, err := fixtures.BuildFixturePack()
	if err != nil {
		return nil, err
	}
	fmt.Printf("pipeline: %d stories packed, %d rejected\n", len(p.Stories), rejected)
	return p, nil
}

func cmdBuild(args []string) error {
	out := flagValue(args, "--out")
	if out == "" {
		return fmt.Errorf("build: --out <pack> required")
	}
	pubPath := flagValue(args, "--pub")
	if pubPath == "" {
		pubPath = out + ".pub"
	}
	pk, err := runFixturePipeline()
	if err != nil {
		return err
	}

	// Sign with a supplied minisign secret key, or a fresh ephemeral key.
	var pub minisign.PublicKey
	var priv minisign.PrivateKey
	if hasFlag(args, "--devkey") {
		// DEV-ONLY reproducible key from a public seed — signs vendored test fixtures
		// only, never production packs. Lets ios/Fixtures/ be regenerated deterministically.
		pub, priv = fixtures.DevKey()
	} else if keyPath := flagValue(args, "--key"); keyPath != "" {
		raw, err := os.ReadFile(keyPath)
		if err != nil {
			return err
		}
		if priv, err = minisign.ParseSecret(string(raw)); err != nil {
			return err
		}
		pub = priv.Public()
	} else {
		if pub, priv, err = minisign.GenerateKey(); err != nil {
			return err
		}
	}
	if err := pack.Build(pk, out, priv); err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, []byte(pub.Encode()), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d stories) + pubkey %s\n", out, len(pk.Stories), pubPath)
	return nil
}

func cmdVerify(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("verify: <pack> required")
	}
	packPath := args[0]
	pubPath := flagValue(args, "--pub")
	if pubPath == "" {
		return fmt.Errorf("verify: --pub <file> required")
	}
	raw, err := os.ReadFile(pubPath)
	if err != nil {
		return err
	}
	pub, err := minisign.ParsePublicKey(string(raw))
	if err != nil {
		return err
	}
	man, err := pack.Verify(packPath, pub, nil)
	if err != nil {
		return err
	}
	fmt.Printf("OK: pack %s v%s, lexicon %s, schema %d, %d files\n",
		man.ID, man.Semver, man.LexiconVersion, man.SchemaVersion, len(man.Files))
	return nil
}

func cmdKeygen(args []string) error {
	prefix := flagValue(args, "--out")
	if prefix == "" {
		return fmt.Errorf("keygen: --out <prefix> required")
	}
	pub, priv, err := minisign.GenerateKey()
	if err != nil {
		return err
	}
	if err := os.WriteFile(prefix+".pub", []byte(pub.Encode()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(prefix+".key", []byte(priv.EncodeSecret()), 0o600); err != nil {
		return err
	}
	fmt.Printf("wrote %s.pub / %s.key\n", prefix, prefix)
	return nil
}
