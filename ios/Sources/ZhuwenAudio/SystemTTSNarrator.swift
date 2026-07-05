#if canImport(AVFoundation)
import AVFoundation
import Foundation

/// Layer 3 (handoff §7, FR-5.4): on-device `AVSpeechSynthesizer` narration, used only when a
/// story has no downloaded pack audio. The UI labels it "System voice" (`isSystemVoice`).
/// Word highlight comes from the synthesizer's `willSpeakRangeOfSpeechString` delegate mapped
/// through `CharTokenMap` — the app still computes no timing (Apple drives the ranges).
public final class SystemTTSNarrator: NSObject, AudioNarrator, AVSpeechSynthesizerDelegate {
    private let synth = AVSpeechSynthesizer()
    private let text: String
    private let voice: AVSpeechSynthesisVoice?
    private let map: CharTokenMap

    private var startedAt: Date?
    private var accumulatedMillis: Int = 0
    private var speaking = false
    private var currentRate: Double = PlaybackSpeed.normal.value

    /// Called with the token index the synthesizer is currently speaking (or nil).
    public var onHighlight: ((Int?) -> Void)?

    public init(tokenTexts: [String], voice: AVSpeechSynthesisVoice? = SystemVoice.best()) {
        self.map = CharTokenMap(tokenTexts: tokenTexts)
        self.text = map.fullText
        self.voice = voice
        super.init()
        synth.delegate = self
    }

    // AVSpeechSynthesizer offers no reliable clock; approximate elapsed time for the model's
    // progress bar. The *highlight* comes from the delegate ranges, not this estimate.
    public var currentMillis: Int {
        guard let startedAt, speaking else { return accumulatedMillis }
        return accumulatedMillis + Int(Date().timeIntervalSince(startedAt) * 1000)
    }

    public var durationMillis: Int { 0 } // unknown for TTS
    public var isPlaying: Bool { synth.isSpeaking && !synth.isPaused }
    public let isSystemVoice = true

    public var rate: Double {
        get { currentRate }
        set { currentRate = PlaybackSpeed(newValue).value }
    }

    public var onFinish: (() -> Void)?

    public func play() {
        if synth.isPaused {
            synth.continueSpeaking()
            startedAt = Date()
            return
        }
        guard !synth.isSpeaking else { return }
        let utterance = AVSpeechUtterance(string: text)
        utterance.voice = voice ?? AVSpeechSynthesisVoice(language: "zh-CN")
        // Map 0.6–1.2 onto AVSpeechUtterance's rate around the default.
        utterance.rate = Self.avRate(for: currentRate)
        accumulatedMillis = 0
        startedAt = Date()
        speaking = true
        synth.speak(utterance)
    }

    public func pause() {
        synth.pauseSpeaking(at: .word)
        if let startedAt { accumulatedMillis += Int(Date().timeIntervalSince(startedAt) * 1000) }
        startedAt = nil
    }

    public func seek(toMillis ms: Int) {
        // AVSpeechSynthesizer is not seekable; ignore (pack audio is the seekable path).
    }

    private static func avRate(for speed: Double) -> Float {
        // Default rate is AVSpeechUtteranceDefaultSpeechRate; scale proportionally, clamped.
        let d = Double(AVSpeechUtteranceDefaultSpeechRate)
        let lo = Double(AVSpeechUtteranceMinimumSpeechRate)
        let hi = Double(AVSpeechUtteranceMaximumSpeechRate)
        return Float(Swift.min(hi, Swift.max(lo, d * speed)))
    }

    // MARK: - AVSpeechSynthesizerDelegate

    public func speechSynthesizer(_ synthesizer: AVSpeechSynthesizer,
                                  willSpeakRangeOfSpeechString characterRange: NSRange,
                                  utterance: AVSpeechUtterance) {
        onHighlight?(map.tokenIndex(forRangeLocation: characterRange.location))
    }

    public func speechSynthesizer(_ synthesizer: AVSpeechSynthesizer, didFinish utterance: AVSpeechUtterance) {
        speaking = false
        onHighlight?(nil)
        onFinish?()
    }
}

/// Enhanced zh-CN voice detection (§7): if the user has manually installed an enhanced/premium
/// Mandarin voice, prefer it; otherwise fall back to the default compact zh-CN voice.
public enum SystemVoice {
    public static func best() -> AVSpeechSynthesisVoice? {
        let zh = AVSpeechSynthesisVoice.speechVoices().filter { $0.language.hasPrefix("zh-CN") || $0.language.hasPrefix("zh_CN") }
        if let enhanced = zh.first(where: { $0.quality == .premium })
            ?? zh.first(where: { $0.quality == .enhanced }) {
            return enhanced
        }
        return zh.first ?? AVSpeechSynthesisVoice(language: "zh-CN")
    }

    /// True when a non-compact (enhanced/premium) zh-CN voice is available (§7: prompt the pack
    /// download otherwise).
    public static func hasEnhancedVoice() -> Bool {
        AVSpeechSynthesisVoice.speechVoices().contains {
            ($0.language.hasPrefix("zh-CN") || $0.language.hasPrefix("zh_CN")) && $0.quality != .default
        }
    }
}
#endif
