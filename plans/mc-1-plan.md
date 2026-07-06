# MC-1 — App target in-repo + durable event log (plan)

**Reads with:** `02-midcourse-correction.md` (MC-1), `00` §9 (records), `01` §5,
invariant **I5**. Closes the I5 durability hole: `@main` + SwiftData persistence now
live in-repo and are agent-buildable, and the event log survives a process boundary.

## Design
- New SPM target **`ZhuwenPersistence`** (depends `ZhuwenCore`), host-testable via
  `swift test`, so the launch-replay guarantee runs in CI — not just under Xcode.
  - SwiftData `@Model`s (per `00` §9): `EventRecord`, `StoryProgressRecord`,
    `PlacementResultRecord`, `PrefsRecord`, `PackRecord`.
  - `PersistentEventLog`: append-only SwiftData store conforming to the existing
    event-log surface (`append`, `append(contentsOf:)`, `events`, `count`). Rows are
    never mutated/deleted (I5); `KnownWordModel` stays a projection rebuilt by replay.
  - Optional disposable `ProjectionCheckpoint` row (last event seq + serialized model)
    added **only if** 50k replay misses the NFR-1 budget; the log stays source of truth.
  - Export (FR-10.3): JSON of the raw event log + records. Erase: delete all rows.
- `ZhuwenCore`: additive `EventSink` protocol (`append` / `replaceAll`) so
  `LearnerModel` can mirror appends into any store; in-memory `EventLog` conforms.
- `ZhuwenUI.LearnerModel`: optional `sink: EventSink?`; `record`/`eraseAll`/`import`
  forward to it. Default `nil` keeps every existing test green.
- **App target** vendored via **XcodeGen** (`ios/App/project.yml`): `ZhuwenApp`
  (`@main`, ModelContainer, wires `PersistentEventLog` → `LearnerModel`/`AppModel`,
  bundles the fixture pack) and `ZhuwenAppUITests`. Generated `.xcodeproj` git-ignored;
  `make app` runs xcodegen + xcodebuild. Package platform bumped to macOS 14 (SwiftData).

## Dependencies (handoff §0.4)
- **XcodeGen** (MIT) — build-time only, not shipped. Generates the app `.xcodeproj`.
- **SwiftData** — first-party Apple framework (no NFR-5 concern); requires macOS 14 /
  iOS 17 deployment. Package `platforms` bumped `.macOS(.v13)` → `.macOS(.v14)`.

## Tests
- **Launch-replay integration** (`ZhuwenPersistenceTests`, runs in CI): append a
  scripted history → capture projection → tear down + re-create `ModelContainer` from
  the same store URL (simulated relaunch) → rebuild `WordState`, assert equality with
  the pre-teardown projection, assert event count/order unchanged.
- **Append-only** assertion: no delete/update API path mutates existing rows.
- **50k replay budget** (`ReplayPerfTests`, skipped in fast CI, run in pre-push):
  rebuild from 50k events, measure, record in done note; add checkpoint iff over budget.
- **XCUITest smoke** (`ZhuwenAppUITests`, via `make app` / pre-push): place → read →
  lookup → relaunch app → lookup count + frontier state persist.

## Acceptance (MC-1)
`make app` builds from a clean clone with no manual Xcode steps; launch-replay
integration + XCUITest green; 50k replay measured and recorded; I5 invariant-map row
updated to "enforced + persistence-tested".
