# CP-07 — Done note (Loop completion: comprehension→seal M8, FSRS review M9, Progress M10)

**Status:** complete. iOS `swift test` green (**124 Swift tests**, +34); `swift build` compiles the new
`ZhuwenCore` engines + `ZhuwenUI` screens; `make build-ios` **BUILD SUCCEEDED** (the M8/M9/M10 screens
cross-compile for the iOS 17 simulator); `make bench` green (NFR-2 unaffected, ~2.0 ms release). Factory
`make ci` green (**untouched** — no schema change, no fixture regeneration; questions were already in the
frozen pack since CP-01/CP-02).

## Handoff §6 acceptance
- **✅ P(known) updates verified for exposure/lookup/grade paths.**
  `KnownWordModelTests.testPKnownUpdatesForExposureLookupGradePaths` asserts each direction: an
  **exposure** raises P(known), a **lookup** lowers it, a **good** review grade raises it *and* creates
  FSRS memory (`goodGrades` incremented), a **failing** grade lowers it *and* records a lapse. A companion
  `testReviewGradeFSRSIsReplayable` proves the FSRS card is reconstructed from the log (batch == incremental
  fold), preserving the I5 replay guarantee.
- **Comprehension → seal (M8, FR-6.1/6.2).** `ComprehensionSession` reads the pack's 3 MC questions
  (`ContentDatabase.questions(storyID:)`), grades them, passes at **≥2/3**, and on a pass emits a positive
  `.comprehension` event for every content word the story exposed — boosting P(known) and moving frontier
  words toward *learning*. `ComprehensionView` stamps the 读完为证 seal with a spring press (a plain fade
  under Reduce Motion, NFR-6). Reached from the reader's "Finish — check comprehension" button.
- **FSRS review (M9, FR-7.1/7.2/7.3).** `FSRSScheduler` is the open **FSRS-4.5** algorithm (17 default
  weights, power-law forgetting curve), on-device and deterministic. `ReviewScheduler.dueCards` surfaces
  words whose card `due ≤ now` as **sentence-context** cards — the target word inside a sentence from a
  story the learner actually read, cited to its source — capped at **20/day**, soonest-due first. Grades
  append `.reviewGrade` events that advance the card's memory. `ReviewView` shows the projected Again/Hard/
  Good/Easy intervals.
- **Progress with both-skill estimates (M10, FR-6.3).** `ProgressEstimator.report` computes a **reading**
  band from HSK-level coverage of the known set and a **listening** band folded *independently* from the
  `.listen` events (blind passes count double, FR-5.2), plus words-known, weekly growth (re-projecting the
  log at week boundaries), HSK level + words-to-next, and a CEFR can-do line — all labeled estimates (I4).
  `LearnerProgressView` is M10.

## Invariants preserved
- **I5 every tap teaches / append-only, replayable.** Comprehension outcomes and review grades are new
  `Event`s; `KnownWordModel` (now including each word's `FSRSCard`) remains a **pure projection** of the
  ordered log + placement seed. FSRS state is folded from `.reviewGrade` events with their timestamps —
  never persisted out of band — so `project == incremental fold` still holds (`testReviewGradeFSRSIsReplayable`).
- **I3 pregenerated only.** Comprehension questions ship in the pack; the FSRS scheduler is an open,
  on-device algorithm (no generation, no network). **I2 no network:** all new state is a local projection.
- **Schema frozen (CP-02).** The `question`/`sentence_translation` tables were frozen at CP-02 and the
  `question` table was already populated at CP-01, so CP-07 adds **no schema change and no fixture
  regeneration** — the freeze guard (`schema_test`) is untouched and the factory `make ci` stays green.

## Deliverables
- **ZhuwenPacks:** `QuestionRecord` + `ContentDatabase.questions(storyID:)` + `PackStore.questions(for:)`.
- **ZhuwenCore:** `FSRS.swift` (`FSRSCard`, `FSRSScheduler` FSRS-4.5, `Rating`, `review`/`intervals`);
  `WordState.fsrs` + `KnownWordModel.apply(.reviewGrade)` folds the scheduler + `dueWordIDs(at:)`;
  `Comprehension.swift` (`ComprehensionSession`, seal + completion events); `Review.swift`
  (`ReviewCard`, `ReviewScheduler`); `Progress.swift` (`ProgressReport`, `ProgressEstimator`).
- **ZhuwenUI:** `LearnerModel` (owns the event log → projection; vends M8/M9/M10); `ComprehensionView`
  (M8 + `SealStamp`), `ReviewView` (M9), `LearnerProgressView` (M10); Review/Progress tabs wired in
  `RootView`; reader "Finish → comprehension" entry; `AppModel.learner` + `makeComprehensionView`.

## Tests (unit / integration / e2e)
- **`FSRSTests`** — first- and later-review interval ordering (Again≤Hard≤Good≤Easy), stability grows on
  success, Again reduces stability + counts a lapse, difficulty stays in [1,10], retrievability decays,
  determinism.
- **`ComprehensionTests`** — pass at 2/3 and 3/3, fail at 1/3, seal only on pass, pass boosts P(known) for
  exposed words, fail records the negative outcome, no events mid-session, exposed-word extraction skips
  literals/proper nouns.
- **`ReviewTests`** — only due words surface, cap honored, sentence contains the target from a read story,
  words without a read-story sentence are skipped, a grade advances FSRS `due`, `dueCount` ignores the cap.
- **`ProgressTests`** — reading band tracks coverage, empty model is A0, words-to-next HSK, weekly growth is
  monotonic & ends at the known count; **`testListeningEventsMoveListeningButNotReading`** + blind-weighting
  assert the both-skill separation.
- **`KnownWordModelTests`** — the CP-07 acceptance (exposure/lookup/grade P(known) paths) + FSRS replay.
- **`QuestionPackTests`** — 3 questions per story load from the vendored fixture, ordered, valid answerIdx.

## Exit criteria (standing)
- [x] Handoff §6 acceptance for CP-07 met (comprehension→seal, FSRS review, Progress both-skill;
      P(known) updates verified for exposure/lookup/grade paths).
- [x] EC-1 READMEs updated (root + `ios/`) with status + commands.
- [x] EC-2 `make build` → `bin/` unchanged (factory untouched); `make ci` still green.
- [x] EC-3 `swift test` green (unit + integration + e2e/acceptance); `make build-ios` succeeds; `make bench` green.

## No new dependencies
FSRS-4.5 is implemented from its published formulas in Swift stdlib math; all engines are pure and
host-testable. No Go module additions; the factory is untouched.

## Follow-ons
Real factory-generated comprehension questions + sentence translations replacing the CP-01 stubs (CP-09);
`StoryProgress`/event-log SwiftData persistence, the monthly checkpoint (FR-6.4), and the methodology
citation page (I4) assembled with the `@main` app target; Foundations progress (CP-08a). Next up:
**CP-08 (Commerce & data — CDN `PackClient`, StoreKit 2 SKUs, paywall, export/erase, optional CloudKit).**
