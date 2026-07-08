# CP-09c — done

**Date:** 2026-07-07
**Status:** ✅ Complete (code + tooling); B-4/B-5 owner-in-the-loop items carried per plan
**Refs:** `plans/cp-09c-plan.md`, `plans/cp-09c-image-inventory.md`, `plans/blockers.md` B-4/B-5

Parts A–D of the CP-09c plan delivered. Invariants I2/I3/I4/I6 + NFR-3/NFR-4 preserved.

## Part A — CosyVoice render as an external build-time stage (`internal/tts`)
- ✅ New `internal/tts` package. `Render(tokens, storyID, cfg) → (opusBytes, wordTimings,
  durationMs, provenance)` with two modes (handoff §1 external-stage pattern):
  - **`ModeStub`** — deterministic, hermetic (no network / no venv, I2). Timings from the
    `internal/align` character-rate model; audio bytes a SHA-256 keystream seeded by story ID
    (identical input → identical bytes). Compact by default so vendored fixtures stay tiny;
    `RepresentativeStub` emits realistic 24 kbps weight for the size-budget exercise.
  - **`ModeReal`** — shells out to a local CosyVoice 3.0 render + forced aligner (Apple
    Silicon MPS, build-time only, $0; never linked into the app — I2/I3). Real word timings
    **replace** the character-rate output under the identical `pack.AlignToken` contract,
    validated by `ValidateTimings` before shipping. Records tool/model/voice/rate provenance.
- ✅ 24 kbps mono Opus is the NFR-4 basis; `StubByteLen(ms, bitrate)` is the shared weight
  model used by the size budget.
- ✅ Wired into `internal/pipeline` (`Config.TTS`, default = stub): both the simple and repair
  loop paths now render via `tts.Render`, flowing `AudioData` + `Alignment` into `pack.Story`.
  No app change; no pack schema change (SchemaVersion stays 1).
- ✅ Tests (`internal/tts/tts_test.go`): stub determinism, per-story distinctness, contract
  satisfaction, representative sizing, provenance, `ValidateTimings` rejections, ModeReal
  config guard. Hermetic — CI never shells out.

## Part B — NFR-3/NFR-4 size budgets, measured + build-failing (`internal/pack`)
- ✅ `pack.MeasureBudget(p) → Budget{DownloadBytes, OnDiskBytes, AudioBytes, ImageBytes,
  SQLiteBytes, OtherBytes, PerFile}` over the real assembled file set (audio + images +
  sqlite + manifest). `assembleFiles` factored out of `Build` so the budget measures exactly
  what ships.
- ✅ `pack.CheckBudget` enforces **NFR-3** (download ≤ 90 MB) and **NFR-4** (on-disk ≤ 250 MB);
  **`Build` calls it — an over-budget pack fails the build** (the acceptance gate).
- ✅ Build-failing test (`internal/pack/budget_test.go`): a realistic A1+A2 pack (40 stories ×
  3 min × 24 kbps + HEIC covers) passes; an NFR-4-breach pack and an NFR-3-breach pack both
  fail the build; subtotal classification asserted.
- ✅ `zhuwenctl budget [--pack <zpack>]` subcommand (EC-2) reports the figures.

## Part C — Alignment re-verification against real audio (Swift)
- ✅ Vendored real-render alignment fixture `ios/Fixtures/real-render-alignment.json` — a
  ~3-minute story (534 tokens) with **non-uniform** per-word durations and variable sentence
  pauses (the real forced-aligner shape, unlike the synthesized uniform track).
- ✅ `KaraokeDriftTests` extended: `testHighlightDriftUnder120msWithRealRenderAudio` re-runs
  the drift measurement on the real-render timings — **< 120 ms at every supported speed**
  (0.6×–1.2×). Kept pre-push-gated (`make test-unit` skips `KaraokeDriftTests`); the synthetic
  3-min drift test stays in CI.

## Part D — B-4 at scale: Commons curation toward ~570 images (owner-in-the-loop)
- ✅ `plans/cp-09c-image-inventory.md` — the batching plan: ~568-image demand enumerated
  (217 Foundations F0 + 81 canon covers + ~40 authored + ~230 B1), grouped into
  owner-reviewable batches with a status ladder (`todo → candidates → reviewed → signed-off →
  shipped`). F0 batches marked signed-off (curated in CP-08a).
- ✅ Per-image license sign-off recorded, not assumed (I4/FR-11.2): `images.Provenance` gains
  `SignedOff/SignedBy/SignedAt`; `images.DecisionsToImagesSignedOff` rejects an unsigned cover
  (staging `DecisionsToImages` still accepts). Hermetic test drives both paths.
- ✅ Ship-readiness gate unchanged in code: `pack.validateI6` / `verifyI6` block
  imageless/AI/unprovenanced covers; the inventory doc is the human tracker.

## Docs
- ✅ Root `README.md` (CP-09c row + CosyVoice/size notes), `factory/data/README.md` (CosyVoice
  run instructions + voice/model provenance + size figures), this done note, image inventory.

## Carried (owner-in-the-loop, per plan)
- **B-4** — real Commons curation at ~570 scale. Tooling + sign-off gate shipped; the actual
  per-image human sign-off for canon/authored/B1 batches is owner work, tracked in the
  inventory doc. Must be **closed for the launch pack set** before CP-10 submission.
- **B-5** — CosyVoice tooling + voice-model license. `ModeReal` shipped behind the flag; the
  owner records render tool version + voice-model license in `factory/data/README.md` before
  the real stage ships. Stub keeps CI/dev unblocked.

## Verification
- `make ci` (factory: fmt + vet + `go test ./...`) green, incl. `internal/tts` and the
  build-failing size-budget test.
- `swift test` green; `KaraokeDriftTests` (pre-push) green with real-render audio.
- `make audit` (I2 network-surface gate) green — no new app network surface (TTS is
  build-time only, never linked into the app).
