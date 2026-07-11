# TestFlight deployment (GitHub Actions) — done

## What shipped
`.github/workflows/testflight.yml` builds the in-repo `@main` app (`ios/App`, MC-1) and
uploads it to **TestFlight** for `guru.parso.zhuwen`. First successful upload:
run 28791736870 ("Upload succeeded").

## How it works (macOS runner)
1. **Select latest Xcode** + install XcodeGen; `xcodegen generate` the app project.
2. **Import** the Apple Distribution certificate into a temporary keychain (from
   `DIST_CERT_P12_BASE64` / `DIST_CERT_PASSWORD` / `KEYCHAIN_PASSWORD`).
3. **Install** the pre-made App Store provisioning profile (`PROVISIONING_PROFILE_BASE64`)
   into `~/Library/MobileDevice/Provisioning Profiles`; its UUID + name are read out of the
   decoded profile so nothing is hardcoded.
4. Write the **App Store Connect API key** (`AuthKey_<id>.p8`) — used only for the upload.
5. **Archive** with **MANUAL signing** — `CODE_SIGN_STYLE=Manual`,
   `CODE_SIGN_IDENTITY="Apple Distribution"`, `PROVISIONING_PROFILE_SPECIFIER=<profile name>`.
   **No `-allowProvisioningUpdates`, no Automatic signing** — xcodebuild is never allowed to
   create/mint a new cert or profile.
6. **Export** with `signingStyle=manual`, `signingCertificate=Apple Distribution`, and an
   explicit `provisioningProfiles` mapping → uploads straight to TestFlight.

> **Why manual:** the previous cloud/automatic path (`-allowProvisioningUpdates`) minted a
> fresh distribution certificate on every run, eventually hitting the account's certificate
> cap ("Your account has reached the maximum number of certificates"). Manual signing reuses
> the one pre-made cert + profile forever.

Build number is `git rev-list --count HEAD` (monotonic per commit); marketing version is
`1.0.$BUILD`.

## Triggers
- `workflow_dispatch` (manual: `gh workflow run testflight.yml`), and
- push of a `v*` tag (release builds).
- `ci.yml` also builds + uploads on every push to `main` (same manual-signing steps).

## App config for distribution (`ios/App`)
- Bundle id `guru.parso.zhuwen`; explicit `Info.plist` via XcodeGen with
  `ITSAppUsesNonExemptEncryption=false` (skips the export-compliance prompt) and
  `UISupportedInterfaceOrientations` (+`~ipad`) for the iPad/"1,2" device family.
- `AppIcon` asset (1024×1024, no alpha) in `Assets.xcassets`.
- Project-level signing stays OFF so simulator builds (`make app`, CI, XCUITests) need no
  credentials; the workflow overrides signing on the archive command line.

## Required repo secrets (set via `gh secret set`)
`APPSTORE_API_KEY_ID`, `APPSTORE_ISSUER_ID`, `APPSTORE_API_PRIVATE_KEY`,
`DIST_CERT_P12_BASE64`, `DIST_CERT_PASSWORD`, `PROVISIONING_PROFILE_BASE64`,
`KEYCHAIN_PASSWORD`, `APPLE_TEAM_ID`.

### One-time: create + upload the provisioning profile (manual steps)
`PROVISIONING_PROFILE_BASE64` holds the **"Zhuwen App Store Profile"** (`guru.parso.zhuwen`,
team `3264Y8YUGV`, expires 2027-05-06), signed against the same Apple Distribution cert as
`DIST_CERT_P12_BASE64` (fingerprint verified to match). It is already set. To regenerate it
against the **existing** distribution cert (never create a new cert):
1. Apple Developer portal → **Certificates, IDs & Profiles → Profiles → +**, choose
   **App Store Connect** distribution, App ID `guru.parso.zhuwen`, and pick the Apple
   Distribution cert that matches `DIST_CERT_P12_BASE64`. Download the `.mobileprovision`.
2. Base64-encode single-line and store:
   ```sh
   base64 -i Zhuwen_App_Store_Profile.mobileprovision | tr -d '\n' | \
     gh secret set PROVISIONING_PROFILE_BASE64
   ```
3. If the account's certificate limit is maxed (the failure that motivated this change),
   revoke the stale auto-minted certs in the portal, keeping the one whose private key is in
   `DIST_CERT_P12_BASE64`.

## Gotchas hit & fixed
- The distribution `.p12` used a modern MAC that Apple's `security import` rejects
  ("MAC verification failed") — re-exported with `openssl pkcs12 -export -legacy`.
- Secrets must be single-line: base64 the p12 with `| tr -d '\n'`.
- The runner's default Xcode (15.4) could not read the XcodeGen project format — pinned to
  the latest Xcode on a `macos-15` runner.
- iPad device family requires `UISupportedInterfaceOrientations` or App Store validation fails.
