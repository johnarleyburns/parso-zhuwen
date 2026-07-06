# CP-08 — Done note (Commerce & data: CDN PackClient, StoreKit 2, paywall M12, settings M13, export/erase, optional CloudKit)

**Status:** complete. iOS `swift test` green (**147 Swift tests**, +23); `swift build` compiles the new
`ZhuwenPacks`/`ZhuwenCore`/`ZhuwenUI` types; `make build-ios` **BUILD SUCCEEDED** (the StoreKit- and
CloudKit-guarded paths cross-compile for the iOS 17 simulator); `make bench` green (NFR-2 unaffected,
~3.2 ms release); **`make audit` (`grep-audit.sh`) green** — the network-surface CI gate. Factory `make ci`
green (**untouched** — commerce/data are app-side, no schema change, no fixture regeneration).

## Handoff §6 acceptance
- **✅ Network-surface CI grep green.** `ios/grep-audit.sh` (handoff §8) asserts I2: `URLSession` appears
  **only** in `Sources/ZhuwenPacks/PackClient.swift`; no third-party analytics/crash/ads SDK names; no
  cleartext `http://` literals; no secrets written to `UserDefaults`. Wired as `make audit`.
- **✅ export → erase → import round-trips.** `LearnerArchiveTests.testExportEraseImportRoundTrips` builds a
  log across every state-touching event kind, exports it to JSON (`LearnerArchive` = ordered `Event` log +
  placement seed), erases, imports, and asserts the reprojected `KnownWordModel` **equals** the original —
  P(known), FSRS cards, and counts included. A companion UI-level test drives the same round trip through
  `LearnerModel.exportJSON → eraseAll → importJSON`.
- **CDN `PackClient` (FR-8.2/8.3).** Additional band packs download over an **anonymous, ephemeral**
  `URLSession` GET (no cookies/cache/credentials → no identifiers leave the device) and are **verified with
  the full signed chain (minisign → per-file sha256 → lexicon → I6) before install** — a tampered download
  is rejected and nothing lands on disk (`testTamperedDownloadIsRejectedAndNothingInstalled`). `http://` is
  refused (ATS/I2). A `PackCatalog`/`RemotePack`/`InstalledPack` surface backs the pack-manager UI
  (list/size/delete/re-download).
- **StoreKit 2 SKUs (FR-9.3).** `ProductCatalog` pins the three product ids/prices ($7.99/mo · $59.99/yr +
  30-day trial · $149.99 lifetime); `FeatureGate` is the pure free/Pro policy (free = placement, Foundations,
  **one engine-selected story/day**, dictionary, capped review, progress basics; Pro removes the throttle and
  opens lattice/listening/blind/dashboard/all-packs). `StoreModel` wraps an `EntitlementProvider`:
  `StoreKitEntitlementProvider` (`#if canImport(StoreKit)`, `Product.products`/`purchase`/
  `Transaction.currentEntitlements`, **no receipt server**) on device, `InMemoryEntitlementProvider` on host.
- **Paywall (M12, FR-9.4).** `PaywallView` — a single, factual, **dismissible** screen; three SKUs + Restore;
  presented only from Pro-gated affordances, never mid-story.
- **Settings (M13, FR-10).** `SettingsView` — FR-10.1 toggles (pinyin/underline/theme/font/voice/review cap),
  the pack manager (`PackManagerModel`), **Export / Erase** (share-sheet JSON + confirmed destructive erase),
  an **iCloud sync** toggle, and a plain-language **privacy page** stating the I2 network-surface guarantee.
- **Optional CloudKit sync (FR-10.2).** `SyncModel` + `LearnerSyncEngine`; a no-op default (sync **off**),
  and `CloudKitSyncEngine` (`#if canImport(CloudKit)`) syncing the `LearnerArchive` to the user's **private**
  DB only (learner state, never content). Off by default.

## Invariants preserved
- **I2 no accounts, no server state.** The only network surface is the isolated `PackClient` (anonymous CDN
  GET), StoreKit 2, and the opt-in private CloudKit sync — enforced mechanically by `grep-audit.sh`.
- **I3/I6 pregenerated + imageless-rejected.** Downloaded packs go through the *same* `PackVerifier` chain as
  vendored ones; the golden-tampered fixture is rejected on download exactly as on open.
- **I5 append-only, replayable.** Export/erase/import operate on the ordered event log + seed; nothing derived
  is persisted, so `export→erase→import` is the identity on learner state.
- **Schema frozen / factory untouched (CP-02, EC-2).** No schema change, no fixture regeneration; factory
  `make ci` and `make build`→`bin/` are unchanged.

## Deliverables
- **ZhuwenPacks:** `PackClient.swift` (`PackFetcher`/`URLSessionPackFetcher`, `PackCatalog`/`RemotePack`/
  `InstalledPack`, catalog/download+verify/install/list/size/delete/redownload).
- **ZhuwenCore:** `Entitlements.swift` (`Entitlement`, `StoreProduct`/`ProductCatalog`, `FeatureGate`);
  `LearnerArchive.swift` (Codable export/decode/project); `LearnerSettings.swift` (FR-10.1, sync off).
- **ZhuwenUI:** `StoreModel.swift` (+`EntitlementProvider`, StoreKit-guarded impl); `PaywallView` (M12);
  `SettingsView` + `PackManagerModel` + `PrivacyView`/`PackLibraryView`/`JSONDocument` (M13);
  `SyncModel.swift` (+`LearnerSyncEngine`, CloudKit-guarded impl); `LearnerModel` export/erase/import;
  `AppModel` wiring + `RootView` Settings entry.
- **Tooling:** `ios/grep-audit.sh` + Makefile `audit` target.

## Tests (unit / integration / e2e)
- **`PackClientTests`** — catalog parse; download **verifies + installs** the vendored pack (opens in
  `PackStore`); tampered download **rejected**, nothing installed; `http://` refused; anonymous GET (bare URL,
  no query); delete removes + updates `installedPacks()`; redownload reinstalls.
- **`EntitlementTests`** — free one-story/day gate, Pro unthrottled, Pro-only surfaces, review-cap split, and
  the FR-9.3 product catalog (ids + prices + 30-day annual trial).
- **`LearnerArchiveTests`** (acceptance) — export→erase→import equals the original model (FSRS included); JSON
  stable/Codable; empty imports empty; future schema + malformed rejected. **`LearnerSettingsTests`** —
  defaults (sync off) + Codable round-trip.
- **`ZhuwenUITests`** — `StoreModel` free→Pro purchase flow; `SyncModel` off-by-default no-op; the
  `LearnerModel` export→erase→import round trip.

## Exit criteria (standing)
- [x] Handoff §6 acceptance for CP-08 met (network-surface CI grep green; export→erase→import round-trips).
- [x] EC-1 READMEs updated (root + `ios/`) with status + commands (incl. `make audit`).
- [x] EC-2 factory `make build`→`bin/` unchanged; `make ci` still green.
- [x] EC-3 `swift test` green (unit + integration + e2e/acceptance); `make build-ios` succeeds; `make bench`
      green; `make audit` green.

## No new dependencies
StoreKit 2 and CloudKit are first-party frameworks behind `canImport` guards; `PackClient` uses Foundation
`URLSession`. All commerce/data logic is pure and host-testable. Factory untouched.

## Follow-ons (not CP-08)
Real CDN endpoint + minisign key custody (§10.3); StoreKit sandbox/device purchase QA and App Store product
ids (CP-10); live CloudKit schema + conflict policy hardening (CP-10); §8A images + Foundations F0–F3
(CP-08a, next).
