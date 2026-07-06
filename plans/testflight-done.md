# TestFlight deployment (GitHub Actions) — done

## What shipped
`.github/workflows/testflight.yml` builds the in-repo `@main` app (`ios/App`, MC-1) and
uploads it to **TestFlight** for `guru.parso.zhuwen`. First successful upload:
run 28791736870 ("Upload succeeded").

## How it works (macOS runner)
1. **Select latest Xcode** + install XcodeGen; `xcodegen generate` the app project.
2. **Import** the Apple Distribution certificate into a temporary keychain (from
   `DIST_CERT_P12_BASE64` / `DIST_CERT_PASSWORD` / `KEYCHAIN_PASSWORD`).
3. Write the **App Store Connect API key** (`AuthKey_<id>.p8`).
4. **Archive** with **cloud/automatic signing** — `-allowProvisioningUpdates` +
   `-authenticationKey*` — so Xcode registers the app and creates the distribution
   provisioning profile on the fly (no pre-made profile needed). `DEVELOPMENT_TEAM`,
   `PRODUCT_BUNDLE_IDENTIFIER=guru.parso.zhuwen`, `CODE_SIGN_STYLE=Automatic`.
5. **Export** with `method=app-store-connect`, `destination=upload` → straight to TestFlight.

Build number is unique per run (`${GITHUB_RUN_NUMBER}$(date +%H%M)`); marketing version
comes from `ios/App/project.yml` (`MARKETING_VERSION`).

## Triggers
- `workflow_dispatch` (manual: `gh workflow run testflight.yml`), and
- push of a `v*` tag (release builds).

## App config for distribution (`ios/App`)
- Bundle id `guru.parso.zhuwen`; explicit `Info.plist` via XcodeGen with
  `ITSAppUsesNonExemptEncryption=false` (skips the export-compliance prompt) and
  `UISupportedInterfaceOrientations` (+`~ipad`) for the iPad/"1,2" device family.
- `AppIcon` asset (1024×1024, no alpha) in `Assets.xcassets`.
- Project-level signing stays OFF so simulator builds (`make app`, CI, XCUITests) need no
  credentials; the workflow overrides signing on the archive command line.

## Required repo secrets (set via `gh secret set`)
`APPSTORE_API_KEY_ID`, `APPSTORE_ISSUER_ID`, `APPSTORE_API_PRIVATE_KEY`,
`DIST_CERT_P12_BASE64`, `DIST_CERT_PASSWORD`, `KEYCHAIN_PASSWORD`, `APPLE_TEAM_ID`.

## Gotchas hit & fixed
- The distribution `.p12` used a modern MAC that Apple's `security import` rejects
  ("MAC verification failed") — re-exported with `openssl pkcs12 -export -legacy`.
- Secrets must be single-line: base64 the p12 with `| tr -d '\n'`.
- The runner's default Xcode (15.4) could not read the XcodeGen project format — pinned to
  the latest Xcode on a `macos-15` runner.
- iPad device family requires `UISupportedInterfaceOrientations` or App Store validation fails.
