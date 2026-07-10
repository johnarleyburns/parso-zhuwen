import XCTest
import SwiftData
import ZhuwenCore
@testable import ZhuwenPersistence

/// MC-1 replay performance budget: rebuilding the projection from a large log must fit inside the
/// NFR-1 launch budget on device. This measures reading + replaying 50k persisted events from a
/// file-backed store (the worst case a checkpoint row would optimize). Skipped in the fast CI unit
/// subset (`make test-unit`), run under the pre-push hook and recorded in `plans/mc-1-done.md`.
///
/// If this ever exceeds budget on target hardware, add the disposable `ProjectionCheckpoint` row
/// described in the plan (cache keyed by last event seq); the log remains the source of truth.
final class ReplayPerfTests: XCTestCase {
    private var storeDir: URL!
    private var storeURL: URL!

    override func setUpWithError() throws {
        storeDir = FileManager.default.temporaryDirectory
            .appendingPathComponent("zhuwen-perf-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: storeDir, withIntermediateDirectories: true)
        storeURL = storeDir.appendingPathComponent("learner.store")
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: storeDir)
    }

    func test50kEventReplayWithinLaunchBudget() throws {
        let n = 50_000
        let t0 = Date(timeIntervalSince1970: 1_700_000_000)
        var history: [Event] = []
        history.reserveCapacity(n)
        for i in 0..<n {
            let wid = (i % 400) + 1
            switch i % 5 {
            case 0: history.append(.exposure(wid, storyID: "s\(i % 50)", at: t0.addingTimeInterval(Double(i))))
            case 1: history.append(.lookup(wid, at: t0.addingTimeInterval(Double(i))))
            case 2: history.append(.reviewGrade(wid, grade: i % 4, at: t0.addingTimeInterval(Double(i))))
            case 3: history.append(.comprehension(wid, correct: i % 2 == 0, at: t0.addingTimeInterval(Double(i))))
            default: history.append(.exposure(wid, at: t0.addingTimeInterval(Double(i))))
            }
        }

        try autoreleasepool {
            let log = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
            log.append(contentsOf: history)
            XCTAssertEqual(log.count, n)
        }

        // Cold "relaunch": open a fresh container and replay the whole log.
        let start = Date()
        let log2 = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
        let model = log2.projectedModel()
        let elapsed = Date().timeIntervalSince(start)

        XCTAssertEqual(log2.count, n)
        XCTAssertFalse(model.states.isEmpty)
        // This is a wall-clock full-replay measurement and is sensitive to host load (CI shared
        // runners, a busy dev machine). The *real* NFR-1 launch guarantee is asserted by
        // `testCheckpointKeepsLaunchReplayUnderNFR1Budget` (600 ms, checkpoint fast-path); this
        // full-replay figure is a loose regression signal, so the ceiling is deliberately generous
        // to avoid load-induced flakiness. The recorded on-device figure (done note) is far under.
        XCTAssertLessThan(elapsed, 20.0, "50k-event full replay took \(elapsed)s — investigate if far above the recorded baseline")
        print("MC-1 replay: 50k events read+projected in \(String(format: "%.3f", elapsed))s")
    }

    /// With a checkpoint at the log head, a cold launch replays only the tail — comfortably inside
    /// the NFR-1 600 ms budget even at 50k events, and identical to a full replay (checkpoint is a
    /// pure cache). This is the MC-1.5 mitigation that a full 50k replay would otherwise need.
    func testCheckpointKeepsLaunchReplayUnderNFR1Budget() throws {
        let n = 50_000
        let t0 = Date(timeIntervalSince1970: 1_700_000_000)
        var history: [Event] = []
        history.reserveCapacity(n)
        for i in 0..<n { history.append(.exposure((i % 400) + 1, at: t0.addingTimeInterval(Double(i)))) }

        var full = KnownWordModel()
        try autoreleasepool {
            let log = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
            log.append(contentsOf: history)
            full = log.projectedModel()
            log.saveCheckpoint()          // snapshot at head (app would call on background/quit)
        }

        // Cold "relaunch" with the checkpoint present.
        let start = Date()
        let log2 = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
        let cached = log2.projectedModel()
        let elapsed = Date().timeIntervalSince(start)

        XCTAssertEqual(cached, full, "checkpoint fast-path must equal a full replay (pure cache)")
        XCTAssertLessThan(elapsed, 0.6, "checkpointed launch replay took \(elapsed)s (NFR-1 = 600 ms)")
        print("MC-1 replay (checkpointed): 50k events launched in \(String(format: "%.3f", elapsed))s")
    }

    /// A stale/mismatched checkpoint must never corrupt the model: a different seed forces a full
    /// replay, and appends after the checkpoint are folded on the fast path.
    func testCheckpointSeedMismatchFallsBackToFullReplay() throws {
        let history: [Event] = (0..<100).map { .exposure(($0 % 10) + 1, at: Date(timeIntervalSince1970: 1_700_000_000 + Double($0))) }
        let log = PersistentEventLog(container: try LearnerStore.container(url: storeURL))
        log.append(contentsOf: history)
        log.saveCheckpoint(seed: [1: 0.5])

        // Same seed → fast path; different seed → full replay. Both equal a fresh projection.
        XCTAssertEqual(log.projectedModel(seed: [1: 0.5]), KnownWordModel.project(history, seed: [1: 0.5]))
        XCTAssertEqual(log.projectedModel(seed: [2: 0.9]), KnownWordModel.project(history, seed: [2: 0.9]))

        // Appends after the checkpoint are folded on the fast path.
        log.append(.lookup(1, at: Date(timeIntervalSince1970: 1_800_000_000)))
        XCTAssertEqual(log.projectedModel(seed: [1: 0.5]), KnownWordModel.project(log.events, seed: [1: 0.5]))
    }
}
