# CP-05 — Done note (Placement M1–M3: pseudoword foils, logistic seeding, re-placement merge)

**Status:** complete. iOS `swift test` green (**61 Swift tests**, 19 new); `swift build`
compiles `ZhuwenUI`; `make build-ios` **BUILD SUCCEEDED** (placement screens cross-compile for
the iOS 17 simulator); `make bench` green (NFR-2 unaffected, ~3.5 ms release). Factory
`make ci` green (**58 Go tests**, no factory changes in CP-05).

## Handoff §6 acceptance
- **M1–M3 flow.** `ZhuwenUI.PlacementView` renders M1 welcome/privacy (+ complete-beginner
  route), M2 yes/no word card (progress, "be honest" copy), M3 result (CEFR + HSK cards,
  ≈words-known, guessing-corrected knowledge-curve bars). Driven by `PlacementFlowModel`
  over the pure `ZhuwenCore` engine; reachable from the Today toolbar (FR-1.5 "repeatable").
- **Pseudoword foils (FR-1.1).** `PseudowordGenerator` builds plausible-but-nonexistent
  2-char compounds from the lexicon's own characters (frequency-ordered pool), rejecting any
  real word; deterministic per seed. `PlacementItemBuilder` samples real items **stratified by
  HSK-3.0 level × frequency** plus a foil fraction, 60–120 items.
- **Logistic seeding (FR-1.2).** `PlacementEstimator` fits P(yes) over standardized
  `log(freq_rank)` by ridge-regularized IRLS (finite under all-yes/all-no separation),
  corrects for guessing with the foil false-alarm rate, and emits a conservative
  probabilistic **seed** (`PlacementSeed`) → `KnownWordModel.seeded(_:events:)`, plus a
  **CEFR band (A0–B2)** and **HSK-3.0 level (1–6)** from cumulative HSK vocab thresholds.
- **Re-placement merge (FR-1.5).** `PlacementSeed.merged(with:)` keeps the max prior per word;
  because every `KnownWordModel` per-event update is monotonic in its input prior,
  re-projecting the same append-only event log over the merged seed **cannot lower any word's
  P(known)** — merges, never destroys. (I5 preserved: the seed is the projection's prior, not
  a log mutation.)
- **✅ Simulated learners recover HSK band within ±1.** `PlacementSimulationTests` sweeps
  logistic learners (6 midpoints × 4 RNG seeds) through the full pipeline and asserts
  `|estimated − true| ≤ 1` on a shared HSK ruler, plus monotonicity of the estimate in ability
  and run determinism.

## Also delivered
- **FR-1.3 reading refinement.** `PlacementEstimator.refine(_:passages:)` folds two short
  comprehension outcomes into a bounded reading factor: poor comprehension (strong spoken,
  weak reading) pulls the estimate/seed down; acing it does not lower it.
- **FR-1.4 absolute-beginner path.** `PlacementSession.skipAsBeginner()` → empty seed,
  `route == .foundations`, HSK 0 / CEFR A0.
- **Conservative seed (risk §13).** The reported estimate is honest; the gate seed is
  discounted (`conservatism`) and floored (`seedFloor`) so a mis-seed starves rather than
  floods and self-corrects from lookups (I5).

## Deliverables
- **ZhuwenCore:** `Random.swift` (`SplitMix64`), `Pseudoword.swift`, `Placement.swift`
  (items/builder/session/seed/result/curve/config/passage), `PlacementEstimator.swift`
  (IRLS fit + guessing correction + band mapping + refine); `KnownWordModel.seeded(_:events:)`.
- **ZhuwenUI:** `PlacementView.swift` (`PlacementFlowModel` + M1–M3), `.jade` accent color,
  Today-toolbar entry + `AppModel.makePlacementFlow()`.

## Tests (unit / integration / e2e)
- **Unit:** `PseudowordTests` (2-char, non-word, distinct, deterministic, freq-ordered pool);
  `PlacementTests` (builder count/foil-fraction/strata/determinism; session advance/complete/
  beginner-skip; estimator all-yes⇒high / all-no⇒low; guessing correction; passage refinement).
- **Integration:** `PlacementTests.testSeedFeedsKnownWordModel` (seed → effective known set),
  `testReplacementMergeNeverLowersPKnown` (FR-1.5 property over an event log).
- **E2E / simulation:** `PlacementSimulationTests.testRecoversHSKBandWithinOne` (+ monotonicity,
  determinism) — the full item-sample → simulate → fit → band pipeline.

## Exit criteria (standing)
- [x] Handoff §6 acceptance for CP-05 met (M1–M3, foils, logistic seeding, re-placement merge;
      simulated ±1 HSK recovery).
- [x] EC-1 READMEs updated (root + `ios/`) with status + commands (no new commands).
- [x] EC-2 `make build` → `bin/` unchanged (no factory code this CP; `make ci` still green).
- [x] EC-3 `swift test` green (unit + integration + e2e/simulation); `make build-ios` succeeds.

## No new dependencies
Swift stdlib + Foundation `log/exp` only; hand-written `SplitMix64` for reproducible sampling
(no `SystemRandomNumberGenerator`). No Go/factory changes.

## Follow-ons
SwiftData persistence of `PlacementResult` + first-run gating (assembled with the `@main` app
target); full CAT item selection; the Foundations F0–F3 program itself (CP-08a). Next up:
**CP-06 (Listening — pack-audio karaoke via alignment; speeds; blind mode; TTS fallback).**
