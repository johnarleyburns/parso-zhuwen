import XCTest
@testable import ZhuwenCore

final class LearnerArchiveTests: XCTestCase {
    /// A representative log across every event kind that touches state.
    private func sampleEvents(_ t0: Date) -> [Event] {
        [
            .exposure(1, storyID: "s1", at: t0),
            .exposure(2, storyID: "s1", at: t0.addingTimeInterval(1)),
            .lookup(2, storyID: "s1", at: t0.addingTimeInterval(2)),
            .comprehension(1, correct: true, storyID: "s1", at: t0.addingTimeInterval(3)),
            .reviewGrade(1, grade: Rating.good.rawValue, at: t0.addingTimeInterval(4)),
            .listen(storyID: "s1", blind: true, at: t0.addingTimeInterval(5)),
            .markKnown(3, at: t0.addingTimeInterval(6)),
        ]
    }

    /// The CP-08 acceptance: export → erase → import reproduces learner state exactly.
    func testExportEraseImportRoundTrips() throws {
        let t0 = Date(timeIntervalSince1970: 1_700_000_000)
        let seed: [Int: Double] = [5: 0.7, 6: 0.9]
        let events = sampleEvents(t0)
        let original = KnownWordModel.project(events, seed: seed)

        // Export everything (FR-10.3).
        let archive = LearnerArchive(events: events, seed: seed, exportedAt: t0)
        let json = try archive.encoded()

        // Erase: nothing left.
        let erased = KnownWordModel.project([], seed: [:])
        XCTAssertTrue(erased.states.isEmpty)

        // Import from the exported JSON and re-project.
        let decoded = try LearnerArchive.decoded(from: json)
        let restored = decoded.projectedModel()

        XCTAssertEqual(restored, original, "reprojected model must equal the exported one")
        XCTAssertEqual(decoded.events, events)
        XCTAssertEqual(decoded.seed, seed)
        // FSRS memory survives the round trip (the graded word carries a card).
        XCTAssertNotNil(restored.states[1]?.fsrs)
    }

    func testJSONIsStableAndCodable() throws {
        let t0 = Date(timeIntervalSince1970: 1_700_000_000)
        let archive = LearnerArchive(events: sampleEvents(t0), seed: [1: 0.5], exportedAt: t0)
        let a = try archive.encoded()
        let b = try LearnerArchive.decoded(from: a).encoded()
        XCTAssertEqual(a, b, "encode∘decode∘encode is stable")
    }

    func testEmptyArchiveImportsToEmptyModel() throws {
        let json = try LearnerArchive(events: [], seed: [:]).encoded()
        let model = try LearnerArchive.decoded(from: json).projectedModel()
        XCTAssertTrue(model.states.isEmpty)
    }

    func testFutureSchemaRejected() throws {
        let future = LearnerArchive(events: [], seed: [:],
                                    schemaVersion: LearnerArchive.currentSchemaVersion + 1)
        let json = try future.encoded()
        XCTAssertThrowsError(try LearnerArchive.decoded(from: json)) { error in
            XCTAssertEqual(error as? LearnerArchive.ArchiveError,
                           .unsupportedSchema(LearnerArchive.currentSchemaVersion + 1))
        }
    }

    func testMalformedRejected() {
        XCTAssertThrowsError(try LearnerArchive.decoded(from: Data("not json".utf8)))
    }
}

final class LearnerSettingsTests: XCTestCase {
    func testDefaultsAreSaneAndSyncOff() {
        let s = LearnerSettings.default
        XCTAssertFalse(s.iCloudSyncEnabled, "iCloud sync is off by default (FR-10.2)")
        XCTAssertEqual(s.audioVoice, .pack)
        XCTAssertEqual(s.pinyinMode, .frontierOnly)
        XCTAssertTrue(s.frontierUnderline)
        XCTAssertGreaterThan(s.dailyReviewCap, 0)
    }

    func testCodableRoundTrip() throws {
        var s = LearnerSettings.default
        s.theme = .sepia
        s.audioVoice = .systemTTS
        s.iCloudSyncEnabled = true
        s.readerFontSize = 28
        let data = try s.encoded()
        XCTAssertEqual(LearnerSettings.decoded(from: data), s)
    }
}
