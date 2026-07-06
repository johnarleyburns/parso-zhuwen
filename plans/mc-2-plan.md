# MC-2 — Content-reality spike (plan)

**Reads with:** `02-midcourse-correction.md` (MC-2), handoff §4.1–4.4, invariants
**I1** (gate budgets, never loosened) and **I3** (no generation in the app). De-risks CP-09.

## Scope & honesty constraints
The spike's *thesis validation* — real LLM pass-rates inside a real band lexicon — has two
hard external inputs I will not fake (handoff §0.7, MC-2.6 "do not loosen gate budgets to
make numbers"):
1. A **license-cleared HSK-3.0 word list**. The official lists' redistribution terms are
   unclear, so per MC-2.1 the raw data is **not shipped**; the ingest command + provenance
   doc are, and a blocker is filed. The fixture lexicon remains for tests.
2. A **live LLM API key**. No keys in the repo (I2). The LLM path is behind a runtime flag
   and never exercised in CI; unit tests cover prompt construction hermetically.

So MC-2 lands the **harness** (ingest, eval, LLM client, repair loop) with hermetic tests,
plus a spike report that (a) validates the pipeline mechanics end-to-end and (b) records the
live-LLM run as **blocked pending owner-provided key + lexicon** rather than inventing metrics.

## Tasks
1. **Lexicon ingest (§4.1).** `internal/lexicon` gains `WriteSQLite`/`ReadSQLite` and a
   HSK-list reader; `zhuwenctl lexicon ingest --src <dir> --out lexicon.sqlite --version <v>`.
   `factory/data/README.md` documents source/provenance/terms; `plans/blockers.md` records the
   licensing uncertainty. Dependency: `modernc.org/sqlite` (already vendored, pure-Go, CI-safe).
2. **Segmenter eval.** `internal/segment` gains a coverage/ambiguity evaluator;
   `zhuwenctl segment eval` reports token/type coverage + ambiguity hotspots over the spike
   stories. Decision (jieba-parity necessity) recorded in the done note.
3. **Real generation stage.** `internal/gen` gains `LLMProvider` (OpenAI-compatible; base URL,
   model, key via env — DeepSeek). Prompt = §4.2 brief contract; N=3, low temperature. Network
   only via an explicit `--live` flag; the fixture provider stays the default (CI hermetic).
4. **Repair loop (§4.4).** New `internal/repair`: turns a gate `Result`'s failure codes into a
   targeted rewrite prompt naming exact violations; max 4 iterations; discard+log after.
   Hermetic unit tests over fixture gate failures + a scripted-provider convergence test.
5. **Run the spike.** Harness over 5 canon entries × A2 × 3 candidates through
   gen→segment→gate→repair; `plans/mc-2-spike-report.md` with the metrics that are obtainable
   hermetically, and the live-LLM metrics marked blocked.
6. **Go/adjust decision recorded** in the report. Gate budgets untouched (I1).

## Tests (hermetic)
- `lexicon`: WriteSQLite→ReadSQLite round-trips the fixture lexicon (ids/attrs stable).
- `segment eval`: coverage math + ambiguity detection on crafted inputs.
- `gen`: LLM **prompt construction** golden test (no network); response parsing from a canned
  JSON body; fixture provider determinism (also back-fills MC-4.3).
- `repair`: prompt names each violation code; loop converges with a scripted provider and
  discards after 4 failures.

## Acceptance
Ingest + eval + repair merged with hermetic tests; spike report committed; CI untouched by
network paths; blocker filed for the license-cleared lexicon + live key.
