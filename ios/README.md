# Zhuwen iOS

SwiftUI app + local SPM packages (handoff §1, §5). iOS 17 floor. No third-party
dependencies (NFR-5): Ed25519 + SHA-256 via CryptoKit, SQLite via the system `SQLite3`
module, zip via an in-package reader.

## Packages
| Target | Role |
|--------|------|
| `ZhuwenPacks` | pack I/O: `ZipArchive` (STORED zip), `Minisign` verify, `PackVerifier` (sig + hashes + lexicon_version + content-level I6), `PackStore` typed SQLite queries + story audio extraction + `alignment(storyID:)`; **`PackClient`** (CP-08): the app's sole network chokepoint — anonymous ephemeral CDN GET via `PackFetcher`/`URLSessionPackFetcher`, verify-before-install, `PackCatalog`/`RemotePack`/`InstalledPack` pack manager (list/size/delete/redownload). Owns pack I/O + network (I2). |
| `ZhuwenCore` | domain models + `ReaderModel` (tap-to-gloss); the learner engine: `EventLog` (append-only, I5; incl. `.listen` blind/karaoke events), `KnownWordModel` (replayable projection, now with per-word `FSRSCard` memory), `FrontierQueue` (FR-2.4), `CoverageGate`+`StoryCandidate` (I1), `Selector` (bitmap AND + popcount scoring, NFR-2), `WordBitmap`, `LexiconStore`; **placement** (FR-1): `PseudowordGenerator` foils, `PlacementItemBuilder`/`PlacementSession`, `PlacementEstimator` (logistic fit + guessing correction → CEFR/HSK + seed), `PlacementSeed` (re-placement merge), `PlacementSnapshot`/`PlacementStore` (durable onboarding gate); **Foundations** (FR-11, CP-08a): `FoundationsProgram`/`FoundationsSet`/`FoundationsCard`, `FoundationsSession` (F0 introduce→recognize→read→bind, ends on an F1/F2 recombination pass), `FoundationsDeck` (FR-11.3 distractors), `HandoffGate` (F3: ≥20 A1 stories ≥98%, reuses the I1 formula), `startingSet(for:)` (FR-11.6); **loop completion** (CP-07): `FSRSScheduler` (open FSRS-4.5, on-device), `ComprehensionSession` (M8 → seal + P(known) boost), `ReviewScheduler`/`ReviewCard` (M9 sentence-context, 20/day cap), `ProgressEstimator`/`ProgressReport` (M10 both-skill CEFR + HSK gap + growth); **commerce & data** (CP-08): `Entitlement`/`StoreProduct`/`ProductCatalog` (FR-9.3 SKUs) + `FeatureGate` (free/Pro), `LearnerArchive` (export/erase/import round-trip, FR-10.3), `LearnerSettings` (FR-10.1, sync off by default). |
| `ZhuwenAudio` | listening (FR-5): `AlignmentTrack` (pure position→token resolver, the drift-critical core), `Karaoke` engine (speed 0.6×–1.2×, blind reveal), `CharTokenMap` (TTS range→token), `AudioNarrator` protocol + `PackAudioNarrator` (AVAudioPlayer, pitch-preserved rate) / `SystemTTSNarrator` (labeled `AVSpeechSynthesizer` fallback, §7) / `SystemVoice` (enhanced-voice detection). |
| `ZhuwenUI` | SwiftUI shell: `RootView` (onboarding gate → Foundations → tabs: Today · Library · Review · Progress) + Settings entry, `ReaderView` (tap-to-gloss sheet, cinnabar frontier underline, Listen + Finish→comprehension entries), `ListeningView` (M7 karaoke) + `ListeningModel`, `PlacementView` (M1–M3) + `PlacementFlowModel`, `FoundationsView` (M14 picture-word course) + `FoundationsModel` + attribution/`CreditsView` + `MethodologyView`, `ComprehensionView` (M8 seal stamp, Reduce-Motion fade), `ReviewView` (M9), `LearnerProgressView` (M10, "Pre-A1 · Foundations" until handoff), `LearnerModel` (owns the event log → projection → M8/M9/M10 + export/erase/import); **commerce & data** (CP-08): `StoreModel`+`EntitlementProvider` (StoreKit 2 wrapper, `#if canImport(StoreKit)`), `PaywallView` (M12), `SettingsView`+`PackManagerModel`+privacy page (M13), `SyncModel`+`LearnerSyncEngine` (`CloudKitSyncEngine` guarded, off by default), `AppModel` (owns the first-run onboarding route). |

The `@main` app target + on-simulator XCUITests are assembled in Xcode (thin shell over
the tested packages); all logic is covered by `swift test` on the host.

## Build & test
```sh
cd ios
make test        # swift test — unit + integration (shared gate vectors) + e2e (host, no simulator)
make bench       # NFR-2 selector benchmark under -c release (asserts the 50 ms gate)
make audit       # network-surface CI gate (I2): URLSession only in PackClient, no SDKs/http:///secrets
make build       # swift build (host)
make build-ios   # xcodebuild ZhuwenUI for the iOS 17 simulator
```

The coverage gate (I1) runs the same vectors as the Go factory gate
(`Fixtures/gate-vectors.json`), so the invariant can't drift between the two
implementations (handoff §7). The NFR-2 selector benchmark's 50 ms budget is only asserted
under `-c release` (an optimized build); the default debug `swift test` applies a coarse
regression guard and prints the measurement.

## Fixtures
`Fixtures/` holds golden artifacts vendored from the factory (regenerate with
`cd ../factory && make fixtures`):
- `fixture-a2-v0.zpack` + `zhuwen-dev.pub` — positive pack + verify key. Each story now
  ships stub audio (`audio/<id>.opus`) + word-level `alignment` rows (CP-06; real CosyVoice
  render is CP-09). The pack also ships Foundations F0 `foundations_card` rows (CP-08a) that
  resolve to provenanced images and drive the on-device Foundations engine + M14 UI.
- `golden-{unsigned,tampered,imageless}.zpack` — must be rejected by `PackVerifier`
  (mirrors the Go golden suite; handoff §7 — one vector set, two implementations).
- `gate-vectors.json` — the shared coverage-gate (I1) vector suite generated by the Go
  reference gate; asserted by both `go test ./internal/gatevec` and `swift test`.

## Status
- CP-03: PackStore verifies signatures + content-level I6; Reader renders a fixture story
  with tap-to-gloss (headless-tested; SwiftUI shell compiles for iOS 17). ✅
- CP-04: `EventLog`/`KnownWordModel` (replayable projection, I5), `FrontierQueue`,
  `CoverageGate`+`StoryCandidate` (I1, shared vectors with Go), `Selector` (NFR-2). ✅
- CP-05: placement (M1–M3): pseudoword foils, logistic fit → probabilistic seed of
  `KnownWordModel` + CEFR/HSK estimate, re-placement merge (FR-1.5); simulated learners
  recover the HSK band within ±1. `PlacementView` compiles for iOS 17. ✅
- CP-06: listening (M7): pack-audio karaoke synced to word-level `alignment`, speeds
  0.6×–1.2×, blind mode, labeled system-TTS fallback. Highlight drift < 120 ms across a
  3-min story (`KaraokeDriftTests`). `ListeningView` compiles for iOS 17. ✅
- CP-07: loop completion — comprehension → seal (M8), on-device FSRS review (M9,
  sentence-context cards, 20/day cap), Progress (M10) with **separate** reading & listening
  CEFR estimates. `KnownWordModel` folds review grades into per-word FSRS memory; state stays
  a replayable projection of the log (I5). P(known) updates verified for exposure/lookup/grade
  paths (`KnownWordModelTests`). M8/M9/M10 views compile for iOS 17. ✅
- CP-08: commerce & data — `PackClient` downloads additional packs over an anonymous, ephemeral
  CDN GET and **verifies them before install** (tampered/unsigned/imageless rejected, nothing
  lands on disk), backing a size/delete/re-download pack manager; StoreKit 2 SKUs (FR-9.3) behind
  a pure `FeatureGate` (free = full method at one story/day, Pro removes the throttle); paywall
  (M12) + settings (M13, FR-10.1) + privacy page; **export→erase→import round-trips** the exact
  `KnownWordModel` (`LearnerArchiveTests`, the acceptance); opt-in private-CloudKit sync off by
  default. `URLSession` confined to `PackClient`, enforced by `make audit` (`grep-audit.sh`, §8).
  All commerce/data views compile for iOS 17. ✅
- CP-08a: images/Foundations. The pack now ships Foundations F0 `foundations_card` rows resolving to
  fully-provenanced Commons images (I6 extended to `foundations_card`, `verifyFoundationsI6`).
  `ZhuwenCore.FoundationsProgram`/`FoundationsSession` drive the F0 four-step cycle and end each
  5–8 min sitting on an F1/F2 recombination pass (FR-11.4); every interaction folds into the one
  `KnownWordModel` (I5). `HandoffGate` reuses the I1 coverage formula (F3: ≥20 A1 stories ≥98%); a
  zero-knowledge learner reaches handoff in ~300 words (`FoundationsSimulationTests`, the CP-08a
  acceptance). `FoundationsView` (M14) renders photo + audio + hanzi + tone-colored pinyin with
  recognition grids, a long-press attribution sheet, and a full Credits screen (FR-11.2). First-run
  onboarding auto-presents placement, persists the result (`PlacementSnapshot`/
  `PersistentPlacementStore`), and routes complete/partial beginners into Foundations at their first
  unmastered set (FR-1.4/11.6) until the F3 handoff (FR-11.5); re-run from Settings still merges
  (FR-1.5). Methodology page states the §5A.4 honest limits (I4). `make audit` stays green (no new
  app network surface; Commons fetch is factory-only, I2). ✅
- CP-08+: scale-up, polish — pending.
