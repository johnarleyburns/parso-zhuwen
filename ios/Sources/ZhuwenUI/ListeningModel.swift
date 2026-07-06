import Foundation
import SwiftUI
import ZhuwenAudio
import ZhuwenCore
import ZhuwenPacks

#if canImport(AVFoundation)
import AVFoundation
#endif

/// Drives the M7 listening screen: it owns an `AudioNarrator` (pack audio, or the labeled
/// system-TTS fallback) and the pure `Karaoke` engine, samples the narrator clock on a timer,
/// and republishes the highlighted token, transport state, speed, and blind-mode state for
/// SwiftUI (handoff §5, FR-5.1/5.2/5.4). All timing/decisions live in `ZhuwenAudio`.
@MainActor
public final class ListeningModel: ObservableObject {
    public let tokens: [DisplayToken]
    private let karaoke: Karaoke
    private let audioURL: URL?
    private let storyID: String
    private let onEvent: (Event) -> Void

    @Published public private(set) var highlightedIndex: Int?
    @Published public private(set) var isPlaying = false
    @Published public private(set) var speed: PlaybackSpeed
    @Published public private(set) var blindMode: Bool
    @Published public private(set) var revealed: Bool
    @Published public private(set) var usingSystemVoice = false
    @Published public private(set) var progress: Double = 0

    private var narrator: AudioNarrator?
    private var timer: Timer?

    public init(story: StoryRecord, tokens: [DisplayToken], alignment: [AlignmentToken],
                audioURL: URL?, blind: Bool = false,
                onEvent: @escaping (Event) -> Void = { _ in }) {
        self.tokens = tokens
        self.storyID = story.id
        self.audioURL = audioURL
        self.karaoke = Karaoke(track: AlignmentTrack(alignment), blindMode: blind)
        self.speed = .normal
        self.blindMode = blind
        self.revealed = !blind
        self.onEvent = onEvent
    }

    /// Whether the reader text should be hidden right now (blind mode, pre-reveal, FR-5.2).
    public var textHidden: Bool { karaoke.textHidden }

    public var hasAlignment: Bool { !karaoke.track.isEmpty }

    // MARK: - Transport

    public func playPause() {
        ensureNarrator()
        guard let narrator else { return }
        if narrator.isPlaying {
            narrator.pause()
            isPlaying = false
            stopTimer()
        } else {
            narrator.play()
            isPlaying = true
            startTimer()
        }
    }

    public func setSpeed(_ speed: PlaybackSpeed) {
        self.speed = speed
        karaoke.setSpeed(speed)
        narrator?.rate = speed.value
    }

    public func toggleBlind() {
        let on = !blindMode
        blindMode = on
        karaoke.setBlind(on)
        revealed = karaoke.textHidden ? false : true
    }

    /// Reveal the text in blind mode; logs the blind-listening pass (FR-5.2).
    public func reveal() {
        karaoke.reveal()
        revealed = true
        onEvent(.listen(storyID: storyID, blind: true, at: Date()))
    }

    /// Tap a word to seek to it (FR-5.1). No-op on the non-seekable TTS fallback.
    public func seek(toToken index: Int) {
        ensureNarrator()
        let ms = karaoke.seekMillis(toTokenAt: index)
        narrator?.seek(toMillis: ms)
        refreshHighlight()
    }

    public func stop() {
        narrator?.pause()
        isPlaying = false
        stopTimer()
    }

    // MARK: - Narrator wiring

    private func ensureNarrator() {
        guard narrator == nil else { return }
        #if canImport(AVFoundation)
        // Layer 1: pack audio if present and decodable; else layer 3: labeled system TTS.
        if let url = audioURL, let pack = try? PackAudioNarrator(url: url) {
            pack.rate = speed.value
            pack.onFinish = { [weak self] in Task { @MainActor in self?.handleFinish() } }
            narrator = pack
            usingSystemVoice = false
        } else {
            let tts = SystemTTSNarrator(tokenTexts: tokens.map(\.text))
            tts.rate = speed.value
            tts.onHighlight = { [weak self] idx in Task { @MainActor in self?.highlightedIndex = idx } }
            tts.onFinish = { [weak self] in Task { @MainActor in self?.handleFinish() } }
            narrator = tts
            usingSystemVoice = true
        }
        #endif
    }

    private func handleFinish() {
        isPlaying = false
        stopTimer()
        progress = 1
        // Completing a blind pass reveals the text (FR-5.2) and logs the listen.
        if blindMode && !revealed { reveal() }
        else { onEvent(.listen(storyID: storyID, blind: false, at: Date())) }
    }

    // MARK: - Highlight sampling

    private func startTimer() {
        stopTimer()
        // ~60 Hz sample of the narrator clock (drift << the 120 ms budget; see KaraokeDriftTests).
        let t = Timer(timeInterval: 1.0 / 60.0, repeats: true) { [weak self] _ in
            guard let self else { return }
            Task { @MainActor in self.refreshHighlight() }
        }
        RunLoop.main.add(t, forMode: .common)
        timer = t
    }

    private func stopTimer() {
        timer?.invalidate()
        timer = nil
    }

    private func refreshHighlight() {
        guard let narrator else { return }
        // Pack audio drives highlight from the alignment track; TTS drives it via delegate.
        if !usingSystemVoice, hasAlignment {
            highlightedIndex = karaoke.highlightedToken(atMillis: narrator.currentMillis)
        }
        if narrator.durationMillis > 0 {
            progress = min(1, Double(narrator.currentMillis) / Double(narrator.durationMillis))
        }
    }
}
