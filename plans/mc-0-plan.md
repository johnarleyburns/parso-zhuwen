# MC-0 — License change to GPLv3 + App Store exception (plan)

**Reads with:** `02-midcourse-correction.md` (MC-0), `01-agentic-handoff.md` §0,
`plans/exit-criteria.md`. Repo hygiene item, done first.

## Tasks
1. Replace `LICENSE` with verbatim GPLv3 text; sha256 it against
   https://www.gnu.org/licenses/gpl-3.0.txt in the done note.
2. Add `NOTICE-APP-STORE.md` — GPLv3 §7 additional permission for Apple App
   Store/TestFlight distribution + contributor DCO + license-with-exception requirement.
3. Add `CONTRIBUTING.md`: DCO sign-off (`git commit -s`); contributions GPLv3 + §7
   exception; plan-first workflow per handoff §0.
4. Update `README.md` license section: "GPLv3 with an App Store additional permission
   (see LICENSE and NOTICE-APP-STORE.md)".
5. DCO sign-off CI check — **deferred to MC-3** (wire alongside `factory-ci.yml`).
6. No per-file license headers.

## Tests / acceptance
- `LICENSE` sha256 == gnu.org GPLv3 source (recorded in done note).
- README / NOTICE / CONTRIBUTING consistent.
- `grep -ri agpl` clean over tracked sources (build artifacts and this MC series'
  descriptive text excepted).

## Notes
No code, no deps, no invariant touched. EC-2/EC-3 unaffected (no build changes);
EC-1 satisfied by the README license section.
