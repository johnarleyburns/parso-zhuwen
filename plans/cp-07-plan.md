# CP-07 — Loop completion: comprehension→seal (M8), FSRS review (M9), Progress with both-skill estimates (M10)

**Refs:** handoff §6 (CP-07 acceptance: *P(known) updates verified for exposure/lookup/grade paths*),
§5 (`ZhuwenCore`: `FSRS` scheduler; `KnownWordModel` projection). `00` §4 FR-2.2 (event-driven state),
FR-6.1/6.2 (3 MC questions → seal + P(known) boost), FR-6.3 (CEFR reading + listening bands, HSK level,
words-to-next), FR-7.1–7.3 (on-device FSRS, sentence-context cards, 20/day cap, grades feed the model),
§9 (`StoryProgress.sealEarned`, `Event`). Mockups M8 (comprehension→seal 读完为证), M9 (review card),
M10 (progress). Depends on CP-04 (`EventLog`/`KnownWordModel`, I5), CP-05 (placement seed), CP-06
(`.listen` events retained for the listening estimate).

**Acceptance (handoff §6):** comprehension→seal (M8), FSRS review (M9), Progress (M10) with both-skill
estimates. ✅ **P(known) updates verified for exposure/lookup/grade paths.** Plus standing exit criteria.

## Invariants in play
- **I5 every tap teaches / append-only, replayable.** Comprehension outcomes and review grades are
  appended as `Event`s; the whole learner state (`KnownWordModel` **and** the FSRS memory of every card)
  is a pure, deterministic projection of the ordered event log (+ placement seed). FSRS card state is
  reconstructed by folding `.reviewGrade` events with their timestamps — never stored out-of-band — so
  replay determinism (the CP-04 guarantee) still holds.
- **I3 pregenerated only.** Comprehension questions ship in the pack (`question` table, already populated
  at CP-01); the app renders and grades them. The FSRS scheduler is an open, on-device algorithm (no
  generation, no network).
- **I2 no accounts/network.** All new state is local (event log projection). No new `URLSession`.
- **No new dependencies** (NFR-5): FSRS is hand-written Swift stdlib math; all engines are host-testable.

## Design decisions
1. **Pure engines in `ZhuwenCore`, thin `@MainActor` glue in `ZhuwenUI`** (the CP-04/05/06 split):
   - `FSRS` — the open FSRS-4.5 algorithm (17 default weights, documented). `FSRSCard`
     {stability, difficulty, due, lastReview, reps, lapses}; `review(card, grade, at:)` and
     `intervals(card, at:)` (projected Again/Hard/Good/Easy days for the M9 buttons). Deterministic.
   - `Comprehension` — `ComprehensionSession` (M8): the pack's 3 questions, answer/advance, pass ≥2/3,
     `sealEarned`. On completion it yields the `Event`s to append: a `.comprehension(correct:)` for every
     distinct content word the story exposed (FR-6.2 "boosts P(known) for its words"; frontier words move
     toward *learning*). The pass is the seal moment.
   - `Review` — `ReviewScheduler.dueCards(...)`: words whose FSRS `due ≤ now` become **sentence-context**
     cards (FR-7.1) — the target word inside a sentence from a story the learner actually read; capped
     (default 20/day, FR-7.2), ordered by due. Grades append `.reviewGrade` events (FR-7.3).
   - `Progress` — `ProgressEstimator.report(...)`: **separate** reading and listening CEFR bands
     (FR-6.3). Reading band from HSK-level coverage of the known set; listening band folds the `.listen`
     events (blind counts double, FR-5.2) and is computed **independently of reading** (the both-skill
     separation). Plus words-known, weekly growth (re-project at week boundaries), HSK level +
     words-to-next-level, a CEFR can-do line. Everything labeled "estimate" (I4/FR-6.3).
2. **`KnownWordModel` gains FSRS.** `WordState` carries an optional `FSRSCard`; `apply(.reviewGrade)`
   advances it via `FSRSScheduler.default` **and** nudges the P(known) heuristic (unchanged direction).
   `.comprehension`/`.exposure`/`.lookup` keep their CP-04 rules. The switch stays exhaustive; `.listen`
   still a no-op for reading P(known) (folded only into the listening estimate).
3. **`LearnerModel` (@MainActor, `ZhuwenUI`)** owns the `EventLog` + placement seed, re-projects the
   `KnownWordModel` on every append, and vends the comprehension session, the review queue, and the
   progress report to the M8/M9/M10 screens. Persistence (SwiftData) is assembled with the `@main` target.
4. **Factory unchanged.** The `question` table is already populated (CP-01 stub questions; real generation
   is CP-09) and frozen since CP-02; the reader reconstructs review sentences from the packed body tokens.
   No schema change, no fixture regeneration.

## Tasks
1. **`ZhuwenPacks`** — `QuestionRecord` + `ContentDatabase.questions(storyID:)` + `PackStore.questions(for:)`.
2. **`ZhuwenCore/FSRS.swift`** — `FSRSCard`, `FSRSScheduler` (FSRS-4.5), `review`, `intervals`, `Rating`.
3. **`ZhuwenCore` model** — `WordState.fsrs`; `KnownWordModel.apply(.reviewGrade)` folds the scheduler.
4. **`ZhuwenCore/Comprehension.swift`** — `ComprehensionQuestion`, `ComprehensionSession`, seal + events.
5. **`ZhuwenCore/Review.swift`** — `ReviewCard`, `ReviewScheduler` (due, sentence-context, cap).
6. **`ZhuwenCore/Progress.swift`** — `ProgressReport`, `ProgressEstimator` (both-skill, growth, HSK gap).
7. **`ZhuwenUI`** — `LearnerModel`; `ComprehensionView` (M8, seal stamp w/ Reduce-Motion fade, NFR-6);
   `ReviewView` (M9); `ProgressView` (M10); wire the Review/Progress tabs + a "Finish → questions" entry
   from the reader. `make build-ios`.
8. **Docs** — root + `ios/` READMEs (CP-07 ✅); `plans/cp-07-done.md`.

## Tests (unit / integration / e2e)
- **`FSRSTests`** — interval ordering Again<Hard<Good<Easy; stability grows on success, lapse (Again)
  reduces it and increments lapses; difficulty stays in [1,10]; determinism (same events ⇒ same card).
- **`ComprehensionTests`** — pass at 2/3 and 3/3, fail at 1/3; seal only on pass; the emitted events boost
  P(known) for exposed words and move a frontier word toward *learning*; replay-stable.
- **`ReviewTests`** — only due words surface; cap honored; card sentence contains the target word and cites
  a read story; a grade appends a `.reviewGrade` that advances the card's FSRS `due`.
- **`ProgressTests`** — reading band tracks the known set; **listening events move the listening band but
  not the reading band** (both-skill separation); words-known count + weekly growth; HSK level +
  words-to-next-level; everything labeled estimate.
- **`KnownWordModelTests` (acceptance)** — `testPKnownUpdatesForExposureLookupGradePaths`: exposure raises,
  lookup lowers, a *good* grade raises and advances FSRS while a *fail* grade lowers — the CP-07 §6 check.
- **`ContentDatabaseTests`/`PackStoreTests`** — 3 questions per story load from the vendored fixture.

## No new dependencies
FSRS-4.5 is implemented from its published formulas in Swift stdlib math; all engines are pure and
host-testable. No Go module additions; the factory is untouched.

## Follow-ons (not CP-07)
Real factory-generated comprehension questions + sentence translations replacing CP-01 stubs (CP-09);
`StoryProgress`/event-log SwiftData persistence, monthly checkpoint (FR-6.4), and the methodology citation
page (I4) assembled with the `@main` app target; Foundations progress (CP-08a).
