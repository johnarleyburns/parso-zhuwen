# CP-03 — Done note (App Skeleton, iOS)

**Status:** complete. Factory `make ci` green (55 Go tests); iOS `swift test` green
(18 Swift tests); `ZhuwenUI` builds for the iOS 17 simulator (`make build-ios`).

## Handoff §6 acceptance
- **Tabs** — `ZhuwenUI.RootView`: Today · Library · Review · Progress (`TabView`, cinnabar tint).
- **PackStore verifying signatures** — `ZhuwenPacks.PackStore` verifies the vendored pack:
  minisign (CryptoKit Ed25519) over `manifest.json` → per-file SHA-256 → lexicon_version →
  content-level I6. Rejects the golden unsigned / tampered / imageless packs.
- **Reader renders a fixture story with tap-to-gloss** — `ZhuwenCore.ReaderModel` maps the
  pack body to display tokens and resolves taps to glosses; `ReaderView` renders them with a
  wrapping `FlowLayout`, cinnabar dotted underline on frontier words, and a gloss sheet.
- **Launch < 600 ms (NFR-1)** — deferred to the Xcode app-target instrumentation (CP-10 QA);
  the load path is a signature-verify + SQLite attach with no network.

## Deliverables (`ios/`)
- `Package.swift` — `ZhuwenPacks`, `ZhuwenCore`, `ZhuwenUI` + two test targets. No deps (NFR-5).
- **ZhuwenPacks:** `ZipArchive` (central-directory STORED reader), `Minisign`, `Models`,
  `ContentDatabase` (SQLite3, typed queries + `verifyI6`), `PackVerifier`, `PackStore`.
- **ZhuwenCore:** `Gloss`, `DisplayToken`, `ReaderModel`.
- **ZhuwenUI:** `AppModel`, `RootView`, `ReaderView`, `FlowLayout`, cinnabar `Color`.
- `ios/README.md`, `ios/Makefile`.

## Cross-cutting change
Factory pack builder now writes zip entries **STORED** (verification-neutral; lets the iOS
reader skip an inflate dependency). Shared `internal/fixtures` package + new `cmd/genfixtures`
produce the vendored positive pack **and** the three golden negatives deterministically
(`make fixtures`). `PACK_FORMAT.md` updated.

## Tests (unit / integration / e2e)
- **Unit:** `ZipArchiveTests` (read stored entries, reject non-zip); `MinisignTests` (verify
  real sig; reject tampered message / tampered trusted-comment / key_id mismatch);
  `ReaderModelTests` (token mapping, frontier flag, proper-noun/literal, gloss resolution).
- **Integration:** `PackVerifierTests` — accept positive; reject unsigned / tampered /
  imageless(I6) / unknown-lexicon; `PackStore` queries return 10 stories + 30 lexicon rows.
- **E2E:** `ReaderModelTests.testRendersRealFixtureStoryWithGlosses` — load the real vendored
  pack → render tokens → every in-lexicon word resolves a gloss → frontier 坚持 present.

Parity: the Swift `PackVerifier` runs against the **same golden files** the Go verifier
rejects (handoff §7: one vector set, two implementations).

## Exit criteria (standing)
- EC-1 README updated (root + `ios/README.md`). ✅
- EC-2 `make build` → `bin/` (factory: `zhuwenctl`, `genfixtures`). iOS uses `swift`/`xcodebuild`
  via `ios/Makefile` (no command binaries). ✅
- EC-3 green gates; unit + integration + e2e present. ✅

## Follow-ons
`@main` app target + XCUITest on simulator (Xcode project); real launch-time measurement
(CP-10). KnownWordModel/EventLog/Selector + Swift `CoverageGate` with the shared gate-vector
suite = CP-04.
