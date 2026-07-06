# MC-4 — Documentation truth-up (plan)

**Reads with:** `02-midcourse-correction.md` (MC-4), `plans/exit-criteria.md` (EC-1). Half a day.

## Tasks
1. Amend `01-agentic-handoff.md` §1: monorepo `parso-zhuwen` (`factory/` + `ios/`) decision
   record; fixture vendoring is a path, not a cross-repo sync.
2. Annotate `plans/cp-01-done.md`: shipped 10 fixture stories vs 20 implied (scale → CP-09);
   resumability back-filled at MC-3 (already noted).
3. Add missing tests: `internal/brief` beat-sheet golden test; `internal/gen` fixture
   determinism (landed with MC-2).
4. Refresh `README.md` per EC-1: MC status table, `make app`, license section, CI badges
   (badges/app/license/commands landed across MC-0…3; MC-4 adds the MC table + consolidation).

## Acceptance
EC-1 holds; `grep -ri agpl` clean over the code/README/license set; docs match repo reality.
