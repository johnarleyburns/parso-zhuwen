# Contributing to Zhuwen

Thanks for your interest. This project is copyleft **and** ships on the Apple App
Store; keeping both true requires a small amount of contributor discipline. Please
read this before opening a pull request.

## Licensing of contributions

By submitting a contribution (patch, pull request, or any other change) you agree
that your contribution is licensed under the **GNU General Public License, version 3**
(see [`LICENSE`](LICENSE)) **including the GPLv3 section 7 additional permission for
Apple App Store / TestFlight distribution** described in
[`NOTICE-APP-STORE.md`](NOTICE-APP-STORE.md). Do not contribute code you cannot
license on these terms.

## Developer Certificate of Origin (DCO)

Every commit must be signed off, certifying you have the right to submit it under the
license above (see the [DCO](https://developercertificate.org/)). Add the sign-off
trailer with:

```sh
git commit -s
```

This appends `Signed-off-by: Your Name <you@example.com>` using your configured
`user.name` / `user.email`. Commits without a valid sign-off are rejected by CI.

## Plan-first workflow (handoff §0)

This repository is developed plan-first. For any non-trivial change:

1. **Write the plan first.** Each checkpoint begins by writing/updating
   `plans/cp-XX-plan.md` (or `plans/mc-X-*.md`) with the task breakdown and test list;
   implementation follows the plan.
2. **Cite requirement IDs** (`I*`, `FR-*`, `NFR-*`, `CP-*`, `EC-*`) in every commit
   message — e.g. `CP-04: implement CoverageGate (I1, FR-3.1)`.
3. **Invariants I1–I6 are non-negotiable.** If a change appears to require weakening
   one, stop and write `plans/blockers.md` instead of coding around it.
4. **No new dependencies** without listing them (name, license, why) in the plan.
   License floor: Apache-2.0 / MIT / BSD / MPL for code; per `00` §8A for content assets.
5. **Tests before merge.** Every acceptance criterion maps to at least one automated
   test. Golden-file tests are the backbone of the factory.
6. **No telemetry, no accounts, no runtime network** beyond `PackClient` + StoreKit
   (I2). CI greps enforce this.

## Before you push

Enable the local quality gate once per clone (runs the heavier integration/e2e suites,
the NFR-2 benchmark, and the network-surface audit on `git push`):

```sh
git config core.hooksPath .githooks
```

`make ci` (factory) and `make test` (ios) must be green, and the standing exit
criteria in [`plans/exit-criteria.md`](plans/exit-criteria.md) must hold.
