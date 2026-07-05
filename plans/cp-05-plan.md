# CP-05 — Placement (M1–M3): pseudoword foils, logistic seeding, re-placement merge

**Refs:** handoff §5 (`PlacementEstimator` — "logistic fit over frequency rank; conservative
seed per risk table"), §6 (CP-05 acceptance: *simulated learners at known curves recover
level ±1 HSK band*). `00` §4 FR-1.1–1.5, FR-2.1/2.3 (seed feeds `KnownWordModel`), §9
(`PlacementResult`), §10 M1–M3, §13 risk ("placement mis-seeds → conservative seed + fast
correction from lookups (I5)"). Depends on CP-04 (`KnownWordModel.project(_:seed:)`,
`LexiconStore`, `FrontierQueue`).

**Acceptance (handoff §6):** M1–M3 flow; pseudoword foils; logistic seeding into the
known-word model; re-placement merge (FR-1.5). ✅ **Simulated learners at known curves
recover level ±1 HSK band.** Plus standing exit criteria (`plans/exit-criteria.md`).

## Invariants in play
- **I5 every tap teaches / replay.** The placement seed stays *outside* the event log: it is
  the `seed:` prior of the existing pure projection `KnownWordModel.project(events, seed:)`.
  The log remains append-only and the model remains a replayable fold over it — placement
  only supplies the P(known) base the events fold onto. Re-placement re-projects; it never
  mutates or deletes events.
- **I2 no accounts/network.** Placement is entirely on-device: items are sampled from the
  vendored pack lexicon, foils are generated locally. No new network surface.
- **No new dependencies** (NFR-5): Swift stdlib + Foundation `log/exp` only. Deterministic
  RNG (`SplitMix64`) is hand-written so sampling/simulation are reproducible.

## Design decisions
1. **Placement is a `ZhuwenCore` engine + a thin `ZhuwenUI` flow** — the same split as
   CP-03/CP-04 (`ReaderModel` in Core, `ReaderView` in UI). All decision logic and math live
   in Core and are unit-tested on the host; the SwiftUI screens (M1–M3) wrap it.
2. **Pseudoword foils (FR-1.1).** `PseudowordGenerator` builds plausible-but-nonexistent
   2-char compounds from the lexicon's own characters (pool ordered by character frequency),
   rejecting any surface that is a real lexicon word. Deterministic per seed. Foils measure
   the learner's **false-alarm (overclaim) rate** `f`.
3. **Item sampling (FR-1.1).** `PlacementItemBuilder` samples real words **stratified by
   HSK-3.0 level × frequency band**, plus a foil fraction (~20%), 60–120 items total,
   deterministic. (True CAT adaptivity is a documented v1 simplification; the mockup M2 spec
   is "stratified by HSK-3.0 level × frequency", which this implements.)
4. **Logistic fit (FR-1.2).** `PlacementEstimator` fits P(yes) as a logistic over
   standardized `log(freq_rank)` by **ridge-regularized IRLS** (Newton; ridge keeps the slope
   finite under perfect separation — the all-yes / all-no learner). Guessing correction:
   P(known|rank) = clamp((P(yes) − f)/(1 − f)). Estimated known-word count = Σ over the
   lexicon of P(known); mapped to an **HSK-3.0 level (1–6)** and **CEFR band (A0–B2)** by
   cumulative HSK-3.0 vocabulary thresholds.
5. **Conservative seed (risk §13).** The reported *estimate* uses the honest corrected curve;
   the **seed** that feeds the gate multiplies P(known) by a `conservatism` factor (<1) and
   drops sub-floor priors, so a mis-seed starves rather than floods (fast to correct from
   lookups, I5). Seed → `KnownWordModel.seeded(_:events:)` = `project(events, seed:)`.
6. **Reading-passage refinement (FR-1.3).** `PlacementEstimator.refine(_:passages:)` folds two
   short comprehension outcomes into a bounded reading factor — poor comprehension (strong
   spoken vocab, weak reading) pulls the estimate/seed **down**, good comprehension does not
   pull it down. Recomputes count/band/seed.
7. **Absolute-beginner path (FR-1.4).** `PlacementSession.skipAsBeginner()` → empty seed,
   `route == .foundations`, HSK 0 / CEFR A0. Model seeds empty (Foundations bootstrap owns
   CP-08a).
8. **Re-placement merge (FR-1.5).** `PlacementSeed.merged(with:)` takes the **max prior per
   word** — a re-placement can only *raise* a word's base, never destroy it. Because every
   per-event P(known) update is monotonic non-decreasing in its input prior, re-projecting the
   same event log over the merged seed guarantees **no word's P(known) decreases** — the
   "merges, never destroys" property, provable and tested.

## Deliverables
### iOS (`ZhuwenCore`)
- `Random.swift` — `SplitMix64` deterministic `RandomNumberGenerator` (repeatable sampling).
- `Pseudoword.swift` — `PseudowordGenerator` (2-char nonword foils from lexicon chars).
- `Placement.swift` — `PlacementItem`, `PlacementItemBuilder`, `PlacementSession` (welcome →
  wordCheck → result state machine; beginner skip), `PlacementSeed` (merge, FR-1.5),
  `PlacementResult`, `CEFRBand`, `PlacementRoute`, `LogisticCurve`, `PlacementConfig`,
  `PassageOutcome`.
- `PlacementEstimator.swift` — foil false-alarm rate, IRLS logistic fit, guessing correction,
  known-count → HSK/CEFR mapping, seed construction, passage refinement.
- `KnownWordModel.swift` — add `seeded(_:events:)` convenience (placement → model seam).

### iOS (`ZhuwenUI`)
- `PlacementView.swift` — `PlacementFlowModel: ObservableObject` wrapping the Core session +
  estimator; `PlacementView` renders M1 (welcome/privacy + beginner route), M2 (yes/no word
  card, progress, "be honest" copy), M3 (CEFR + HSK cards, knowledge-curve bars, ≈words).
- `FlowLayout.swift` — add `.jade` accent (used by M2 "I know it" / M3 curve).

### Tests (`ZhuwenCoreTests`)
- `PseudowordTests.swift` — foils are 2 real characters, never a real word, distinct,
  deterministic per seed.
- `PlacementTests.swift` — builder count/foil-fraction/strata coverage/determinism; session
  advance + progress + completion + beginner skip; estimator monotonicity (all-yes ⇒ high
  band, all-no ⇒ low band); guessing correction lowers an inflated tail; **FR-1.5 merge never
  lowers P(known)**; FR-1.4 empty-seed Foundations route; FR-1.3 passage refinement doesn't
  raise a failed-comprehension estimate.
- `PlacementSimulationTests.swift` — **acceptance:** across a sweep of true logistic learners
  (varying midpoint) × RNG seeds, the estimator recovers the HSK band within **±1**, and the
  estimate is monotonic in true ability.

## Test plan → acceptance map
- Simulated ±1 HSK recovery (handoff §6) → `PlacementSimulationTests.testRecoversHSKBandWithinOne`.
- Pseudoword foils (FR-1.1) → `PseudowordTests`.
- Logistic seeding into the model (FR-1.2) → `PlacementTests.testSeedFeedsKnownWordModel`.
- Re-placement merge (FR-1.5) → `PlacementTests.testReplacementMergeNeverLowersPKnown`.
- Beginner path (FR-1.4) → `PlacementTests.testBeginnerSkipSeedsEmptyFoundations`.
- Passage refinement (FR-1.3) → `PlacementTests.testPassageRefinementCatchesOverclaim`.

## Exit criteria (standing)
- EC-1 READMEs updated (root + `ios/`) with CP-05 status + commands (no new commands).
- EC-2 `make build` → `bin/` unchanged (no factory code in CP-05; `make ci` still green).
- EC-3 `swift test` green (unit: pseudoword/builder/session/estimator; integration: seed →
  `KnownWordModel`; e2e/simulation: ±1 band recovery). `swift build` compiles `ZhuwenUI`.

## Out of scope (later CPs)
Full CAT item selection; SwiftData persistence of `PlacementResult` + first-run gating
(assembled with the `@main` app target); Foundations F0–F3 program itself (CP-08a); FSRS
memory params (CP-07). No factory/Go changes.
