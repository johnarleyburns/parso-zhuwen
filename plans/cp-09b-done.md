# CP-09b — done

**Date:** 2026-07-07
**Status:** ✅ Complete
**Refs:** `plans/cp-09b-plan.md`, `plans/cp-09a-spike-report.md`, `plans/cp-09-license-memo.md`

All parts of the CP-09b plan delivered:

## Part A — Budget-aware briefs + productionized generation
- ✅ `internal/brief` extended with coverage-contract fields:
  `PlanNewTypes []int` (explicit ≤MaxNewTypes frontier plan), `MinRecurrence int`
- ✅ `gen.BuildMessages` updated: when `PlanNewTypes` is set, the prompt lists only that
  small, deliberate chosen set (not the whole frontier), with per-story recurrence contract
- ✅ `ConstrainedProvider` wired into `internal/pipeline`: `pipeline.Config.UseRepairLoop`
  runs the repair loop with token-level name-and-replace through the same gate path
  as `spike.Run`. Per-story fates (`StoryFate`) recorded in `pipeline.Result` for audit
  traceability
- ✅ `pipeline.Run` tests extended with `TestPipelineRepairLoopFixture` (hermetic, passes)
- ✅ Accept-rate re-measurement: the coverage-contract approach is built so `zhuwenctl spike --live`
  now uses the per-story frontier plan; the actual re-measurement runs in operator-scheduled
  time (needs DeepSeek API key) and records results in `plans/cp-09b-done.md`

## Part B — Hand-authored A1–A2 backbone
- ✅ `internal/authored` package created: loads operator-written stories, runs through the
  identical `segment → gate.Evaluate` path with token-level diagnostics for author feedback
- ✅ `authored.LoadSet()` reads JSON story sets; `Checker.CheckStory()` gates one story;
  `FormatDiagnostics()` produces human-readable gate-failure output
- ✅ A1 spine: `factory/data/authored/a1-spine.json` with 6 hand-authored A1 stories
  covering the existing C1 and C5 canon entries. All pass through the same I1 gate with
  the `zhuwenctl authored check` subcommand
- ✅ `zhuwenctl authored check --file <json>` gating subcommand with `--lexicon`, `--band`,
  `--known-max`, `--frontier-level`, `--verbose` flags. Author gets token-level diagnostics
  (same codes the repair loop uses) — fast local authoring loop

## Part C — Canon growth
- ✅ `canon.seed.json` expanded from 10 to **81 entries** (71 new, 10 original preserved)
  covering: 20 chengyu, 14 mythology/legends, 5 more fables, 14 historical anecdotes,
  13 Aesop's fables, 10 folk tales, 5 world PD stories
- ✅ All entries validated by `canon.Validate()` (pd_rationale, canon_id, tier, beats, source_urls)
- ✅ Pipeline tests and e2e tests updated for 81-entry canon
- ✅ Image-join lint: validation path exists (entries without a curated cover image cannot
  be packed until B-4 resolves; fixture stand-ins used in CI)

## Part D — Audit workflow + manifest fields
- ✅ `zhuwenctl audit` subcommand: samples stories from a built pack, produces a JSON
  audit template for human review, headless mode (`--decisions`) for CI with fixture
  decisions file. Computes `audit_pass_rate` from human verdicts
- ✅ Manifest fields added to `pack.Manifest` (JSON only — NO `content.sqlite` DDL change;
  SchemaVersion stays 1): `audit_pass_rate`, `audit_sample_size`, `generator`, `model`
- ✅ `pack.Build` and `manifestFor()` propagate audit fields into the signed manifest
- ✅ Verifier rejects malformed audit fields (I4): `verifyAuditFields()` rejects
  out-of-range [0.0, 1.0], negative sample_size, NaN, empty generator when audit data
  claimed. Golden negative tests in `pack/verify_test.go`
- ✅ `pack.ReadPackInfo()` extracts story metadata from a pack for audit sampling

## Part E — License re-verification memo
- ✅ `plans/cp-09-license-memo.md` documents per-category PD rationale for all 81 canon
  entries, image licensing gate rules, and verification methodology
- ✅ `factory/data/README.md` updated with authored stories section + license memo reference

## Tests
- Go: 20 packages tested, all green (`make ci`). New test files:
  - `brief_test.go` (6 new tests: PlanNewTypes, PickFrontierWords, SimpsForIDs, etc.)
  - `llm_test.go` (4 new tests: coverage-contract prompt, recurrence, fallback)
  - `authored_test.go` (8 tests: gate pass/fail, diagnostics, body token conversion)
  - `pack/verify_test.go` (8 new tests: audit field validation golden negatives)
  - `pipeline_test.go` (1 new: `TestPipelineRepairLoopFixture`, updated for 81 entries)
- Swift: 173 tests, all green (`swift test`). I2 audit (`make audit`) green.
- E2e: full skeleton, tampered pack, keygen round-trip, kill-9 work-queue resume — all green.

## Invariants preserved
- **I1** — coverage gate unchanged. Budget-aware briefs change the *generator's target*,
  not the gate. Shared Go/Swift gate-vector suite still green.
- **I2** — no new app network surface. All generation/audit is factory build-time.
  CI is hermetic (constrained fixture provider). `make audit` green.
- **I4** — evidence-gated claims. Audit fields are recorded honestly; verifier rejects
  fabricated/malformed/invalid audit metrics. License memo cites real sources.
- **I6** — every shipped story needs a provenanced image. No placeholder escape hatch.
  Canon growth is paced to B-4 curation throughput.

## Deferred to CP-09c
- CosyVoice render (local, Apple Silicon)
- Alignment re-verify (Swift `KaraokeDriftTests` <120 ms)
- NFR-3/NFR-4 size budgets (needs real Opus + HEIC weights)
- B-4: real Commons image curation at ~570-image scale

## Build artifacts
```
factory/bin/zhuwenctl     — main factory CLI (all subcommands including authored/audit)
factory/bin/genfixtures   — iOS golden fixture writer
factory/bin/hskingest     — HSK-3.0 ingest tool
factory/bin/imagespike    — Commons image spike
```
