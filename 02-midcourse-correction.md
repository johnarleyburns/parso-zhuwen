# Zhuwen — Mid-Course Correction Handoff (02)

**Reads with:** `00-requirements-and-design.md` (v3), `01-agentic-handoff.md`,
`plans/exit-criteria.md`. Requirement IDs refer to `00`; EC-* refer to the standing
exit criteria. This document remedies the gaps found in the post-CP-07 repo audit
(July 2026) and re-sequences the remaining checkpoints. Work the MC items **in order**;
each is a normal checkpoint (plan file → implementation → done note → EC checklist).

**Audit summary being remedied:** (1) `@main` app target and SwiftData persistence
live outside the repo, so I5 durability is untested — events do not survive relaunch;
(2) no repair loop (§4.4) and no resumable work queue — pipeline is pure-function
synchronous; (3) core content bet (constrained retelling pass-rates with a real LLM
and real lexicon) is unvalidated at fixture scale (32-word lexicon, deterministic gen,
FMM-only segmentation); (4) no hosted CI — EC-3 is local-only; (5) license was
AGPL-3.0 without a contribution policy, creating App Store/relicensing friction;
(6) doc drift: monorepo (not two repos), CP-01 shipped 10 stories (not 20).

**Revised checkpoint order after MC-4:** CP-08a (images + Foundations) → CP-08
(commerce & data) → CP-09 (content scale-up, de-risked by MC-2) → CP-10.
Rationale: content and curation are the schedule's long pole; StoreKit is well-trodden.

---

## MC-0 — License change to GPLv3 + App Store exception (repo hygiene, do first)

**Tasks**
1. Replace `LICENSE` with the verbatim GPLv3 text (file provided alongside this doc;
   sha256 it against https://www.gnu.org/licenses/gpl-3.0.txt in the done note).
2. Add `NOTICE-APP-STORE.md` (provided) — the GPLv3 §7 additional permission for
   Apple App Store/TestFlight distribution, plus the contributor DCO +
   license-with-exception requirement.
3. Add `CONTRIBUTING.md`: DCO sign-off required (`git commit -s`); contributions are
   GPLv3 + the §7 exception; plan-first workflow per handoff §0.
4. Update `README.md` license section: "GPLv3 with an App Store additional permission
   (see LICENSE and NOTICE-APP-STORE.md)" — matches the Cladiron licensing posture.
5. Add DCO sign-off check to CI (arrives with MC-3; wire the check there).
6. Do NOT add per-file license headers (repo-level licensing is sufficient; keep
   diffs clean).

**Acceptance:** LICENSE hash matches gnu.org source; README/NOTICE/CONTRIBUTING
consistent; no AGPL references remain (`grep -ri agpl` clean).

---

## MC-1 — App target in-repo + durable event log (closes the I5 hole; was "CP-07b")

**Problem.** `ZhuwenUI` compiles, but the `@main` app target is assembled ad hoc in
Xcode outside the repo, and `EventLog` is an in-memory array. I5's replay guarantee
has never been tested across a process boundary; agents cannot build the actual app.

**Tasks**
1. Vendor the app target into the repo, agent-buildable. Preferred: **XcodeGen** —
   commit `ios/App/project.yml` (targets: `ZhuwenApp` [@main, depends on the four SPM
   packages], `ZhuwenAppUITests`), git-ignore the generated `.xcodeproj`, and add
   `make app` (xcodegen + xcodebuild) to `ios/Makefile`. XcodeGen is a build-time
   tool, not a shipped dependency — list it per handoff §0.4 anyway (MIT).
2. `ZhuwenApp` module: `@main` App struct, SwiftData `ModelContainer` for
   `EventRecord`, `StoryProgressRecord`, `PlacementResultRecord`, `PrefsRecord`,
   `PackRecord` (per `00` §9), dependency wiring of `LearnerModel`/`AppModel`.
3. **Durable EventLog.** `PersistentEventLog` (in `ZhuwenApp` or a thin
   `ZhuwenPersistence` target): append-only SwiftData store conforming to the existing
   `EventLog` API; `KnownWordModel` remains a pure projection rebuilt on launch by
   replay (chunked; measure). Never mutate or delete event rows (I5).
4. **Launch-replay test (the point of this MC):** integration test that (a) appends a
   scripted event history, (b) tears down and re-creates the ModelContainer from the
   same store URL (simulating relaunch), (c) rebuilds `WordState` and asserts equality
   with the pre-teardown projection, and (d) asserts event count/order unchanged.
   Plus an XCUITest smoke: place → read → lookup → relaunch app → lookup count and
   frontier state persist.
5. Replay performance budget: rebuilding from 50k events must complete inside the
   NFR-1 launch budget on device; if not, add a snapshot/checkpoint row (projection
   cache keyed by last event ID) — cache is disposable, log remains source of truth.
6. Export/erase groundwork (FR-10.3): JSON export of the raw event log + records;
   erase deletes the store. (Full settings UI remains CP-08.)

**Acceptance:** `make app` builds from a clean clone with no manual Xcode steps;
launch-replay integration test + XCUITest green; 50k-event replay measured and
recorded in the done note; I5 row in the invariant map updated to "enforced +
persistence-tested".

---

## MC-2 — Content-reality spike (de-risks CP-09; the product bet)

**Problem.** The gate has only ever seen deterministic fixtures. Whether a real LLM
can retell canon beats inside a real band lexicon at acceptable pass rates — the
thesis of the whole factory — is unmeasured, and there is no repair loop.

**Tasks**
1. **Real lexicon ingest (§4.1, brought forward).** `zhuwenctl lexicon ingest` reading
   the HSK-3.0 vocabulary lists into `lexicon.sqlite` (~11k stable word IDs,
   level + frequency-rank attributes). Source files and their provenance/terms go in
   `factory/data/README.md` — if the canonical list's licensing is unclear, stop and
   file `plans/blockers.md` rather than shipping it (handoff §0.7); the fixture
   lexicon remains for tests either way.
2. **Segmenter upgrade check.** Keep FMM but add a dictionary-coverage evaluation
   command (`zhuwenctl segment eval`) that reports token/type coverage and ambiguity
   hotspots over the spike stories; decide jieba-parity necessity with data, not vibes.
   Record the decision in the done note (this closes the §13 segmentation risk or
   promotes it to CP-09 work).
3. **Real generation stage.** `internal/gen` gains an LLM client (OpenAI-compatible
   endpoint; config: base URL, model, key via env — DeepSeek per house pattern).
   Prompt = §4.2 brief contract. N=3 candidates per brief, temperature low.
   No API keys in repo; runnable offline via the existing fixture provider (CI stays
   hermetic — the LLM path is behind a build/config flag, never exercised in CI).
4. **Repair loop (§4.4).** `internal/repair`: takes a gate `Result`'s failure codes and
   emits a targeted rewrite prompt naming exact violations; max 4 iterations; discard
   and log after. Unit-test the prompt construction against fixture failures (hermetic).
5. **Run the spike.** 5 canon entries (2×C1, C2, C4, C5) × A2 band × 3 candidates,
   through gen → segment → gate → repair. Produce `plans/mc-2-spike-report.md`:
   pass rate at iteration 0, mean repair iterations to pass, discard rate, token cost
   per shipped story, failure-code histogram, and 3 verbatim shipped stories for
   J's human read.
6. **Go/adjust decision recorded:** if mean repair iterations ≤2 and discard ≤20%,
   CP-09 proceeds as planned; otherwise propose prompt/brief changes or budget
   adjustments in the report — **do not** loosen gate budgets (I1) to make numbers.

**Acceptance:** ingest + eval + repair merged with hermetic tests; spike report
committed with the metrics above; CI untouched by network paths.

---

## MC-3 — Hosted CI + resumable pipeline decision

**Tasks**
1. `.github/workflows/factory-ci.yml`: on push/PR — `make ci` (fmt, vet, test) on
   ubuntu-latest with pinned Go; artifact the built `zhuwenctl`; add the DCO check
   (MC-0.5). Keep it fast (<3 min).
2. Swift CI: add a macOS runner job for `swift test` if runner minutes are acceptable;
   otherwise document in README that Swift tests are local-gated (EC-3 note) and add
   a weekly scheduled macOS job as compromise. Owner J decides cost tolerance —
   default to the weekly job.
3. **Work queue (§4 preamble), sized by MC-2 data:** convert the pipeline to the
   SQLite work-queue design (stages resumable/idempotent, kill-9-safe) **iff** the
   spike confirms real LLM/TTS stages (it will — multi-second network stages with
   money attached must be resumable). Table: `work(id, stage, ref, state, attempts,
   last_error, updated_at)`; `zhuwenctl run --stage X --resume`. E2E test: kill the
   process mid-stage (test harness), rerun, assert identical final pack and no
   double-charged gen calls (idempotency keys per brief+candidate).

**Acceptance:** green badge on README; kill-9 e2e test passes; CP-01's original
resumability acceptance is now genuinely met and noted in `plans/cp-01-done.md`
as a back-filled criterion.

---

## MC-4 — Documentation truth-up (half a day)

1. Amend `01-agentic-handoff.md` §1: monorepo `parso-zhuwen` with `factory/` + `ios/`
   (decision record: simpler for a solo architect; fixture vendoring becomes a path,
   not a cross-repo sync).
2. Annotate CP-01 done note: shipped 10 fixture stories vs 20 specified (scale
   deferred to CP-09) and resumability back-filled at MC-3.
3. Add missing test files for `internal/gen` (fixture provider determinism) and
   `internal/brief` (beat-sheet compilation golden test).
4. Refresh README per EC-1: MC status table, new `make app` instructions, license
   section, CI badge.

**Acceptance:** EC-1 holds; `grep -ri agpl` clean; docs match repo reality.

---

## Sequencing & effort guide

| Item | Depends on | Rough size |
|------|-----------|------------|
| MC-0 license | — | hours |
| MC-1 app target + durable I5 | — | 1–2 days |
| MC-2 content spike | MC-0 (keys/env conventions) | 2–3 days incl. report |
| MC-3 CI + queue | MC-2 (queue sizing) | 1–2 days |
| MC-4 docs | all | half day |

Then resume the main line in the revised order: **CP-08a → CP-08 → CP-09 → CP-10**.
CP-09's plan must open by importing the MC-2 spike report's decisions.

## Standing rules (unchanged)

Handoff §0 applies in full: plan-first, requirement IDs in commits, invariants I1–I6
are stop-and-file-a-blocker territory, no new deps unlisted, EC-1/2/3 on every MC item.
The MC series does not modify any invariant, any gate budget, or the pack format
(schema stays frozen at v1; MC-2's real lexicon ships as a new `lexicon_version`,
which the format already supports).
