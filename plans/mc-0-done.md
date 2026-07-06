# MC-0 — License change to GPLv3 + App Store exception (done)

## What shipped
- `LICENSE` replaced with the verbatim GNU GPL version 3 text.
- `NOTICE-APP-STORE.md` — GPLv3 §7 additional permission authorizing Apple App
  Store / TestFlight distribution, plus the contributor DCO + license-with-exception
  requirement.
- `CONTRIBUTING.md` — DCO sign-off (`git commit -s`), contributions under GPLv3 + the
  §7 exception, and the plan-first workflow (handoff §0).
- `README.md` — new **License** section: "GPLv3 with an App Store additional permission
  (see LICENSE and NOTICE-APP-STORE.md)".

## Verification
- **LICENSE integrity:** `shasum -a 256 LICENSE` ==
  `3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`,
  byte-identical to https://www.gnu.org/licenses/gpl-3.0.txt (verified by `diff`).
- **No AGPL:** `grep -ri agpl` over tracked sources is clean. The only remaining
  matches are (a) `.build/` compiler artifacts (git-ignored) and (b) descriptive text
  in the MC handoff/plan/done docs referring to the *former* license — not a license
  declaration.
- **Consistency:** README, NOTICE-APP-STORE, and CONTRIBUTING all state GPLv3 + the
  §7 App Store additional permission and the DCO requirement.

## Deferred (by design)
- MC-0.5 DCO sign-off CI check is wired in **MC-3** alongside `factory-ci.yml`.
- No per-file license headers (repo-level licensing is sufficient; keeps diffs clean).

## Exit-criteria checklist
- [x] MC-0 acceptance met (LICENSE hash matches; docs consistent; `grep -ri agpl` clean)
- [x] EC-1 README updated (License section added)
- [x] EC-2 `make build` → `bin/` unaffected (no build/code changes)
- [x] EC-3 `make ci` unaffected (no code changes); DCO CI check deferred to MC-3
