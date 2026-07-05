import XCTest
@testable import ZhuwenPacks

/// CP-06: the vendored pack now ships story audio + word-level alignment (FR-5.1). These
/// assert the pack plumbing the listening layer relies on: every story has an audio entry
/// present in the (hash-verified) archive, and its alignment has one strictly-increasing,
/// non-overlapping row per body token.
final class AudioPackTests: XCTestCase {
    func testEveryStoryHasAudioAndAlignment() throws {
        let store = try Fixtures.store()
        let stories = try store.stories()
        XCTAssertFalse(stories.isEmpty)
        for s in stories {
            XCTAssertNotNil(s.audioFile, "story \(s.id) missing audio_file")
            XCTAssertNotNil(store.audioData(for: s), "story \(s.id) audio bytes absent from archive")

            let align = try store.alignment(storyID: s.id)
            XCTAssertEqual(align.count, s.body.count,
                           "story \(s.id): \(align.count) alignment rows vs \(s.body.count) body tokens")
        }
    }

    func testAlignmentRowsAreOrderedAndNonOverlapping() throws {
        let store = try Fixtures.store()
        let story = try XCTUnwrap(try store.stories().first)
        let align = try store.alignment(storyID: story.id)
        XCTAssertGreaterThan(align.count, 1)
        for (i, a) in align.enumerated() {
            XCTAssertEqual(a.tokenIdx, i)
            XCTAssertLessThan(a.t0ms, a.t1ms, "row \(i) has non-positive duration")
            if i > 0 {
                XCTAssertGreaterThanOrEqual(a.t0ms, align[i - 1].t1ms, "row \(i) overlaps previous")
            }
        }
    }

    func testAudioURLExtractsPlayableFile() throws {
        let store = try Fixtures.store()
        let story = try XCTUnwrap(try store.stories().first)
        let url = try XCTUnwrap(try store.audioURL(for: story))
        XCTAssertTrue(FileManager.default.fileExists(atPath: url.path))
        // Cached: same URL on a second call.
        let again = try store.audioURL(for: story)
        XCTAssertEqual(url, again)
    }
}
