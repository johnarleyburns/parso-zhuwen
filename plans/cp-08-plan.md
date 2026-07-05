# CP-08 — Commerce & data: CDN PackClient, StoreKit 2 SKUs, paywall (M12), settings (M13), export/erase, optional CloudKit sync

**Refs:** handoff §6 (CP-08 acceptance: *Network-surface CI grep green; export→erase→import round-trips*),
§8 (CI gates — `grep-audit.sh`: URLSession outside `PackClient`, analytics SDK names, `http://`,
secrets-in-storage). `00` §2 I2 (no accounts/server state; network limited to anonymous CDN GET + StoreKit 2
+ optional private CloudKit), §4 FR-8 (packs/offline, M13), FR-9 (monetization, M12), FR-10 (settings & data,
M13). Mockups M12 (paywall), M13 (settings & packs). Depends on CP-02 (`PackVerifier`/`Minisign` signature
chain reused by the downloader), CP-07 (`LearnerModel` event log = the exported state).

**Acceptance (handoff §6):** CDN `PackClient`, StoreKit 2 SKUs (FR-9.3), paywall (M12), settings, export/erase,
optional CloudKit sync. ✅ **Network-surface CI grep green; export→erase→import round-trips.** Plus standing
exit criteria (EC-1 READMEs, EC-2 factory `make build`→`bin/` untouched, EC-3 `make ci`/`swift test` green).

## Invariants in play
- **I2 no accounts, no server state.** All learner data stays on device. The **only** network surface is:
  (1) anonymous CDN pack GETs, isolated in `ZhuwenPacks/PackClient`; (2) StoreKit 2 (Apple, no receipt
  server); (3) *optional*, off-by-default private CloudKit sync of learner state only. `URLSession` appears
  **only** inside `PackClient.swift`; enforced by `grep-audit.sh` (§8). No analytics/crash SDKs (NFR-5),
  no `http://`, no secrets in `UserDefaults`.
- **I3 pregenerated only / I6 imageless-rejected.** Downloaded packs are verified with the *same*
  `PackVerifier` chain (minisign → per-file sha256 → lexicon acceptance → I6) before install — a tampered or
  imageless download is rejected exactly like a vendored one (reuse the golden fixtures).
- **I5 append-only, replayable.** Export is the ordered `Event` log + placement seed serialized to JSON;
  import replays it into the identical `KnownWordModel`. Erase clears the log. `export→erase→import` is the
  identity on learner state (the CP-08 acceptance test).
- **No new dependencies** (NFR-5): StoreKit 2 / CloudKit are first-party and behind availability guards;
  `PackClient` uses `URLSession` (Foundation). Pure commerce/data logic is host-testable.

## Design decisions
1. **`ZhuwenPacks/PackClient` — the single network chokepoint.**
   - `protocol PackFetcher { func fetch(_ url: URL) async throws -> Data }` abstracts I/O so tests inject a
     local/in-memory fetcher (no sockets in CI). `URLSessionPackFetcher` uses an **ephemeral** session
     (no cookies/cache/credentials) and a bare GET — anonymous, no identifiers (FR-8.3).
   - `PackCatalog`/`RemotePack` (Codable) describe available packs (id, band, semver, sizeBytes, url).
   - `PackClient.catalog()` fetches+parses the catalog; `download(_:to:)` GETs a `.zpack`, **verifies** it
     with `PackVerifier` + the pinned `Minisign.PublicKey` before writing it into the packs directory
     (verify-before-install); `installedPacks()`/`sizeOnDisk(_:)`/`delete(_:)`/`redownload(_:)` back the
     pack-manager UI (FR-8.3). Only `https://` URLs accepted; `http://` throws.
2. **`ZhuwenCore/Entitlements` — pure free/Pro gating (FR-9).** `ProductCatalog` carries the three SKUs
   (FR-9.3: monthly `$7.99`, annual `$59.99` + 30-day trial, lifetime `$149.99`) by stable product id.
   `Entitlement` = `.free | .pro`. `FeatureGate(entitlement:, dailyStoryCount:)` answers FR-9.1/9.2:
   free = placement + Foundations + **one engine-selected story/day** + dictionary + **capped** review +
   progress basics; Pro = unlimited stories, lattice browsing, listening packs & blind mode, full dashboard,
   monthly checkpoints, all packs. Deterministic; the StoreKit layer only *supplies* the `Entitlement`.
3. **`ZhuwenCore/LearnerArchive` — export/erase/import.** `LearnerArchive: Codable`
   `{ schemaVersion, exportedAt, events:[Event], seed:[Int:Double] }`. `encode()`→pretty JSON `Data`;
   `decode(_:)`→archive; `KnownWordModel.project(archive.events, seed:)` reproduces state. This is the
   FR-10.3 "export everything (JSON) / erase everything" and the round-trip acceptance.
4. **`ZhuwenCore/LearnerSettings` — FR-10.1 toggles** (Codable, sensible defaults): pinyin mode, frontier
   underline, theme, reader font size, audio voice (pack vs system TTS), daily review cap, **iCloud sync off
   by default** (FR-10.2). Pure model; the view binds to it and the app persists it.
5. **`ZhuwenUI` thin layers (host-compilable, iOS features behind availability):**
   - `EntitlementProvider` protocol + `StoreModel` (`@MainActor`): a default in-memory provider (free) for
     host/tests; `StoreKitEntitlementProvider` (`#if canImport(StoreKit)`) loads `Product`s, runs
     `purchase`, and derives `Entitlement` from `Transaction.currentEntitlements`. No receipt server (FR-9.3).
   - `PaywallView` (M12, FR-9.4): single, factual, **dismissible** screen; three SKUs + Restore; never shown
     mid-story (callers gate on non-reading surfaces only).
   - `SettingsView` (M13, FR-10): the FR-10.1 toggles, the **pack manager** (`PackManagerModel` over
     `PackClient`: size/delete/re-download), **Export / Erase** (share-sheet JSON + destructive erase with
     confirm), an **iCloud sync** toggle (off by default), and a **privacy page** stating the I2
     network-surface guarantee in plain language.
   - `LearnerSyncEngine` protocol + no-op default; `CloudKitSyncEngine` (`#if canImport(CloudKit)`) pushes/
     pulls the event log to the user's **private** DB only (learner state, never content), gated by the
     off-by-default toggle.
   - `LearnerModel` gains `exportArchive()`, `importArchive(_:)` (replace-and-reproject), `eraseAll()`.
   - `RootView`: a Settings entry (toolbar) + paywall presentation from Pro-gated affordances.
6. **`grep-audit.sh` (§8) + `make audit`.** Fails the build if: `URLSession` is referenced outside
   `Sources/ZhuwenPacks/PackClient.swift`; any known analytics/crash SDK name appears; an `http://` literal
   appears; or a secret-ish key is written to `UserDefaults`. Wired as `make audit`; part of the CP gate.
7. **Factory untouched.** No schema change, no fixture regeneration; commerce/data are app-side. The factory
   `make ci` stays green and `make build`→`bin/` is unchanged (EC-2).

## Tasks
1. `ZhuwenPacks/PackClient.swift` — `PackFetcher`, `URLSessionPackFetcher`, `PackCatalog`/`RemotePack`,
   `PackClient` (catalog/download+verify/install/list/size/delete/redownload).
2. `ZhuwenCore/Entitlements.swift` — `Entitlement`, `Product`/`ProductCatalog` (FR-9.3 SKUs), `FeatureGate`.
3. `ZhuwenCore/LearnerArchive.swift` — `LearnerArchive` (Codable), `encode`/`decode`, projection helper.
4. `ZhuwenCore/LearnerSettings.swift` — `LearnerSettings` (Codable, defaults; sync off).
5. `ZhuwenUI` — `StoreModel`+`EntitlementProvider` (+ StoreKit-guarded impl); `PaywallView` (M12);
   `SettingsView` + `PackManagerModel`/`PackManagerView` + privacy page (M13); `SyncModel`+`LearnerSyncEngine`
   (+ CloudKit-guarded impl); `LearnerModel.export/import/erase`; wire Settings/paywall in `RootView`.
6. `ios/grep-audit.sh` + Makefile `audit` target; run in the CP gate.
7. Docs — root + `ios/` READMEs (CP-08 ✅); `plans/cp-08-done.md`.

## Tests (unit / integration / e2e)
- **`PackClientTests`** — catalog JSON parse; `download` of the vendored `fixture-a2-v0.zpack` (served by an
  in-memory/file `PackFetcher`) **verifies + installs**, and the installed pack opens in `PackStore`; a
  **tampered** download (golden-tampered) is **rejected** and nothing is installed; `http://` URL throws;
  `delete` removes the file and updates `installedPacks()`; the fetcher is asked for a bare URL (no query
  identifiers) — anonymous GET.
- **`EntitlementTests`** — free gates: one story/day (second same-day story blocked), review cap applied, no
  lattice/blind/listening-packs; Pro unlocks all; `ProductCatalog` exposes exactly the FR-9.3 ids & prices.
- **`LearnerArchiveTests` (acceptance)** — `testExportEraseImportRoundTrips`: build a log (exposures, lookups,
  a review grade, a comprehension pass, a listen), export→JSON, **erase** (empty), **import**, and assert the
  reprojected `KnownWordModel` (P(known), FSRS cards, counts) **equals** the original; JSON is stable/Codable;
  a foreign/empty archive imports to an empty model.
- **`LearnerSettingsTests`** — Codable round-trip; defaults (sync **off**, pack-voice, sane cap).
- **CI gate** — `make audit` (`grep-audit.sh`) green: URLSession only in `PackClient.swift`.

## No new dependencies
StoreKit 2 and CloudKit are first-party frameworks used behind `canImport` guards; `PackClient` uses
Foundation `URLSession`. All commerce/data logic is pure and host-testable. Factory untouched.

## Follow-ons (not CP-08)
Real CDN endpoint + minisign key custody (§10.3); StoreKit sandbox/device purchase QA and the App Store
product ids (CP-10); §8A images + Foundations F0–F3 (CP-08a); live CloudKit schema + conflict policy (the
protocol/off-by-default toggle land here, the production sync hardening is a ship-polish item).
