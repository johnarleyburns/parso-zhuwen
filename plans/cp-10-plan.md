# CP-10 — Ship polish: final QA pass + App Store submission prep

**Refs:** `plans/cp-09-plan.md` §Follow-ons ("CP-10 ship polish: accessibility, App Store assets,
HSK-3.0 launch messaging, methodology page with citations (I4), full-device QA checklist"),
`plans/testflight-done.md` (TestFlight upload already wired: `guru.parso.zhuwen`), `00`
§FR-8/§FR-11 (packs, attribution/credits), NFR-1/NFR-2 (device perf), NFR-3/NFR-4 (sizes, closed in
CP-09c), NFR-5 (no third-party SDKs), I2 (privacy/network surface), I4 (evidence-gated claims).
Depends on: **CP-09c complete** (real audio + measured size budgets + B-4 covers signed off) — a
canon entry blocked on imagery or over budget does not ship.

## Goal / one-liner
Turn a green, correct build into a submittable App Store release: accessibility, store assets +
metadata, launch messaging that is honest (I4), a full-device QA pass, and the App Store Connect
submission itself. This is the "whatever didn't fit in 09c" bucket plus the ship mechanics.

## Preconditions (must be true before CP-10 exits)
- CP-09c done: real CosyVoice Opus + real alignment shipped; NFR-3/NFR-4 build-failing test green.
- **B-4 cleared for the launch pack set** — every shipped story has a signed-off, gate-passing,
  provenanced Commons cover (I6). Fixture stand-ins are *not* shippable.
- I1 coverage gate + shared Go/Swift gate-vector suite green; `make audit` (I2) green.

## Invariants in play
- **I2 — privacy / network surface.** App network is only anonymous pack CDN GET + StoreKit 2 +
  optional private-CloudKit sync. Privacy manifest declares zero tracking; ATS pinned to the pack CDN.
  `make audit` grep-gate stays green through any polish change.
- **I4 — evidence-gated claims.** Store copy, methodology page, and launch messaging cite real
  sources (HSK-3.0 provenance, audit pass-rates in manifests, CC attribution). No fabricated metrics,
  no unverifiable marketing claims.
- **I6 — provenanced imagery.** Credits/attribution screen lists every shipped image's license +
  author + source (FR-11.2), generated from pack provenance, not hand-maintained.
- **NFR-5 — no third-party SDKs.** No analytics/marketing SDK sneaks in during store prep.

## Part A — Accessibility pass
1. **Dynamic Type** to XXL / accessibility sizes across reading, Foundations, karaoke, and pack-manager
   screens; no truncation/overlap; reading view remains usable at largest sizes.
2. **VoiceOver** labels/traits/order on all interactive controls; karaoke highlight + audio transport
   are operable and announced; Foundations cards have meaningful labels.
3. **Contrast + reduce-motion** honored (karaoke highlight has a non-motion affordance); test on a
   physical device with the accessibility inspector. Record results in `plans/cp-10-qa.md`.

## Part B — Full-device QA pass (`plans/cp-10-qa.md`)
4. **Device matrix.** Run on the iOS-17 floor device + a current device + an iPad (orientation already
   declared in TestFlight). First-run onboarding → Foundations F0→F3 handoff → first story read with
   real audio + karaoke → pack download/delete (FR-8.3) → offline read after download.
5. **Perf (NFR-1/NFR-2).** Verify launch + reading interaction budgets on a real device (benchmark
   regression per handoff §1); no jank in karaoke at supported speeds.
6. **Edge cases.** Airplane mode (no-audio "System voice" fallback, `00` §voice fallback), low storage,
   pack signature failure (tampered pack rejected), iCloud sync on/off. Checklist with pass/fail +
   device/OS recorded (I4 honesty).

## Part C — App Store assets + metadata
7. **Store assets.** App icon (all sizes), screenshots per required device class (reading view,
   Foundations, karaoke, methodology/credits), optional preview. App Store description, keywords,
   subtitle, category, age rating, privacy "nutrition label" matching the privacy manifest (I2).
8. **Legal/attribution surfaces.** In-app Credits screen (image + voice + HSK-3.0 attribution from
   provenance) reachable and complete; `NOTICE-APP-STORE.md` current; license memo
   (`plans/cp-09-license-memo.md`) reflects the final shipped canon + image set.
9. **Launch messaging (I4).** HSK-3.0 alignment framed honestly (coverage-gated, audit pass-rates
   recorded in manifests); methodology page (CP-08a) citations current. No claim we can't evidence.

## Part D — Submission mechanics
10. **Build + upload.** Reuse the TestFlight pipeline (`plans/testflight-done.md`,
    `guru.parso.zhuwen`) for the release build; verify version/build bump, entitlements, privacy
    manifest, and starter-content embed within NFR-3 download budget.
11. **App Store Connect submission.** Fill metadata, attach assets, answer export-compliance +
    content-rights (attribution) + privacy questions, submit for review. Capture the submission
    checklist + any review feedback loop in `plans/cp-10-done.md`.

## Tasks
- A: accessibility (Dynamic Type XXL, VoiceOver, contrast/reduce-motion) across all screens.
- B: `plans/cp-10-qa.md` device-matrix checklist; run it; record results honestly.
- C: store assets + metadata + privacy label; Credits/attribution completeness; launch copy (I4).
- D: release build via existing TestFlight pipeline; App Store Connect submission; `plans/cp-10-done.md`.
- Docs: root README (ship status), `ios/README.md` (accessibility/QA notes), `NOTICE-APP-STORE.md`
  refresh, `plans/cp-10-qa.md`, `plans/cp-10-done.md`.

## Tests / verification
- **Accessibility (Swift, where automatable):** snapshot/label assertions for key screens at
  accessibility text sizes; manual device pass recorded in `plans/cp-10-qa.md`.
- **I2 audit:** `make audit` green after all store-prep changes; privacy manifest ↔ store privacy
  label consistency check.
- **Regression:** `make ci` (factory) + `swift test` green; I1 gate-vector + pack golden suites green;
  NFR-3/NFR-4 build-failing size test (CP-09c) stays green on the release pack set.
- **Provenance:** Credits screen enumerates every shipped image/voice/HSK license from pack
  provenance; a story with an unprovenanced cover fails the build (I6, `pack.validateI6`).

## Dependencies (handoff §0.4) — owner-in-the-loop
- **App Store Connect account + signing** (owner) — metadata, assets, export-compliance answers,
  submission are owner actions; agents prepare assets, checklists, and the release build.
- **B-4 launch set signed off** (from CP-09c) — hard precondition; shipping on fixture covers violates
  I6.
- **CosyVoice voice license** (B-5) recorded and shippable.

## Blockers carried
- **B-4** — must be *closed for the launch pack set* before submission (Part 0 precondition).
- **B-5** — CosyVoice voice-model license cleared for distribution of rendered audio.

## Follow-ons (post-launch, not CP-10)
- Broader B2 content; multi-voice CosyVoice; automated (non-sampled) audit; additional bands/packs via
  the CDN pack-delivery path (FR-8.2) without an app update.
