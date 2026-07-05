# CP-04 — Done note (KnownWordModel + EventLog + Selector, iOS)

**Status:** complete. Factory `make ci` green (**58 Go tests**; both commands build to
`bin/`); iOS `swift test` green (**42 Swift tests**, 24 new); NFR-2 selector benchmark green
under `make bench` (release: 5,000 stories gated + scored in ~13 ms, budget 50 ms).

## Handoff §6 acceptance
- **EventLog / KnownWordModel / FrontierQueue / CoverageGate / Selector** — all in
  `ZhuwenCore`:
  - `EventLog` — append-only event list (no mutate/delete API), the I5 source of truth.
  - `KnownWordModel` — a **pure, replayable projection** of the event log: `project(events)`
    folds them; `applying(_:)` folds one more. State ∈ {unseen, introduced, learning, known,
    mastered}, P(known) ∈ [0,1] (FR-2.1/2.2); effective known set = P≥0.8 ∪ learning-frontier
    (FR-2.3). Placement seed hook for CP-05.
  - `FrontierQueue` — orders candidates by HSK level → frequency → character-familiarity
    bonus (FR-2.4).
  - `CoverageGate` + `StoryCandidate` — the on-device I1 gate; `StoryCandidate`'s init is
    private, so only `CoverageGate.evaluate()` can produce a populated one.
  - `Selector` — indexes the lattice by word-type bitmap (`WordBitmap`), gates by
    `story & ~known` popcount + `⊆ frontier`, and scores survivors by frontier/SRS payload,
    novelty, and length fit (FR-3.1/3.2).
- **Replay test (I5)** — `KnownWordModelTests`: incremental fold == batch `project`;
  re-projection is byte-identical; order-sensitive but deterministic.
- **Gate property tests (I1)** — `CoverageGateTests`: the shared vector suite + a randomized
  400-case property test cross-checking codes/pass against an independent budget calc + the
  private-init proof.
- **Selector benchmark (NFR-2)** — `SelectorTests.testGates5kStoryLatticeUnder50ms`:
  asserts < 50 ms under `-c release` (`make bench`); the debug run prints the number and
  applies a gross-regression guard (unoptimized wall-clock isn't comparable to the 50 ms
  device target).

## Cross-cutting: one gate, two implementations (handoff §7)
The Go reference gate is the single source of truth. New Go package `internal/gatevec`
defines deterministic gate inputs, runs each through `gate.Evaluate`, and serializes inputs
+ language-neutral expectations (`pass`, `denomTokens`, `newTokens`, `coverageBps`,
`newTypeIDs`, `codes`) to `ios/Fixtures/gate-vectors.json` (written by `genfixtures` /
`make fixtures`). `gate.Result` gained non-breaking `Codes`/`DenomTokens`/`NewTokens` fields;
seven stable reason codes are shared verbatim by the Swift `GateCode`. The vendored JSON is:
- generated + self-consistency-checked by `go test ./internal/gatevec`,
- locked to the committed file (`TestVendoredVectorsUpToDate`),
- reproduced field-for-field by the Swift gate (`CoverageGateTests`).
So I1 cannot drift between the Go factory and the iOS app.

## Deliverables
- **factory:** `internal/gate` (codes + denom/new counts), `internal/gatevec` (+ tests),
  `internal/fixtures` + `cmd/genfixtures` emit `gate-vectors.json`.
- **ios (`ZhuwenCore`):** `Bitmap.swift`, `CoverageGate.swift`, `EventLog.swift`,
  `KnownWordModel.swift`, `LexiconStore.swift`, `FrontierQueue.swift`, `Selector.swift`.
- **ios (`Fixtures`):** `gate-vectors.json`. **Makefile:** `make bench`.

## Tests (unit / integration / e2e)
- **Unit:** `WordBitmap` ops (via gate/selector), `CoverageGate` baseline + randomized
  property test, `EventLog` ordering/codable, `KnownWordModel` transitions + replay,
  `FrontierQueue` ordering + familiarity bonus, `Selector` gate/score/novelty.
- **Integration:** the shared `gate-vectors.json` suite through the Swift gate; Go `gatevec`
  self-consistency + code coverage + vendored-file lock.
- **E2E:** `SelectorTests.testRecommendsRealFixtureStories` — events → projection → frontier
  queue → selector gate over the **real vendored pack** returns every gated story at ≥98%.

## Exit criteria (standing)
- [x] Handoff §6 acceptance for CP-04 met (model/selector + I5 replay, I1 gate, NFR-2 bench).
- [x] EC-1 READMEs updated (root + `ios/`) with status + run/test commands (incl. `make bench`).
- [x] EC-2 `make build` → `bin/{zhuwenctl,genfixtures}` runnable; iOS uses `swift`/`xcodebuild`.
- [x] EC-3 `make ci` green; `swift test` green; unit + integration + e2e present; no new deps
      (Swift stdlib only; Go adds none).

## No new dependencies
Swift: standard library only (`UInt64.nonzeroBitCount`, `JSONDecoder`). Go: none.

## Follow-ons
Placement/logistic seeding feeding `KnownWordModel.seed` (CP-05); FSRS memory params + due
scheduling that refine `srsPayload` (CP-07); SwiftData persistence of the event log and UI
wiring of Today/Library selection (assembled with the app target). Next up: **CP-05
(Placement, M1–M3; pseudoword foils; logistic seeding; re-placement merge).**
