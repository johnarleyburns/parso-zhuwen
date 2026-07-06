# MC-2 — Content-reality spike (done)

## What shipped (all hermetic; CI never takes a network path)
- **Lexicon ingest (§4.1).** `internal/lexicon`: `WriteSQLite`/`ReadSQLite` (frozen
  `lexicon.sqlite` shape — stable word IDs + level/freq attributes + char_ids), `IngestDir`
  (operator TSV[s] → Lexicon), `FromWords`. CLI: `zhuwenctl lexicon ingest --src --out --version`.
  `factory/data/README.md` documents the source format + provenance requirements.
- **Segmenter eval (MC-2.2).** `internal/segment`: `Eval` reports token/type coverage,
  literal (unresolved) rate, and FMM ambiguity hotspots. CLI: `zhuwenctl segment eval`.
- **Real generation stage (§4.2).** `internal/gen`: `LLMProvider` (OpenAI-compatible;
  DeepSeek defaults), `BuildMessages` (pure, golden-tested §4.2 prompt), `parseCompletion`.
  Network only via `Retell` with `ZHUWEN_LLM_API_KEY`; refuses without a key.
- **Repair loop (§4.4).** `internal/repair`: `HintsFromResult`/`RewritePrompt` (one hint per
  gate code, names exact violations), `Reprocessor.Run` (retell→gate→repair, max 4 iters,
  discard+log), `PipelineChecker` (segment→gate).
- **Spike harness.** `internal/spike` + `zhuwenctl spike [--n k] [--live]`; metrics aggregation.
- **Report + blockers.** `plans/mc-2-spike-report.md`, `plans/blockers.md` (B-1 HSK licensing,
  B-2 live key).

## Dependency note (handoff §0.4)
`modernc.org/sqlite` (BSD-3-Clause, pure-Go, no cgo — already vendored for pack building)
is now a **direct** dependency of `internal/lexicon`. No new module added; CI-safe on Linux.

## Decisions recorded
- **Segmenter:** keep FMM; jieba-parity question **promoted to CP-09**, to be re-decided via
  `segment eval` over the first real LLM corpus (closes §13 tracking, not the risk itself).
- **Go/adjust:** deferred, harness-ready. Live content-bet numbers require B-1 + B-2.

## Honesty / invariant posture
- No HSK data shipped (B-1); fixture lexicon remains the tested lexicon.
- No API keys in repo (I2); LLM path behind a runtime flag, never in CI.
- Gate budgets untouched (I1). No spike metrics fabricated.

## Verification
- `make ci` green; new hermetic tests in `lexicon`, `segment`, `gen`, `repair`, `spike`.
- `zhuwenctl lexicon ingest` verified on a sample TSV → `lexicon.sqlite`.
- `zhuwenctl segment eval` and `zhuwenctl spike --n 5` run offline; `spike --live` errors
  without a key (proves CI hermetic).

## Exit-criteria checklist
- [x] MC-2 acceptance: ingest + eval + repair merged with hermetic tests; report committed;
  CI untouched by network paths; blockers filed
- [x] EC-1 README updated (new commands + blocker pointer)
- [x] EC-2 `make build` → `bin/zhuwenctl` runs the new subcommands
- [x] EC-3 `make ci` green; unit tests for every new feature

## Follow-up (B-1 + B-2 resolved, 2026-07-06)
Owner cleared the HSK licensing and supplied a DeepSeek key, so the spike was run for real:
- **Real HSK-3.0 lexicon shipped** (`factory/data/hsk3.0/level-*.tsv`, 12,283 forms =
  vocabulary + recognition characters, exact per-level mapping) via new `cmd/hskingest`;
  ingested to `lexicon.sqlite` (`hsk3.0-v1`). Provenance in `factory/data/README.md`.
- **Live DeepSeek spike run** with real repair-feedback (`gen.RepairProvider`), the completed
  grammar whitelist (把/被 added), and the real HSK band (`pipeline.BuildHSKBand`).
- **Outcome = ADJUST** (`plans/mc-2-spike-report.md`): 0% pass at A2 and B1 — naive prompting
  can't hold the I1 budget; CP-09 needs vocabulary-constrained decoding, token-level repair,
  budget-aware briefs, and proper-noun handling. **Gate budgets left untouched (I1).**
- Blockers B-1 and B-2 marked RESOLVED.

