# CP-09c — CosyVoice render (local, $0) + NFR-3/NFR-4 size budgets + alignment re-verify; B-4 at scale

**Refs:** `plans/cp-09-plan.md` Parts C & E (deferred from CP-09b, see `plans/cp-09b-done.md`
§"Deferred to CP-09c"), handoff §1 (external build-time stage pattern: TTS/aligner/HEIC shelled out,
deterministic stub in CI), §4.7 (TTS/alignment contract), §6 (CP-06 karaoke drift <120 ms; CP-09
acceptance: *pack sizes within NFR-3/4*), `00` NFR-3 (≤90 MB download), NFR-4 (A1+A2 ≤250 MB on disk;
Opus 24 kbps mono ≈180 KB/min), §8A (image inventory ~570). Depends on: CP-06 `internal/align` +
`ios/…/ZhuwenAudio` karaoke; CP-09b `pack.Manifest` + audit; CP-08a `internal/images` curate/join.

## Goal / one-liner
Replace the last fixture stubs on the critical path — **stub audio bytes** and **assumed pack
weights** — with real, measured artifacts, and make the size budget a **build-failing test** rather
than an assumption. Re-verify karaoke drift against *real* audio. Advance **B-4** (owner-in-the-loop
Commons curation) from F0 (11 words) toward the ~570-image launch inventory that gates shipping the
81 canon entries + A1–B1 packs.

## Invariants in play
- **I2 — app network surface unchanged.** CosyVoice runs **build-time only**, never linked into the
  app; CI stays hermetic via the deterministic stub encoder (no network, no venv). `make audit` green.
- **I3 — app never computes timing.** Real alignment rows are produced in the factory and shipped in
  the pack; the app only reads them. The CosyVoice render *replaces the timing values* under the same
  one-row-per-token contract `internal/align` already emits — no app-side change.
- **I4 — evidence-gated claims.** Measured download/on-disk sizes are recorded honestly (real Opus +
  real HEIC weights), not assumed; the CP-08a image-weight estimate is rechecked against reality.
- **I6 — every story has a provenanced, non-AI Commons image.** No canon entry / pack ships until its
  cover clears the §8A gate with a per-image license sign-off. B-4 throughput paces what can ship.
- **NFR-3/NFR-4 — size budgets are asserted, not assumed.** A pack that breaches fails the build.

## Part A — CosyVoice render as an external build-time stage (`internal/assets` / pipeline)

The CP-06 stub bytes (`pack.go:216` `ZHUWEN-FIXTURE-AUDIO:<id>`) become a real render. Same shape as
the aligner/HEIC encoder (handoff §1): shell out behind a config flag; deterministic stub for CI.

1. **`internal/tts` render stage.** New package wrapping a CosyVoice 3.0 invocation as an external
   process (Python venv, local, Apple Silicon MPS, **$0** — no hosted API). Interface:
   `Render(text, voice, cfg) → (opusBytes, wordTimings)`. Config flag selects **real** (shells out) vs
   **stub** (deterministic encoder; the current byte-stub path, kept for CI). Never linked into the
   app binary (I2). Records tool + model + voice + sample-rate/bitrate provenance.
2. **Opus encoding.** Emit 24 kbps mono Opus (the NFR-4 budget basis). Encoder is part of the render
   stage; stub path emits deterministic placeholder bytes of a *representative size* (see Part B — the
   size test must exercise realistic weights even in the stub, or the budget test is theater).
3. **Real word-level `alignment` rows.** The render returns forced-aligner word timings that **replace**
   `internal/align`'s deterministic character-rate model output, under the identical
   `pack.AlignToken` contract (strictly increasing, non-overlapping, contiguous-within-sentence,
   sentence-gap pauses). `internal/align` stays the CI/stub timing source; the real path is behind the
   flag. No pack schema change (SchemaVersion stays 1).
4. **Wire into pipeline + workq.** The render stage runs after gate-pass, before pack build; resumable
   (MC-3 `internal/workq`) so a killed render resumes. Per-story audio bytes + timings flow into
   `pack.Story.AudioData` / `alignment`.

## Part B — NFR-3/NFR-4 size budgets, measured + build-failing (`internal/pack`, factory test)

5. **Measure real weights.** Build a representative A1+A2 pack with **real Opus** (24 kbps mono) and
   **real HEIC** covers; record actual bytes per story-minute of audio and per cover image. Recheck the
   CP-08a image-weight estimate against reality; note deltas in `plans/cp-09c-done.md`.
6. **Size-budget model + assertion.** Add `pack.MeasureBudget(p) → Budget{DownloadBytes, OnDiskBytes}`
   computing the NFR-3 (app + embedded starter ≤90 MB **download**) and NFR-4 (A1+A2 with audio
   ≤250 MB **on disk**) figures from the actual file set (audio + images + sqlite + manifest).
7. **Build-failing test.** A Go test constructs an over-budget pack (oversized synthetic audio/images)
   and asserts the build **fails** with a budget breach; and asserts a realistic A1+A2 pack is within
   budget. This is the acceptance gate — a pack that breaches NFR-3/4 cannot be built green.

## Part C — Alignment re-verification against real audio (Swift `KaraokeDriftTests`)

8. **Sampled real render → drift re-verify.** Take one ~3-minute story rendered by the real CosyVoice
   path, ship its real Opus + real alignment rows into a test fixture, and re-run the CP-06
   `KaraokeDriftTests` (`ios/Tests/ZhuwenAudioTests/KaraokeDriftTests.swift`) — highlight drift must
   stay **<120 ms** across the story at every supported speed with the *real* timings (not the
   synthesized track). Wire as a pre-push check (like CP-06), keep the synthetic-track test in CI.

## Part D — B-4 at scale: real Commons curation toward ~570 images (owner-in-the-loop)

The schedule long pole. `factory/cmd/imagespike` + `internal/images` (fetch/gate/curate/join) already
implement the batch workflow (self-contained HTML review sheet → `*-image-decisions.json` →
`images.DecisionsToImages`). CP-09c operationalizes it at scale; the owner is the curator.

9. **Inventory + batching plan.** Enumerate the full §8A image demand for the 81 canon entries + the
   A1–A2 authored spine + B1 packs (~570 images). Group into owner-reviewable batches (by canon tier /
   band / topic) so each `imagespike` run + review sheet is a bounded sitting. Track batch status in
   `plans/cp-09c-image-inventory.md` (word/story → assigned/curated/signed-off).
10. **Per-batch curation loop.** For each batch: `imagespike` regenerates candidates (Commons,
    anonymous GET, no secret — I2 CI never takes this path); owner reviews the HTML sheet, overrides
    picks, exports decisions; `zhuwenctl images curate` locks per-word/per-story image + provenance
    into the pack via `DecisionsToImages`. **Per-image license sign-off on the Commons page**
    (CC-BY/SA attribution is a legal obligation, FR-11.2) is recorded, not assumed (I4).
11. **Ship-readiness gate.** A canon entry / pack graduates from fixture stand-in to shippable only
    when every story's cover has a signed-off, gate-passing image. `pack.validateI6` already blocks
    imageless/AI/unprovenanced covers; the inventory doc is the human tracker of what has cleared.

## Tasks
- A: `internal/tts` external CosyVoice stage (real + deterministic stub); Opus 24 kbps encoder; real
  forced-aligner word timings replacing `internal/align` output behind the flag; wire into
  pipeline/workq (resumable). No app change; no schema change.
- B: `pack.MeasureBudget` (NFR-3/NFR-4 from real file set); build-failing size-budget Go test;
  image-weight recheck note.
- C: real-render sample fixture; re-run Swift `KaraokeDriftTests` <120 ms with real audio (pre-push).
- D: `plans/cp-09c-image-inventory.md` batching plan; run owner curation batches through
  imagespike → decisions → `images curate`; per-image license sign-off; graduate packs off stubs.
- Docs: root README + `factory/data/README.md` (CosyVoice run instructions, voice/model provenance,
  size figures) + `ios/README.md`; `plans/cp-09c-done.md`.
- Blockers: resolve/advance **B-5** (CosyVoice tooling + voice licensing) and **B-4** (Commons
  curation) in `plans/blockers.md`; fixture stand-ins remain until each batch clears.

## Tests (unit / integration / e2e)
- **TTS stub path (Go, hermetic):** deterministic — identical input yields identical Opus bytes +
  timings; CI never shells out / hits network / needs a venv (I2). `make audit` green.
- **Alignment contract (Go):** real-path timings (fed from a captured sample) satisfy the
  `pack.AlignToken` invariants (strictly increasing, non-overlapping, contiguous-in-sentence).
- **Size budget (Go):** over-budget pack fails the build (NFR-3 **and** NFR-4 breach cases); realistic
  A1+A2 pack passes; assertion runs in `make ci`.
- **Karaoke drift (Swift, pre-push):** real-audio sample stays <120 ms at every supported speed; the
  synthetic 3-min drift test stays in CI.
- **B-4 curation (Go, hermetic):** `DecisionsToImages` + `validateI6` reject a story whose cover lacks
  a signed-off/provenanced/gate-passing image; a fixture decisions file drives CI (no Commons network).
- **CI gate:** `make ci` (factory) + `swift test` green; `make audit` green (no new app network
  surface); `make build` → any new subcommands runnable from `bin/` (EC-2).

## Dependencies (handoff §0.4) — confirm with owner before wiring
- **CosyVoice 3.0 local tooling** — Python venv on Apple Silicon (MPS), run **build-time only**, $0;
  voice-model license recorded before wiring (B-5). CI uses the deterministic stub encoder.
- **No hosted TTS / no Qwen API needed for 09c** (the CP-09 Qwen A/B is separate; local CosyVoice is
  the render path). No new **shipped app** dependency (audio is pregenerated, I3).
- **B-4 curation is owner-in-the-loop** — the ~570-image sign-off is human judgment (semantic/cultural
  fit + per-image license); agents build/run the tooling and stage decisions, owner approves.

## Blockers filed / carried
- **B-4 — real Commons image curation at ~570 scale** — ⏳ owner-in-the-loop; the schedule long pole.
  81 canon + A1–B1 covers can't ship on fixture stand-ins. Advanced batch-by-batch in Part D.
- **B-5 — CosyVoice tooling + voice licensing** — record render tool + voice-model license before the
  real stage ships; stub path keeps CI/dev unblocked meanwhile.

## Follow-ons (→ CP-10)
- Multi-voice CosyVoice; automated (non-sampled) audit; broader B2 content. Ship polish, App Store
  submission prep, and full-device QA move to `plans/cp-10-plan.md`.
