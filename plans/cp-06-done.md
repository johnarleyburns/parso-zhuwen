# CP-06 — Done note (Listening M7: pack-audio karaoke via alignment, speeds, blind, TTS fallback)

**Status:** complete. iOS `swift test` green (**90 Swift tests**, +29); `swift build` compiles
`ZhuwenAudio` + `ZhuwenUI`; `make build-ios` **BUILD SUCCEEDED** (the listening screen +
AVFoundation audio backends cross-compile for the iOS 17 simulator); `make bench` green (NFR-2
unaffected, ~2.2 ms release). Factory `make ci` green (**66 Go tests**, +8 new in `internal/align`
+ pack round-trip).

## Handoff §6 acceptance
- **✅ Highlight drift < 120 ms across a 3-min story.** `KaraokeDriftTests` synthesizes a 180 s
  alignment track (>400 tokens, the factory aligner's shape) and plays it through the pure
  `AlignmentTrack` resolver at a UI refresh cadence across **0.6× / 0.8× / 1.0× / 1.2×**,
  measuring at 1 ms resolution the longest span the displayed token differs from the truly
  active token (= the highlight lag). It asserts the drift is **> 0** (the sim genuinely
  exercises refresh-cadence staleness) **and < 120 ms** at every speed, at a conservative 20 Hz
  UI tick; a 60 Hz CADisplayLink cadence has < 40 ms margin. The resolver itself is exact
  (`AlignmentTrackTests`), so on-device drift is bounded by this cadence.
- **Pack-audio karaoke via alignment (FR-5.1).** A new factory `internal/align` stage assigns
  every body token a `[t0,t1)` window (character-rate model; contiguous within a sentence, a
  pause between sentences); the builder writes the `alignment` table + `story.alignment` +
  `story.audio_file` and ships a per-story audio entry hashed into the signed manifest.
  `AlignmentTrack.index(atMillis:)` (binary search) drives the on-device highlight; tap a word
  to seek via `startMillis(ofTokenAt:)`.
- **Speeds 0.6×–1.2× (FR-5.1).** `PlaybackSpeed` clamps to range; `PackAudioNarrator` uses
  `AVAudioPlayer.enableRate` (pitch-preserved).
- **Blind mode (FR-5.2).** `Karaoke` hides text until reveal; completing/refresh reveals and
  logs the pass; `ListeningView` shows the "Listen first" placeholder.
- **System-TTS fallback, labeled (FR-5.4, §7).** `SystemTTSNarrator` (`AVSpeechSynthesizer`
  zh-CN) is used when a story has no decodable pack audio; highlight comes from
  `willSpeakRangeOfSpeechString` mapped through the testable `CharTokenMap`; the UI shows a
  "System voice" banner; `SystemVoice.best()` prefers a user-installed enhanced/premium voice.

## Invariants preserved
- **I3 pregenerated only.** Audio + timings are produced offline in the factory and shipped in
  the pack; the app computes no timing. The fallback is Apple's on-device synthesizer (no
  generation model). Fixture audio bytes are stubs (`fixture:true`); CosyVoice render + real
  forced alignment is CP-09 behind the same timestamp contract.
- **I5 append-only / replay.** A completed listen is an append-only `Event(.listen, blind:)`;
  the reading-oriented `KnownWordModel` projection deliberately leaves it untouched (listening
  skill is separated, folded in CP-07). `testListenEventDoesNotAlterReadingPKnown` +
  `testListenEventReplaysDeterministicallyInLog` (Codable round-trip of the `blind` payload).
- **I2 no network / I6 unchanged.** All audio is local (pack bytes or on-device TTS); no new
  URLSession. The schema was already frozen at CP-02 (the `alignment` table + `audio_file`
  columns existed) — **no schema change**, so the freeze guard (`schema_test`) still passes.

## Deliverables
- **Factory:** `internal/align/align.go` (deterministic aligner + `DefaultConfig`);
  `pack.Story.{AudioFile,AudioData,Alignment}` + `pack.AlignToken`; builder ships audio +
  writes the `alignment` table / `story.alignment` / `story.audio_file`; pipeline runs the
  aligner. Regenerated `ios/Fixtures/*` (audio + alignment; content.sqlite 114 KB → 303 KB).
- **ZhuwenPacks:** `StoryRecord.audioFile`, `AlignmentToken`, `ContentDatabase.alignment(storyID:)`,
  `PackStore.{audioData,audioURL}(for:)` (lazy extraction + cleanup).
- **ZhuwenAudio (new SPM target):** `AlignmentTrack`, `Karaoke`, `PlaybackSpeed`, `CharTokenMap`,
  `AudioNarrator` + `PackAudioNarrator` / `SystemTTSNarrator` / `SystemVoice` (AVFoundation-guarded).
- **ZhuwenCore:** `EventKind.listen` + `Event.blind` + `Event.listen(...)`; `case .listen` no-op
  (documented skill separation).
- **ZhuwenUI:** `ListeningModel` (@MainActor; timer-sampled narrator clock → highlight) +
  `ListeningView` (M7); "Listen" (headphones) entry in `ReaderView` from Today/Library.

## Tests (unit / integration / e2e)
- **Factory unit:** `align_test.go` — one row per token, strictly increasing / non-overlapping,
  per-sentence contiguity + gap, per-char duration + floor, determinism, empty; `pack_test`
  `TestAudioAndAlignmentRoundTrip` (audio hashed in manifest + `alignment` table + `story.alignment`).
- **iOS unit:** `AlignmentTrackTests` (lead-in, active, gap-holds-previous, after-end, seek,
  exhaustive scan), `KaraokeTests` (speed clamp/label, seek, blind reveal/re-hide),
  `CharTokenMapTests` (range→token), `AudioPackTests` (every story has audio + one align row
  per body token, ordered/non-overlapping, extractable URL).
- **iOS integration:** `AlignmentTrackTests.testBuildsFromVendoredPackStory` (fixture pack →
  track invariants); `KnownWordModelTests` listen-event separation + replay.
- **iOS e2e / acceptance:** `KaraokeDriftTests` — the 3-min < 120 ms drift acceptance (+ margin
  + determinism).

## Exit criteria (standing)
- [x] Handoff §6 acceptance for CP-06 met (karaoke via alignment, speeds, blind, labeled TTS
      fallback; highlight drift < 120 ms over a 3-min story).
- [x] EC-1 READMEs updated (root + `ios/`) with status + commands.
- [x] EC-2 `make build` → `bin/` unchanged; `make ci` still green (aligner is a library stage).
- [x] EC-3 `swift test` green (unit + integration + e2e/acceptance); `make build-ios` succeeds.

## No new dependencies
AVFoundation is a first-party Apple framework (handoff §5/§7); the resolver/engine and all math
use only the Swift stdlib. No Go module additions; hand-written binary search / char-rate model.

## Follow-ons
Real CosyVoice 3.0 render + forced alignment replacing stub audio/timings (CP-09); listening-skill
CEFR estimate folding `.listen` events (CP-07); background `AVAudioSession` + `MPNowPlayingInfoCenter`
lock-screen controls (FR-5.3) and `StoryProgress.listenedBlind` SwiftData persistence — assembled
with the `@main` app target. Next up: **CP-07 (loop completion — comprehension→seal, FSRS review,
progress with both-skill estimates).**
