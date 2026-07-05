# Standing Exit Criteria (all checkpoints)

These apply to **every** checkpoint (CP-01 … CP-10) in addition to that checkpoint's own
acceptance criteria in `01-agentic-handoff.md` §6. A checkpoint is not "done" until all
standing criteria pass. Added per project-owner instruction.

## EC-1 — README is current
`README.md` (repo root) MUST contain:
- a summary of **where the app is** (which checkpoints are complete, what works today,
  what is stubbed/deferred), and
- **instructions for running and testing** every buildable component (exact commands).

Update `README.md` as part of finishing each checkpoint.

## EC-2 — Commands build via make into `bin/`
Every command (`cmd/*`) MUST be buildable with `make build` and the resulting executables
MUST land in a `bin/` directory and be runnable from there (e.g. `./bin/zhuwenctl …`).
`bin/` is git-ignored. This holds per repo/module that has commands (currently `factory/`).

## EC-3 — Green gates (pre-existing, restated)
`make ci` (fmt + vet + test) green; tests cover each feature at unit / integration / e2e
levels; invariants I1–I6 enforced and tested; no new deps without listing them in the CP
plan (handoff §0.4).

## Checklist to copy into each `plans/cp-XX-done.md`
- [ ] Handoff §6 acceptance for CP-XX met
- [ ] EC-1 README updated (status + run/test instructions)
- [ ] EC-2 `make build` → `bin/` executables run
- [ ] EC-3 `make ci` green; unit + integration + e2e present for new features
