#if canImport(AVFoundation)
import AVFoundation
import Foundation

/// Layer 1 (handoff §7): plays the factory-rendered pack audio (Opus), pitch-preserved time
/// stretch for 0.6×–1.2× (FR-5.1). The karaoke highlight is driven by the paired
/// `AlignmentTrack` reading this narrator's clock — no timing computed on device (I3).
public final class PackAudioNarrator: NSObject, AudioNarrator, AVAudioPlayerDelegate {
    private let player: AVAudioPlayer

    public init(url: URL) throws {
        self.player = try AVAudioPlayer(contentsOf: url)
        super.init()
        player.enableRate = true          // pitch-preserved rate (FR-5.1)
        player.rate = Float(PlaybackSpeed.normal.value)
        player.prepareToPlay()
        player.delegate = self
    }

    public var currentMillis: Int { Int(player.currentTime * 1000) }
    public var durationMillis: Int { Int(player.duration * 1000) }
    public var isPlaying: Bool { player.isPlaying }
    public let isSystemVoice = false

    public var rate: Double {
        get { Double(player.rate) }
        set { player.rate = Float(PlaybackSpeed(newValue).value) }
    }

    public var onFinish: (() -> Void)?

    public func play() { player.play() }
    public func pause() { player.pause() }

    public func seek(toMillis ms: Int) {
        let clamped = Swift.min(Swift.max(0, ms), durationMillis)
        player.currentTime = Double(clamped) / 1000
    }

    public func audioPlayerDidFinishPlaying(_ player: AVAudioPlayer, successfully flag: Bool) {
        onFinish?()
    }
}
#endif
