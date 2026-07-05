import XCTest
import ZhuwenPacks
@testable import ZhuwenAudio

final class KaraokeTests: XCTestCase {
    private func karaoke() -> Karaoke {
        Karaoke(track: AlignmentTrack([
            AlignmentToken(tokenIdx: 0, t0ms: 0, t1ms: 300),
            AlignmentToken(tokenIdx: 1, t0ms: 300, t1ms: 600),
            AlignmentToken(tokenIdx: 2, t0ms: 600, t1ms: 900),
        ]))
    }

    func testSpeedClampsToSupportedRange() {
        XCTAssertEqual(PlaybackSpeed(0.1).value, PlaybackSpeed.minimum)
        XCTAssertEqual(PlaybackSpeed(9.0).value, PlaybackSpeed.maximum)
        XCTAssertEqual(PlaybackSpeed(0.9).value, 0.9, accuracy: 1e-9)
    }

    func testSpeedLabel() {
        XCTAssertEqual(PlaybackSpeed(1.0).label, "1.0×")
        XCTAssertEqual(PlaybackSpeed(0.6).label, "0.6×")
    }

    func testSetSpeed() {
        let k = karaoke()
        k.setSpeed(PlaybackSpeed(0.6))
        XCTAssertEqual(k.speed.value, 0.6, accuracy: 1e-9)
    }

    func testHighlightDelegatesToTrack() {
        let k = karaoke()
        XCTAssertEqual(k.highlightedToken(atMillis: 350), 1)
    }

    func testSeekToToken() {
        let k = karaoke()
        XCTAssertEqual(k.seekMillis(toTokenAt: 2), 600)
        XCTAssertEqual(k.seekMillis(toTokenAt: 99), 0) // defensive fallback
    }

    func testBlindModeHidesTextUntilReveal() {
        let k = karaoke()
        XCTAssertFalse(k.textHidden)          // not blind
        k.setBlind(true)
        XCTAssertTrue(k.textHidden)           // blind, not revealed
        k.reveal()
        XCTAssertFalse(k.textHidden)          // revealed
    }

    func testEnteringBlindReHidesText() {
        let k = karaoke()
        k.setBlind(true)
        k.reveal()
        XCTAssertFalse(k.textHidden)
        k.setBlind(false)
        k.setBlind(true) // re-entering blind hides again
        XCTAssertTrue(k.textHidden)
    }
}
