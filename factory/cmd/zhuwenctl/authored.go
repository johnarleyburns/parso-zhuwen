package main

import (
	"fmt"

	"github.com/parso/zhuwen-factory/internal/assets"
	"github.com/parso/zhuwen-factory/internal/authored"
	"github.com/parso/zhuwen-factory/internal/gate"
	"github.com/parso/zhuwen-factory/internal/grammar"
	"github.com/parso/zhuwen-factory/internal/lexicon"
	"github.com/parso/zhuwen-factory/internal/pipeline"
)

func cmdAuthored(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("authored: expected subcommand: check")
	}
	switch args[0] {
	case "check":
		return cmdAuthoredCheck(args[1:])
	default:
		return fmt.Errorf("authored: unknown subcommand %q (try: check)", args[0])
	}
}

func cmdAuthoredCheck(args []string) error {
	file := flagValue(args, "--file")
	if file == "" {
		// Positional first arg as fallback
		if len(args) > 0 && args[0] != "" {
			file = args[0]
		}
	}
	if file == "" {
		return fmt.Errorf("authored check: --file <path> required")
	}

	// Load lexicon (fixture by default; can switch to real via --lexicon).
	lexPath := flagValue(args, "--lexicon")
	var lex *lexicon.Lexicon
	var err error
	if lexPath != "" {
		lex, err = lexicon.ReadSQLite(lexPath)
		if err != nil {
			return fmt.Errorf("open lexicon: %w", err)
		}
		fmt.Printf("using real lexicon: %s (%d words)\n", lex.Version(), lex.Len())
	} else {
		lex, err = assets.Lexicon()
		if err != nil {
			return err
		}
	}

	// Band: default fixture A2 band; --known-max / --frontier-level for real HSK bands.
	knownMax := 2
	frontierLevel := 3
	if v := flagValue(args, "--known-max"); v != "" {
		fmt.Sscanf(v, "%d", &knownMax)
	}
	if v := flagValue(args, "--frontier-level"); v != "" {
		fmt.Sscanf(v, "%d", &frontierLevel)
	}
	bandName := flagValue(args, "--band")
	if bandName == "" {
		bandName = "A1"
	}
	spec := pipeline.BuildHSKBand(lex, bandName, knownMax, frontierLevel, knownMax)
	fmt.Printf("band %s: known HSK≤%d (%d), frontier HSK %d (%d)\n",
		bandName, knownMax, len(spec.Known), frontierLevel, len(spec.Frontier))

	// Load authored set.
	set, err := authored.LoadSet(file)
	if err != nil {
		return err
	}
	fmt.Printf("loaded %d authored stories (origin: %s)\n", len(set.Stories), set.Origin)

	// Build checker — same gate path as pipeline.
	checker := &authored.Checker{
		Seg:      nil, // rebuilt per-story from Dict
		Band:     gate.Band{Known: spec.Known, Frontier: spec.Frontier, Grammar: spec.Grammar},
		Detector: grammar.MarkerDetector{},
		MaxID:    lex.MaxID(),
		Cfg:      gate.DefaultConfig(),
		Dict:     lex.DictEntries(),
	}
	// Build per-story propers from canon registry.
	reg, err := assets.Canon()
	if err != nil {
		return err
	}
	propersMap := map[string]map[string]string{}
	for _, e := range reg.All() {
		propers := map[string]string{}
		for _, ch := range e.Characters {
			if ch.NameZH == "" {
				continue
			}
			gloss := ch.Gloss
			if gloss == "" {
				gloss = ch.NameZH
			}
			propers[ch.NameZH] = gloss
		}
		propersMap[e.CanonID] = propers
	}

	passed := 0
	total := 0
	for _, s := range set.Stories {
		propers := propersMap[s.CanonID]
		cr := checker.CheckStory(s, propers)
		total++
		if cr.Pass {
			passed++
		}
		fmt.Printf("\n[%s] %s — %s\n", s.CanonID, s.TitleZH, crLabel(cr))
		fmt.Printf("  coverage=%.2f%% new-types=%d\n",
			cr.Coverage*100, len(cr.NewTypeStats))
		if !cr.Pass {
			for _, c := range cr.Codes {
				fmt.Printf("  code: %s\n", c)
			}
			if len(cr.Literals) > 0 {
				fmt.Printf("  literals: %v\n", cr.Literals)
			}
			for _, nt := range cr.NewTypeStats {
				simp := fmt.Sprintf("#%d", nt.ID)
				if w, ok := lex.LookupID(nt.ID); ok {
					simp = w.Simp
				}
				status := "in-frontier"
				if !nt.InFrontier {
					status = "OUT-OF-FRONTIER"
				}
				fmt.Printf("  「%s」: %d× (%s)\n", simp, nt.Count, status)
			}
		}
		if hasFlag(args, "--verbose") {
			fmt.Printf("  text: %s\n", s.Text)
		}
	}

	fmt.Printf("\n---\n%d/%d passed (%.0f%%), %d failed\n",
		passed, total, 100*float64(passed)/float64(total), total-passed)
	if total-passed > 0 {
		return fmt.Errorf("authored check: %d story(s) failed the I1 gate", total-passed)
	}
	return nil
}

func crLabel(cr authored.CheckResult) string {
	if cr.Pass {
		return "PASS"
	}
	return "FAIL"
}
