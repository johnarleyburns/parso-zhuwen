import Foundation

/// A narration source the listening UI can drive: a clock (`currentMillis`) plus transport
/// controls. Two backends implement it (handoff §7 three-layer audio): `PackAudioNarrator`
/// plays the factory-rendered pack audio synced to the alignment track (FR-5.1); the labeled
/// `SystemTTSNarrator` fallback uses on-device `AVSpeechSynthesizer` (FR-5.4). Keeping the
/// protocol Foundation-only lets the karaoke engine and models be reasoned about without
/// AVFoundation.
public protocol AudioNarrator: AnyObject {
    /// Current playback position in milliseconds.
    var currentMillis: Int { get }
    /// Total duration in milliseconds (0 if unknown, e.g. before TTS starts).
    var durationMillis: Int { get }
    var isPlaying: Bool { get }

    /// Playback rate (time-stretch, pitch-preserved). Setter clamps to `PlaybackSpeed`.
    var rate: Double { get set }

    /// True for the system-TTS fallback so the UI can label it "System voice" (FR-5.4, §7).
    var isSystemVoice: Bool { get }

    func play()
    func pause()
    /// Seek to an absolute position (ms), clamped to `[0, durationMillis]`.
    func seek(toMillis ms: Int)

    /// Invoked when playback reaches the end.
    var onFinish: (() -> Void)? { get set }
}

public extension AudioNarrator {
    var isFinished: Bool { durationMillis > 0 && currentMillis >= durationMillis }
}
