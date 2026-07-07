# CP-08a — Done note (Commons image pipeline §8A · Foundations F0–F3 · first-run onboarding gating)

**Status:** complete (Parts A, B, C). Factory `make ci` green (**+30 Go tests** across `internal/images`,
`internal/foundations`, `internal/pack`); `gofmt`/`go vet` clean; `make build` emits the new
`zhuwenctl images` subcommands to `bin/`. iOS `make test-unit` green (**163 Swift tests**, +19);
`swift build`, `make build-ios`, and `make app` all **BUILD SUCCEEDED** for the iOS 17 simulator;
**`make audit` green** — the app network surface is unchanged (Commons fetch is factory-only, I2).

Parts A & B landed in PR #9; Part C (iOS) is this note's focus.

## Handoff §6 acceptance
- **✅ I6 builder tests.** `pack.validateI6` (Go) and `ContentDatabase.verifyFoundationsI6` (Swift) both
  reject a `foundations_card` with an empty/missing `image_id`, a missing provenance record, or an
  AI-categorized image; the license/quality gate golden fixtures reject NC/ND/GFDL-only/missing and
  `Category:AI-generated images` and sub-1200px. Existing golden imageless-reject stays green.
- **✅ A scripted zero-knowledge run reaches F3 in ~300 words.**
  `FoundationsSimulationTests.testZeroKnowledgeLearnerReachesHandoffAroundThreeHundredWords`: a learner
  starting from **zero** runs the Foundations program (F0 four-step → F1/F2 recombination per set),
  every interaction folding into the one `KnownWordModel` (I5), and the `HandoffGate` (≥20 A1 stories at
  ≥98%, reusing the I1 coverage formula) fires between the 250th and 300th known word. Not ready at start
  or midway.

## Part C — iOS (this session)
- **`ZhuwenCore/Foundations.swift` — pure engine.** `FoundationsProgram` (groups pack `foundations_card`
  rows into ordered semantic sets), `FoundationsSession` (F0 introduce→recognize→read→bind; ends on an
  F1/F2 recombination pass, **never** on an isolated card — FR-11.4), `FoundationsDeck` distractors drawn
  only from already-taught same-set predecessors with the factory's deterministic rotation (FR-11.3),
  `HandoffGate` (F3), and `FoundationsProgram.startingSet(for:)` (FR-11.6). Every step emits ordinary
  `Event`s; a completed bind marks the word known and the recombination pass re-exposes the set (I5).
- **`ZhuwenUI/FoundationsView.swift` (M14).** Photo + audio + hanzi + tone-colored pinyin, the four-step
  interaction with recognition grids, a **long-press attribution** sheet and a full **Credits** screen
  (author/license/source, FR-11.2), system-TTS on every interaction. `FoundationsModel` owns the session
  and folds events into the shared `LearnerModel`.
- **Progress (FR-11.5).** Words-known counts from #1 in the Foundations header; the M10 reading card reads
  **"Pre-A1 · Foundations"** until the estimate leaves A0.
- **First-run onboarding gating.** `AppModel.onboardingRoute` (`needsPlacement` → `foundations` → `loop`):
  a fresh launch with no persisted result **auto-presents** placement (M1–M3); `completePlacement` persists
  a `PlacementSnapshot` via `PersistentPlacementStore` (SwiftData), merges the seed (FR-1.5), and routes
  complete/partial beginners into Foundations at their first unmastered set (FR-1.4/11.6). The regular loop
  activates at the F3 handoff. The manual "Re-run placement" affordance lives in Settings only.
- **Methodology page (I4).** `MethodologyView` states the §5A.4 honest limits — picture-binding degrades
  for abstraction; a one-line English gloss is a behind-a-tap fallback, off by default.
- **Pack readers.** `ImageRecord`/`FoundationsCardRecord` + `ContentDatabase.foundationsCards()`/`images()`
  + `PackStore.imageData(for:)`; the vendored `fixture-a2-v0.zpack` now ships F0 cards (regenerated via
  `make fixtures`).

## Tests added (Part C)
- **`FoundationsTests` (12):** program ordering, FR-11.3 distractors (subset-of-taught, no repeated set,
  determinism), session ends on recombination, wrong-answer repeat, folds into `KnownWordModel`,
  `startingSet` (beginner/partial/all-mastered), handoff threshold + status.
- **`FoundationsSimulationTests` (2):** the ~300-word zero-knowledge acceptance + replay determinism.
- **`FoundationsPackTests` (5):** fixture ships F0 cards, ordering, distractors-are-taught, images fully
  provenanced, `verifyFoundationsI6` passes.
- **XCUITest `FoundationsOnboardingUITests`:** fresh launch → placement auto-presents → complete-beginner
  → Foundations → answer cards → relaunch persists (events + no re-presented placement). The MC-1
  `LaunchPersistenceUITests` gains a `-uiTestSkipOnboarding` hook so the story-loop smoke stays
  deterministic.

## Invariants preserved
- **I6** extended to `foundations_card` (both implementations); no placeholder-art escape hatch.
- **I5** — Foundations is not a silo: every interaction is an `Event` folded into the one
  `KnownWordModel`; a relaunch mid-course replays exactly (`testFoundationsEventsReplayDeterministically`).
- **I1** — the F3 handoff reuses `CoverageGate.coverageBps`; no gate fork, no loosened budget.
- **I2** — no new app network surface (`make audit` green); Commons fetch stays factory-only, behind `--live`.
- **I4** — the honest-limits deviation is stated on the methodology page.

## Deferred / follow-ons (not CP-08a)
- Real OCR/watermark + NSFW/safety models (stubbed with a documented interface → CP-09 hardening).
- The real ~570-image curated inventory + per-image license sign-off (**B-4**, owner-in-the-loop).
- CosyVoice Foundations card-intro audio; real HEIC covers + pack-size recheck (CP-09).
