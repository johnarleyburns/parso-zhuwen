# CP-03 — App Skeleton (iOS)

**Refs:** handoff §5 (module contracts), §6 (CP-03 acceptance), §3 (pack format).
Depends on CP-02 (vendored `ios/Fixtures/`).

**Acceptance (handoff §6):** Tabs, PackStore verifying signatures, Reader renders a
fixture story with tap-to-gloss from pack data. Launch < 600 ms (NFR-1).

## Environment facts (verified)
- `import CryptoKit` → `Curve25519.Signing` (Ed25519) works via `swiftc`/`swift test`.
- `import SQLite3` works on macOS host and iOS.
- iOS 18 simulators present (iPhone 16). Pure-Swift packages test on macOS host with
  `swift test` — no simulator needed for the core.

## Design decisions
1. **STORED zip entries.** The Go pack builder writes `.zpack` entries with
   `zip.Store` (no compression). Still a standard zip; file hashes and the minisign
   signature are over *file contents*, so this changes nothing about verification. It lets
   the Swift side read packs with a tiny dependency-free, on-device-safe reader.
   (`PACK_FORMAT.md` updated; not a format break — container is still zip.)
2. **No SPM dependencies** (NFR-5). Ed25519 via CryptoKit; SHA-256 via CryptoKit; zip via
   a hand-rolled central-directory reader; SQLite via the system `SQLite3` module.

## Deliverables (`ios/`)
- `Package.swift` — library targets `ZhuwenPacks`, `ZhuwenCore`; test targets.
- **ZhuwenPacks** (owns pack I/O; will own the only URLSession later, I2):
  - `ZipArchive` — read-only STORED-zip reader (parses EOCD + central directory).
  - `Minisign` — parse pubkey, verify legacy `Ed` signature + trusted-comment global sig.
  - `PackVerifier` — signature → file sha256 → lexicon_version → content-level I6.
  - `PackStore` — verify, extract `content.sqlite`, typed queries (story, lexicon, image,
    question, body tokens).
- **ZhuwenCore** — domain models (`Word`, `StoryToken`, `Story`) + `ReaderModel`
  (segmented body → displayable tokens + tap-to-gloss lookup from pack lexicon).
- **ZhuwenUI** (SwiftUI, iOS 17) — `RootView` tab bar (Today/Library/Review/Progress) +
  `ReaderView` rendering a story with tappable words → gloss sheet, driven by `ReaderModel`.
  Compiled but UI-test on simulator is optional this CP (logic is covered on host).

## Test plan (`swift test`, host)
- **Unit:** `ZipArchive` (round-trip a known stored zip); `Minisign` (verify vs the vendored
  `zhuwen-dev.pub`; reject tampered/wrong-key); `ReaderModel` tap-to-gloss + frontier flag.
- **Integration:** `PackVerifier` against `ios/Fixtures/fixture-a2-v0.zpack` (accept) and
  against test-built malformed variants (unsigned/tampered/imageless → reject), mirroring
  the Go golden suite; `PackStore` typed queries return the 10 stories + lexicon rows.
- **E2E:** load the real vendored pack end-to-end → pick a story → `ReaderModel` produces a
  non-empty token stream and every word resolves to a gloss (the "Reader renders a fixture
  story with tap-to-gloss" acceptance, headless).

## Exit criteria (standing)
- EC-1 README updated (iOS build/test). EC-2 `make build`→`bin/` (factory unchanged; note
  iOS uses `swift build`/`swift test`). EC-3 green gates; unit+integration+e2e present.

## Out of scope (later CPs)
KnownWordModel/EventLog/Selector (CP-04); placement (CP-05); audio (CP-06); on-simulator
XCUITest for the SwiftUI shell (assembled in Xcode; tracked as follow-on).
