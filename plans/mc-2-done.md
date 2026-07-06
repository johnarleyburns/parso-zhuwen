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
