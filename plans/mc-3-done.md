# MC-3 — Hosted CI + resumable work queue (done)

## CI
- **`.github/workflows/factory-ci.yml`** (new): Go factory job runs the full `make ci`
  (fmt + vet + `go test ./...`, incl. the `cmd/` e2e and the MC-3 kill-9 work-queue e2e) on
  `ubuntu-latest` with the Go version pinned from `factory/go.mod`; uploads `zhuwenctl` as a
  build artifact; a **DCO** job enforces a `Signed-off-by` trailer on every PR commit (MC-0.5).
- **`.github/workflows/ci.yml`** (restructured to iOS-only): per-push/PR macOS **unit** job
  (`make test-unit` + `make audit`) — kept because it is fast and already green — plus a
  **weekly scheduled** `ios-full` job running the full `swift test` (incl. integration +
  50k-replay perf), `make bench`, and `make audit` as the hosted signal for the heavy suites.
  README documents the split.
- README gains the factory-ci + CI **status badges** (EC-1).

## Resumable, idempotent work queue (§4 preamble; CP-01 back-filled)
- **`internal/workq`**: SQLite `work(id, stage, ref, state, attempts, last_error, updated_at)`
  queue. `Enqueue` (idempotent), `Claim` (pending→running, atomic), `Complete`, `Fail`
  (retry→pending / exhaust→failed), `ResetStale` (running→pending on resume — kill-9 recovery).
  - `result_cache(ref…)`: a completed unit is never recomputed on resume.
  - `charges(idem_key…)`: models the upstream API's idempotency-key dedup — a paid call keyed by
    brief+candidate is recorded at most once even across a crash+retry, so kill-9 mid-stage
    cannot double-charge.
  - `Process(stage, maxAttempts, fn, hook)`: drains a stage; `hook` fires in the danger window
    (after the paid call, before commit) so a test can simulate a crash there.
- **`zhuwenctl run --db <path> [--stage gen] [--resume]`**: drives the queue over the canon;
  `ZHUWEN_CRASH_AFTER=n` injects a mid-stage crash for the e2e.

## Tests
- `internal/workq` unit suite: enqueue-idempotent, reset-stale recovery, cache skip, charge
  dedup, fail→retry→exhaust, process runs/caches, process skips cached.
- **kill-9 e2e** (`cmd/zhuwenctl/workqueue_e2e_test.go`): clean run → interrupted run
  (`os.Exit(137)` mid-stage) → `--resume` → **identical final results**, **exactly one charge
  per unit** (no double charge), stage fully drained. Passes in ~1.2 s.

## Dependency note
No new modules — `modernc.org/sqlite` (already vendored) backs the queue too.

## Acceptance
- [x] Green badge on README (factory-ci + CI).
- [x] kill-9 e2e test passes.
- [x] CP-01's resumability acceptance genuinely met and noted in `plans/cp-01-done.md` as a
  back-filled criterion.

## Exit-criteria checklist
- [x] MC-3 acceptance met
- [x] EC-1 README updated (badges, `run` command, CI section)
- [x] EC-2 `make build` → `bin/zhuwenctl run …` works
- [x] EC-3 `make ci` green; unit + e2e for the queue; DCO enforced in CI
