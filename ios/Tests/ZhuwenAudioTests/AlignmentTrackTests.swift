import XCTest
import ZhuwenPacks
@testable import ZhuwenAudio

final class AlignmentTrackTests: XCTestCase {
    // 我(0)[250,510) 爱(1)[510,770) 中国(2)[770,1290) ‖gap‖ 你(3)[1610,1870) 好(4)[1870,2130)
    private func track() -> AlignmentTrack {
        AlignmentTrack([
            AlignmentToken(tokenIdx: 0, t0ms: 250, t1ms: 510),
            AlignmentToken(tokenIdx: 1, t0ms: 510, t1ms: 770),
            AlignmentToken(tokenIdx: 2, t0ms: 770, t1ms: 1290),
            AlignmentToken(tokenIdx: 3, t0ms: 1610, t1ms: 1870),
            AlignmentToken(tokenIdx: 4, t0ms: 1870, t1ms: 2130),
        ])
    }

    func testLeadInSilenceHighlightsNothing() {
        XCTAssertNil(track().index(atMillis: 0))
        XCTAssertNil(track().index(atMillis: 249))
    }

    func testResolvesActiveToken() {
        let t = track()
        XCTAssertEqual(t.index(atMillis: 250), 0)
        XCTAssertEqual(t.index(atMillis: 509), 0)
        XCTAssertEqual(t.index(atMillis: 510), 1)
        XCTAssertEqual(t.index(atMillis: 800), 2)
        XCTAssertEqual(t.index(atMillis: 1289), 2)
    }

    func testGapKeepsPreviousTokenHighlighted() {
        // In the silent gap [1290,1610) the just-finished token (2) stays lit.
        let t = track()
        XCTAssertEqual(t.index(atMillis: 1300), 2)
        XCTAssertEqual(t.index(atMillis: 1609), 2)
        XCTAssertEqual(t.index(atMillis: 1610), 3)
    }

    func testAfterEndHoldsLastToken() {
        let t = track()
        XCTAssertEqual(t.durationMillis, 2130)
        XCTAssertEqual(t.index(atMillis: 5000), 4)
    }

    func testStartMillisForSeek() {
        let t = track()
        XCTAssertEqual(t.startMillis(ofTokenAt: 3), 1610)
        XCTAssertNil(t.startMillis(ofTokenAt: 99))
    }

    func testEmptyTrack() {
        let t = AlignmentTrack([])
        XCTAssertTrue(t.isEmpty)
        XCTAssertEqual(t.durationMillis, 0)
        XCTAssertNil(t.index(atMillis: 100))
    }

    func testResolutionMatchesLinearScanForEveryToken() {
        let t = track()
        for iv in t.intervals {
            for ms in stride(from: iv.t0ms, to: iv.t1ms, by: 7) {
                XCTAssertEqual(t.index(atMillis: ms), iv.tokenIdx, "ms=\(ms)")
            }
        }
    }

    func testBuildsFromVendoredPackStory() throws {
        let store = try Fixtures.store()
        let story = try XCTUnwrap(try store.stories().first)
        let track = AlignmentTrack(try store.alignment(storyID: story.id))
        XCTAssertEqual(track.tokenCount, story.body.count)
        XCTAssertGreaterThan(track.durationMillis, 0)
        // Every body position resolves at its own start time.
        for iv in track.intervals {
            XCTAssertEqual(track.index(atMillis: iv.t0ms), iv.tokenIdx)
        }
    }
}
