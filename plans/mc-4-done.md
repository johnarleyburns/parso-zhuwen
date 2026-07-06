# MC-4 — Documentation truth-up (done)

## What changed
1. **Handoff §1 monorepo decision record.** `01-agentic-handoff.md` §1 now states the project
   is a single monorepo `parso-zhuwen` (`factory/` + `ios/` + `plans/`), the two-repo table
   labels name directories not repos, fixture vendoring is an in-repo path
   (`make -C factory fixtures`), and the `@main` app lives in `ios/App/` (MC-1).
2. **CP-01 done-note annotations.** Recorded that CP-01 shipped **10** fixture stories (not the
   20 the scope implied) with scale-up deferred to CP-09, alongside the MC-3 resumability
   back-fill already noted.
3. **Missing tests back-filled.**
   - `internal/brief/brief_test.go` — beat-sheet compilation golden test (identity, beats,
     characters, band envelope, lexicon slice) + determinism.
   - `internal/gen/fixture_test.go` — fixture-provider determinism (added with MC-2, satisfies
     MC-4.3's `internal/gen` test requirement).
4. **README refresh (EC-1).** Added the **MC status table**, CI **badges** (MC-3), the
   `make app` instructions (MC-1), the new `zhuwenctl` subcommands (MC-2/3), and the License
   section (MC-0) — all landed across MC-0…MC-4 and now consolidated.

## `grep -ri agpl`
Clean across code, `README.md`, `LICENSE`, `NOTICE-APP-STORE.md`, and `CONTRIBUTING.md`. The
only remaining matches are **descriptive** references in the MC handoff/plan/done docs (which
narrate the *former* AGPL license and the audit that removed it) and git-ignored `.build/`
compiler artifacts — neither is a license declaration.

## Verification
- `make -C factory ci` green (incl. the new `brief` + `gen` tests).
- Docs match repo reality: monorepo layout, 10-story CP-01, in-repo app target, GPLv3+§7,
  hosted CI + work queue, MC-2 harness with blockers.

## Exit-criteria checklist
- [x] MC-4 acceptance: EC-1 holds; `grep -ri agpl` clean (code/README/license set); docs match repo
- [x] EC-1 README updated (MC table, badges, app + command instructions, license)
- [x] EC-2 `make build` unaffected
- [x] EC-3 `make ci` green; `internal/brief` + `internal/gen` tests present
