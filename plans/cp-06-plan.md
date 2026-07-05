# CP-06 — Listening (M7): pack-audio karaoke via alignment, speeds, blind mode, TTS fallback

**Refs:** handoff §5 (`ZhuwenAudio`: "pack playback (word-synced from `alignment`),
AVSpeechSynthesizer layers 2–3 with enhanced-voice detection (§7)"), §6 (CP-06 acceptance:
*Highlight drift <120 ms across a 3-min story on device*), §3 (`alignment(story_id,
token_idx, t0_ms, t1_ms)` in the frozen pack schema), §4.7 (TTS + forced alignment stage).
`00` §4 FR-5.1–5.4 (karaoke / blind / background / fallback), §7 (three-layer audio
strategy + enhanced-voice detection), §9 (`StoryProgress.listenedBlind`). Depends on CP-02
(pack schema: `alignment` table + `story.audio_file` already frozen), CP-03 (`ReaderModel`
display tokens), CP-04 (`EventLog`, I5).

**Acceptance (handoff §6):** pack-audio karaoke via alignment; playback speeds; blind mode;
system-TTS fallback labeled. ✅ **Highlight drift <120 ms across a 3-min story.** Plus the
standing exit criteria (`plans/exit-criteria.md`).

## Invariants in play
- **I3 pregenerated only.** Audio + word-level timestamps are produced *offline in the
  factory* and shipped in the pack; the app renders no audio and computes no alignment. The
  system-TTS fallback (FR-5.4) is Apple's on-device `AVSpeechSynthesizer` (no generation
  model), used only when a story has no pack audio, and is labeled "System voice".
- **I5 every tap teaches / append-only.** A completed listen (esp. blind, FR-5.2) is recorded
  as an append-only `Event` (`.listen`, with a `blind` flag). Listening-skill separation from
  reading is a CP-07 concern, so the current reading-oriented `KnownWordModel` projection
  deliberately **ignores** `.listen` (reading P(known) is untouched); the event is retained in
  the log for CP-07's both-skill estimate. Replay determinism preserved.
- **I2 no accounts/network.** All audio is local (pack bytes or on-device TTS). No new
  network surface; `ZhuwenPacks` remains the only `URLSession` owner.
- **No new dependencies** (NFR-5): AVFoundation is a system framework; the drift-critical
  resolver is pure Swift stdlib and host-testable.

## Design decisions
1. **Factory carries alignment + audio (I3).** A new deterministic `internal/align` stage
   assigns every body token a `[t0_ms, t1_ms)` from its character count at a nominal narration
   rate, with a pause at sentence boundaries. `pack.Story` gains `AudioFile` + `Alignment`;
   the builder writes the `alignment` table, `story.alignment` (JSON), and `story.audio_file`,
   and ships a per-story audio entry (`audio/<id>.opus`) hashed into the signed manifest.
   Fixture audio bytes are stubs (`fixture:true` already at the story level) — **real
   CosyVoice render is CP-09**; CP-06 freezes the *format and the timestamp contract*. The
   aligner guarantees: contiguous non-overlapping intervals within a sentence, strictly
   increasing, one row per body token, total = audio duration.
2. **Drift is a pure, testable resolver.** `ZhuwenAudio.AlignmentTrack.index(atMillis:)` is a
   binary search over the intervals → the highlighted token. On device the highlight lag is
   bounded by (a) resolver correctness (exact: the returned interval contains the query, or is
   the nearest active token in an inter-sentence gap) and (b) the UI sample cadence Δt. The
   acceptance test simulates a **180 s** track played through the resolver at a display-tick
   cadence across all speeds and asserts **max drift < 120 ms** — the honest, automatable core
   of "drift <120 ms over a 3-min story on device".
3. **Engine (pure) + backends (AVFoundation) + thin UI**, matching the CP-03/04/05 split.
   `ZhuwenAudio`:
   - `AlignmentTrack` (value type): intervals, `duration`, `index(atMillis:)`, `startMillis(of:)`.
   - `Karaoke` (pure engine): current highlighted token from a clock in ms, `seekMillis(toToken:)`,
     `PlaybackSpeed` (clamped 0.6×–1.2×), blind-mode reveal state.
   - `AudioNarrator` protocol (a play/pause/seek/rate + `currentMillis` clock) with two
     AVFoundation-guarded backends: `PackAudioNarrator` (AVAudioPlayer, `enableRate` time-stretch
     — FR-5.1 pitch-preserved) and `SystemTTSNarrator` (`AVSpeechSynthesizer` zh-CN with
     `willSpeakRangeOfSpeechString` → token highlight, FR-5.4). `SystemVoice.best()` prefers a
     user-installed enhanced zh-CN voice (§7).
   `ZhuwenUI.ListeningModel` (@MainActor) drives the engine from a timer + narrator and publishes
   `highlightedIndex / isPlaying / speed / blind / revealed`; `ListeningView` is M7.
4. **Blind mode (FR-5.2).** Text hidden until the learner reveals (or playback ends); reveal
   logs a `.listen(blind:)` event per story. Karaoke and blind share one engine.

## Tasks
1. **Factory `internal/align`** — `Align(tokens, cfg) → []pack.AlignToken, totalMs`; deterministic;
   sentence-gap handling. Unit tests (monotonic, non-overlapping, one-per-token, determinism, drift-bound).
2. **Factory `pack`** — `Story.AudioFile/Alignment`; insert `alignment` rows + `story.alignment`/`audio_file`;
   ship stub audio bytes (`audio/<id>.opus`) in `Build`; hash into manifest. Schema unchanged (already frozen).
   Update `pack_test` / `schema_test` and the pipeline to populate alignment. Regenerate `ios/Fixtures`.
3. **iOS `ZhuwenPacks`** — `StoryRecord.audioFile`; `AlignmentToken`; `ContentDatabase.alignment(storyID:)`;
   `PackStore.audioURL(for:)` (extract the pack audio entry to a temp file). Tests vs the regenerated fixture.
4. **iOS `ZhuwenAudio`** (new SPM target) — `AlignmentTrack`, `Karaoke`, `PlaybackSpeed`, `AudioNarrator` +
   `PackAudioNarrator`/`SystemTTSNarrator`/`SystemVoice` (AVFoundation-guarded). Unit + **drift acceptance** tests.
5. **iOS `ZhuwenCore`** — `EventKind.listen` + `Event.blind` + `Event.listen(...)` builder + `case .listen: break`
   (documented separation). Test: a listen event doesn't move reading P(known) and survives replay.
6. **iOS `ZhuwenUI`** — `ListeningModel` + `ListeningView` (M7: karaoke highlight, play/pause, speed 0.6–1.2,
   blind toggle, tap-word-to-seek, "System voice" label); "Listen" entry from the reader. `make build-ios`.
7. **Docs** — root + `ios/` READMEs (CP-06 ✅, new `ZhuwenAudio` package + commands); `plans/cp-06-done.md`.

## Tests (unit / integration / e2e)
- **Factory unit:** `align_test.go` — determinism, strictly-increasing non-overlapping intervals, one row
  per body token, per-sentence contiguity + gap, total duration; `pack_test` — alignment/audio round-trip;
  `schema_test` unchanged (drift guard).
- **iOS unit:** `AlignmentTrackTests` (binary-search resolution, boundaries, gaps, out-of-range);
  `KaraokeTests` (speed clamp/scaling, seek-to-token, blind reveal); `ContentDatabaseTests`/`PackStoreTests`
  (alignment rows + audio entry present, hashes verify).
- **iOS integration:** load the fixture pack → build an `AlignmentTrack` per story → resolver invariants hold.
- **iOS e2e / acceptance:** `KaraokeDriftTests.testHighlightDriftUnder120msOver3MinStory` — synthesize a 180 s
  alignment track, simulate playback at a display tick across 0.6×/1.0×/1.2×, assert max highlight drift
  < 120 ms; plus determinism.

## No new dependencies
AVFoundation is a first-party Apple framework (already permitted, handoff §5/§7). The pure
resolver/engine and all math use only the Swift stdlib. No Go module additions.

## Follow-ons (not CP-06)
Real CosyVoice 3.0 render + forced alignment replacing stub audio/timestamps (CP-09);
listening-skill CEFR estimate folding `.listen` events (CP-07); background audio session +
`MPNowPlayingInfoCenter` lock-screen controls wired to the `@main` app target (FR-5.3, assembled
in Xcode); `StoryProgress.listenedBlind` SwiftData persistence.
