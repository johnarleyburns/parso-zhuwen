# MC-1 — App target in-repo + durable event log (done)

## What shipped

### `ZhuwenPersistence` SPM target (CI-testable via `swift test`, macOS 14+)
- `EventRecord`, `StoryProgressRecord`, `PlacementResultRecord`, `PrefsRecord`,
  `PackRecord` — SwiftData `@Model`s for the on-device learner store (`00` §9).
- `ProjectionCheckpoint` — disposable cache row storing the `KnownWordModel` folded up to
  `lastSeq`, so cold launch replays only the tail (MC-1.5). The append-only log stays the
  source of truth (I5).
- `PersistentEventLog` — append-only SwiftData store conforming to `ZhuwenCore.EventSink`
  (the new protocol factoring the log surface). `append`, `append(contentsOf:)`,
  `replaceAll` (erase/import, FR-10.3), `exportJSON`. Rows are never mutated individually.
- `LearnerStore` — factory for `ModelContainer` (file-backed and in-memory variants).

### `ZhuwenCore` additive changes (all existing tests pass unmodified)
- `EventSink` protocol — `append(_:)`, `replaceAll(_:)`. `EventLog` conforms.
- `FSRSCard`, `LearningState`, `WordState`, `KnownWordModel` — gained `Codable` so the
  checkpoint row can serialize/deserialize the projection cache. These are additive
  conformances (model values are already Equatable+Sendable-safe).

### `ZhuwenUI.LearnerModel` (additive)
- Optional `sink: EventSink?` parameter; `record`, `eraseAll`, `importArchive` forward to
  it. Default `nil` → every existing test is green.
- `lookupCount` counter for XCUITest persistence smoke verification.

### `ReaderView` + `RootView`
- `ReaderView` now records lookup events on word tap via a `tapWord` closure (injected by
  `TodayView`/`LibraryView` from the learner), plus `accessibilityIdentifier("readerToken")`
  for the XCUITest.
- `TodayView` story `NavigationLink` has `accessibilityIdentifier("todayStory")`.

### App target (XcodeGen, buildable via `make app`)
- `ios/App/project.yml` — targets `ZhuwenApp` (`@main`, depends on 5 SPM products) and
  `ZhuwenAppUITests`. Generated `.xcodeproj` git-ignored. Code signing disabled (local builds).
- `ios/App/Sources/ZhuwenApp.swift` — `@main`, opens the file-backed learner store,
  loads the bundled fixture pack, injects `NoOpSyncEngine` (no CloudKit entitlement
  needed for local), and wires the persistent log into `AppModel` + `LearnerModel`.
- `ios/App/UITests/LaunchPersistenceUITests.swift` — XCUITest smoke: launch (clean) →
  open story → tap word (lookup) → relaunch → lookup count persisted.

### Tests
- `LaunchReplayTests` (CI): append → tear down ModelContainer → re-open → equal
  projection, event order preserved, append-after-relaunch monotonic seq, erase+import
  round-trip, JSON export round-trip. 5 tests.
- `ReplayPerfTests` (pre-push only, skipped in `make test-unit`): 50k full replay
  within budget, checkpointed launch within NFR-1 600 ms, seed-mismatch fallback,
  append-after-checkpoint fold. 3 tests.
- `LaunchPersistenceUITests` (XCUITest, via `make app-test`): 1 smoke test.

## Replay performance (measured on Apple M3 Max, debug build)
| Metric | Time |
|--------|------|
| 50k events cold replay (full fetch + fold) | **1.546 s** |
| 50k checkpointed launch (read cache + tail fold) | **0.006 s** ✓ |
| XCUITest smoke (full launch + interaction) | **16.251 s** |

The checkpointed path is well under the NFR-1 600 ms budget; the full 50k replay would
exceed it, so the `ProjectionCheckpoint` row was added per MC-1.5 and is stored by
`saveCheckpoint()` (called on scene-phase background). The log stays the source of truth.

## Acceptance
- [x] `make app` builds from a clean clone with no manual Xcode steps.
- [x] Launch-replay integration tests + XCUITest green.
- [x] 50k-event replay measured (1.546 s cold / 0.006 s checkpointed) and recorded.
- [x] I5 invariant-map row updated to "enforced + persistence-tested".

## Exit-criteria checklist
- [x] MC-1 acceptance met
- [x] EC-1 README (updated below)
- [x] EC-2 `make app` builds; `make app-test` runs XCUITest
- [x] EC-3 `make ci` green; persistence tests run in CI (`LaunchReplayTests`); heavy
  replay perf suite gated by pre-push hook
