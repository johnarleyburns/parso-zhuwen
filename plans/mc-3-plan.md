# MC-3 — Hosted CI + resumable work queue (plan)

**Reads with:** `02-midcourse-correction.md` (MC-3), handoff §4, CP-01 plan
(resumability acceptance). DCO check deferred from MC-0.5.

## CI
1. **`.github/workflows/factory-ci.yml`** (the MC-3 deliverable): on push/PR — `make ci`
   (fmt + vet + test including `./cmd/` e2e) on `ubuntu-latest` with pinned Go; upload
   the built `zhuwenctl` binary as an artifact; include the DCO sign-off check. Target
   < 3 min.
2. **Swift CI:** keep the existing per-PR `macOS-14` unit job (fast, demonstrably green);
   add a **weekly scheduled** macOS job for the full `swift test` integration suite as a
   fallback signal. Document in README.
3. **DCO check:** verify every commit in the PR has a `Signed-off-by:` trailer; the
   check runs in the factory-ci workflow (and in the existing CI yml for completeness).

## Resumable work queue (§4, CP-01 back-filled)
The MC-2 spike confirmed real LLM/TTS stages exist, which are multi-second, network-attached,
money-attached — so resumability is justified. The SQLite work-queue design is the handoff's
original plan.

- **`internal/workq`** package: `work(id, stage, ref, state, attempts, last_error, updated_at)`.
  States: `pending` → `running` → `done` | `failed`. `ref` = brief identifier.
- **Idempotency**: a `result_cache(ref)` table keyed by a stable descriptor of the unit of
  work (canon+brief hash) → cached `result_json` + `tokens_used`. On resume, a unit whose
  cache row exists is skipped — no double-charged gen calls.
- **`zhuwenctl run [--stage X] [--resume]`**: processes the work queue from `db_path`,
  stage-by-stage, with the resume flag picking up `running` items (kill-9 safe — row stays
  in `running` across a crash; on resume/reset, they move to `pending` and are retried,
  with the cache preventing double-execution of completed units).
- **E2E test:** `zhuwenctl run` on a tiny queue, kill the process mid-stage (test
  harness SIGKILLs or Context-cancels), rerun with `--resume`, assert the final pack is
  identical and the mock provider was called ≤N times (no duplicate charges).
- **Back-fill CP-01 done note** with a resumability acceptance postscript.

## Tests
- `workq`: state-machine transitions, idempotency-cache hit, fail→retry exhaustion.
- E2E: kill-9 resume, identical pack output, no double gen calls.

## Acceptance
Green badge on README; kill-9 e2e test passes; CP-01 resumability back-filled.
