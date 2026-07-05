import Foundation

/// Playback speed for karaoke listening (FR-5.1: 0.6×–1.2×, pitch-preserved by the player).
/// Always clamped to the supported range so UI controls can't drive the player out of bounds.
public struct PlaybackSpeed: Equatable {
    public static let minimum = 0.6
    public static let maximum = 1.2
    /// The presets surfaced in the M7 speed control.
    public static let presets: [PlaybackSpeed] = [0.6, 0.8, 1.0, 1.2].map(PlaybackSpeed.init)

    public let value: Double

    public init(_ value: Double) {
        self.value = Swift.min(Self.maximum, Swift.max(Self.minimum, value))
    }

    public static let normal = PlaybackSpeed(1.0)

    /// A short label like "1.0×" for the control.
    public var label: String {
        String(format: "%.1f×", value)
    }
}

/// The pure karaoke engine: it owns an `AlignmentTrack` and the listening *modes* (speed,
/// blind reveal, FR-5.1/5.2) and resolves the highlighted token for a playback position. It
/// holds no timers, players, or UI — those live in `ZhuwenUI.ListeningModel`, which drives
/// this from an `AudioNarrator` clock. Keeping it pure makes the drift guarantee testable.
public final class Karaoke {
    public let track: AlignmentTrack
    public private(set) var speed: PlaybackSpeed
    public private(set) var blindMode: Bool
    public private(set) var revealed: Bool

    public init(track: AlignmentTrack, speed: PlaybackSpeed = .normal, blindMode: Bool = false) {
        self.track = track
        self.speed = speed
        self.blindMode = blindMode
        self.revealed = false
    }

    /// The token to highlight at playback position `ms` (nil during lead-in silence).
    public func highlightedToken(atMillis ms: Int) -> Int? {
        track.index(atMillis: ms)
    }

    /// The seek position (ms) for tapping the token at body position `tokenIndex` (FR-5.1).
    /// Falls back to 0 for tokens without a timing (e.g. skipped punctuation, defensive).
    public func seekMillis(toTokenAt tokenIndex: Int) -> Int {
        track.startMillis(ofTokenAt: tokenIndex) ?? 0
    }

    @discardableResult
    public func setSpeed(_ speed: PlaybackSpeed) -> PlaybackSpeed {
        self.speed = speed
        return speed
    }

    /// Toggle blind-listening (FR-5.2). Entering blind mode re-hides the text.
    public func setBlind(_ on: Bool) {
        blindMode = on
        if on { revealed = false }
    }

    /// Reveal the text in blind mode (after listening, or on demand).
    public func reveal() { revealed = true }

    /// Whether the reader text should be hidden right now (blind + not yet revealed, FR-5.2).
    public var textHidden: Bool { blindMode && !revealed }
}
