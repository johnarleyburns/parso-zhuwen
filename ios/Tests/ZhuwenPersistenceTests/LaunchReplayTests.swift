import XCTest
import SwiftData
import ZhuwenCore
@testable import ZhuwenPersistence

/// The point of MC-1: prove I5's replay guarantee holds **across a process boundary**. We append a
/// scripted history, tear down the `ModelContainer`, re-create it from the same store URL (a
/// simulated relaunch), and assert the rebuilt `WordState` equals the pre-teardown projection and
/// the event count/order are unchanged. The in-memory `EventLog` was previously the only backing,
/// so durability had never been tested — this closes that hole.
final class LaunchReplayTests: XCTestCase {
    private var storeDir: URL!
    private var storeURL: URL!

    private let t0 = Date(timeIntervalSince1970: 1_700_000_000)
    private func ts(_ n: Int) -> Date { t0.addingTimeInterval(Double(n)) }

    override func setUpWithError() throws {
        storeDir = FileManager.default.temporaryDirectory
            .appendingPathComponent("zhuwen-replay-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: storeDir, withIntermediateDirectories: true)
        storeURL = storeDir.appendingPathComponent("learner.store")
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: storeDir)
    }

    private var scriptedHistory: [Event] {
        [
            .exposure(1, storyID: "s1", at: ts(0)),
            .exposure(1, storyID: "s1", at: ts(1)),
            .lookup(2, storyID: "s1", at: ts(2)),
            .exposure(2, storyID: "s1", at: ts(3)),
            .reviewGrade(2, grade: 3, at: ts(4)),
            .comprehension(1, correct: true, storyID: "s1", at: ts(5)),
            .markKnown(3, at: ts(6)),
            .listen(storyID: "s1", blind: true, at: ts(7)),
            .lookup(1, storyID: "s2", at: ts(8)),
            .exposure(4, storyID: "s2", at: ts(9)),
        ]
    }

    func testLogSurvivesRelaunchAndReprojectsIdentically() throws {
        let seed: [Int: Double] = [7: 0.9, 1: 0.2]
        let history = scriptedHistory

        // Session 1 — append the history and capture the live projection, then let the
        // container deallocate (tear down).
        var before = KnownWordModel()
        var beforeEvents: [Event] = []
        try autoreleasepool {
            let container = try LearnerStore.container(url: storeURL)
            let log = PersistentEventLog(container: container)
            for e in history { log.append(e) }
            XCTAssertEqual(log.count, history.count)
            before = log.projectedModel(seed: seed)
            beforeEvents = log.events
        }

        // Session 2 — brand-new container over the same store URL (relaunch).
        let container2 = try LearnerStore.container(url: storeURL)
        let log2 = PersistentEventLog(container: container2)

        // (d) event count + (order) unchanged.
        XCTAssertEqual(log2.count, history.count)
        XCTAssertEqual(log2.events, beforeEvents, "event order must be preserved across relaunch")
        XCTAssertEqual(log2.events, history, "exact history must be preserved")

        // (c) rebuilt WordState equals the pre-teardown projection.
        let after = log2.projectedModel(seed: seed)
        XCTAssertEqual(after, before, "replayed model must equal the pre-relaunch projection (I5)")
    }

    func testAppendAfterRelaunchContinuesOrderMonotonically() throws {
        try autoreleasepool {
            let log = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
            log.append(.exposure(1, at: ts(0)))
            log.append(.exposure(2, at: ts(1)))
        }
        // Relaunch and append more — seq must continue, not restart, so order is stable.
        let log2 = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
        log2.append(.lookup(1, at: ts(2)))
        XCTAssertEqual(log2.count, 3)
        XCTAssertEqual(log2.events.map(\.kind), [.exposure, .exposure, .lookup])
        XCTAssertEqual(log2.events.map(\.wordID), [1, 2, 1])
    }

    func testEraseThenImportRoundTripsThroughStore() throws {
        let history = scriptedHistory
        let log = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
        log.append(contentsOf: history)
        let model = log.projectedModel()

        // Erase (whole-log reset) empties the store.
        log.replaceAll([])
        XCTAssertEqual(log.count, 0)
        XCTAssertTrue(log.events.isEmpty)

        // Import the same history back — the reprojected model is identical (round-trip identity).
        log.replaceAll(history)
        XCTAssertEqual(log.count, history.count)
        XCTAssertEqual(log.events, history)
        XCTAssertEqual(log.projectedModel(), model)
    }

    func testExportJSONRoundTripsAgainstInMemoryArchive() throws {
        let history = scriptedHistory
        let log = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
        log.append(contentsOf: history)

        let data = try log.exportJSON(seed: [1: 0.5])
        let archive = try LearnerArchive.decoded(from: data)
        XCTAssertEqual(archive.events, history)
        XCTAssertEqual(archive.seed, [1: 0.5])
        XCTAssertEqual(archive.projectedModel(), log.projectedModel(seed: [1: 0.5]))
    }
}
