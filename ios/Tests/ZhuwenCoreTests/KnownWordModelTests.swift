import XCTest
@testable import ZhuwenCore

final class KnownWordModelTests: XCTestCase {
    private let t0 = Date(timeIntervalSince1970: 1_700_000_000)
    private func ts(_ n: Int) -> Date { t0.addingTimeInterval(Double(n)) }

    // MARK: - I5 replay determinism

    func testProjectionEqualsIncrementalFold() {
        // "WordState is a rebuildable projection" — folding events one-by-one from empty
        // must equal projecting the whole list at once (00 §2 I5; §9 replay test).
        let events: [Event] = [
            .exposure(1, at: ts(0)),
            .exposure(1, at: ts(1)),
            .lookup(2, at: ts(2)),
            .exposure(2, at: ts(3)),
            .reviewGrade(2, grade: 3, at: ts(4)),
            .comprehension(1, correct: true, at: ts(5)),
            .markKnown(3, at: ts(6)),
            .lookup(1, at: ts(7)),
        ]

        let batch = KnownWordModel.project(events)

        var incremental = KnownWordModel()
        for e in events { incremental = incremental.applying(e) }

        XCTAssertEqual(batch, incremental)

        // Re-projecting the same events reproduces the model exactly (deterministic replay).
        XCTAssertEqual(KnownWordModel.project(events), batch)
    }

    func testReplayIsOrderSensitiveButDeterministic() {
        let a: [Event] = [.lookup(1, at: ts(0)), .markKnown(1, at: ts(1))]
        let b: [Event] = [.markKnown(1, at: ts(0)), .lookup(1, at: ts(1))]
        // Same events, different order → different P(known); each is deterministic.
        XCTAssertEqual(KnownWordModel.project(a).pKnown(1), 1.0, accuracy: 1e-9)   // marked last
        XCTAssertEqual(KnownWordModel.project(b).pKnown(1), 0.5, accuracy: 1e-9)   // looked up last
        XCTAssertEqual(KnownWordModel.project(a), KnownWordModel.project(a))
    }

    // MARK: - Transition semantics (FR-2.1 / FR-2.2)

    func testExposureRaisesAndLookupLowersPKnown() {
        let seen = KnownWordModel.project([.exposure(1, at: ts(0))])
        XCTAssertEqual(seen.pKnown(1), 0.05, accuracy: 1e-9)
        XCTAssertEqual(seen.state(for: 1).state, .introduced)

        let looked = KnownWordModel.project([.markKnown(1, at: ts(0)), .lookup(1, at: ts(1))])
        XCTAssertEqual(looked.pKnown(1), 0.5, accuracy: 1e-9)
        XCTAssertEqual(looked.state(for: 1).state, .learning)
        XCTAssertEqual(looked.state(for: 1).lookups, 1)
    }

    func testMarkKnownReachesKnownAndMastery() {
        let known = KnownWordModel.project([.markKnown(1, at: ts(0))])
        XCTAssertEqual(known.state(for: 1).state, .known) // pKnown 1.0 but no grades yet

        let mastered = KnownWordModel.project([
            .markKnown(1, at: ts(0)),
            .reviewGrade(1, grade: 3, at: ts(1)),
            .reviewGrade(1, grade: 2, at: ts(2)),
        ])
        XCTAssertEqual(mastered.state(for: 1).goodGrades, 2)
        XCTAssertEqual(mastered.state(for: 1).state, .mastered)
    }

    func testUnseenWordHasDefaultState() {
        let m = KnownWordModel()
        XCTAssertEqual(m.state(for: 42).state, .unseen)
        XCTAssertEqual(m.pKnown(42), 0)
    }

    func testFirstAndLastSeenTracked() {
        let m = KnownWordModel.project([.exposure(1, at: ts(0)), .exposure(1, at: ts(9))])
        XCTAssertEqual(m.state(for: 1).firstSeen, ts(0))
        XCTAssertEqual(m.state(for: 1).lastSeen, ts(9))
        XCTAssertEqual(m.state(for: 1).exposures, 2)
    }

    // MARK: - Effective known set (FR-2.3)

    func testEffectiveKnownSetIncludesKnownPlusLearningFrontier() {
        let m = KnownWordModel.project([
            .markKnown(1, at: ts(0)),                         // known (pKnown 1)
            .markKnown(2, at: ts(1)), .lookup(2, at: ts(2)),  // pKnown 0.5 → learning
            .exposure(3, at: ts(3)),                          // pKnown 0.05 → introduced
        ])
        // Without frontier context: only pKnown ≥ 0.8 counts.
        XCTAssertEqual(m.effectiveKnownSet(), [1])
        // Frontier word 2 is in `learning` ⇒ it joins the effective known set.
        XCTAssertEqual(m.effectiveKnownSet(frontier: [2, 3]), [1, 2])
        XCTAssertEqual(m.learningWords(), [2])
    }

    // MARK: - CP-07 acceptance: P(known) updates for exposure / lookup / grade paths

    func testPKnownUpdatesForExposureLookupGradePaths() {
        // handoff §6 CP-07: "P(known) updates verified for exposure/lookup/grade paths."
        let base = 0.5

        // Exposure raises P(known).
        let exposed = KnownWordModel.project([.markKnown(1, at: ts(0)), .lookup(1, at: ts(1)),
                                              .exposure(1, at: ts(2))])
        XCTAssertGreaterThan(exposed.pKnown(1), base)

        // Lookup lowers P(known).
        let looked = KnownWordModel.project([.markKnown(2, at: ts(0)), .lookup(2, at: ts(1))])
        XCTAssertLessThan(looked.pKnown(2), 1.0)
        XCTAssertEqual(looked.pKnown(2), 0.5, accuracy: 1e-9)

        // Grade path — a good grade raises P(known) AND advances FSRS memory (FR-7.3).
        let good = KnownWordModel.project([.markKnown(3, at: ts(0)), .lookup(3, at: ts(1)),
                                           .reviewGrade(3, grade: 2, at: ts(2))])
        XCTAssertGreaterThan(good.pKnown(3), base)
        XCTAssertNotNil(good.state(for: 3).fsrs, "a review grade must create FSRS memory")
        XCTAssertEqual(good.state(for: 3).goodGrades, 1)

        // Grade path — a failing grade lowers P(known) and records a lapse.
        let fail = KnownWordModel.project([.markKnown(4, at: ts(0)),
                                           .reviewGrade(4, grade: 0, at: ts(1))])
        XCTAssertLessThan(fail.pKnown(4), 1.0)
        XCTAssertEqual(fail.state(for: 4).fsrs?.lapses, 1)
    }

    func testReviewGradeFSRSIsReplayable() {
        // FSRS card state is reconstructed from the log ⇒ the projection stays deterministic (I5).
        let events: [Event] = [
            .reviewGrade(1, grade: 2, at: ts(0)),
            .reviewGrade(1, grade: 2, at: ts(90_000)),
            .reviewGrade(1, grade: 0, at: ts(180_000)),
        ]
        let batch = KnownWordModel.project(events)
        var incremental = KnownWordModel()
        for e in events { incremental = incremental.applying(e) }
        XCTAssertEqual(batch, incremental)
        XCTAssertEqual(batch.state(for: 1).fsrs, incremental.state(for: 1).fsrs)
    }

    // MARK: - Placement seed (CP-05 forward-compat)

    func testSeedInitializesStatesWithoutEvents() {
        let m = KnownWordModel.project([], seed: [1: 0.9, 2: 0.5, 3: 0.0])
        XCTAssertEqual(m.state(for: 1).state, .known)
        XCTAssertEqual(m.state(for: 2).state, .learning)
        XCTAssertEqual(m.state(for: 3).state, .unseen)
    }

    // MARK: - Listening events (CP-06): tracked in the log, separate from reading P(known)

    func testListenEventDoesNotAlterReadingPKnown() {
        // FR-5.2: a listening pass is recorded but does not move the reading-oriented model.
        let base = KnownWordModel.project([.exposure(1, at: ts(0))])
        let withListen = KnownWordModel.project([
            .exposure(1, at: ts(0)),
            .listen(storyID: "s1", blind: true, at: ts(1)),
            .listen(storyID: "s1", blind: false, at: ts(2)),
        ])
        XCTAssertEqual(withListen.pKnown(1), base.pKnown(1), accuracy: 1e-12)
        XCTAssertEqual(withListen, base) // listen events leave the projection untouched
    }

    func testListenEventReplaysDeterministicallyInLog() {
        let events: [Event] = [
            .listen(storyID: "s1", blind: false, at: ts(0)),
            .exposure(1, at: ts(1)),
            .listen(storyID: "s1", blind: true, at: ts(2)),
        ]
        let batch = KnownWordModel.project(events)
        var incremental = KnownWordModel()
        for e in events { incremental = incremental.applying(e) }
        XCTAssertEqual(batch, incremental)
        // The event survives a Codable round-trip with its blind payload (SwiftData persistence).
        let coded = try! JSONEncoder().encode(events[2])
        let decoded = try! JSONDecoder().decode(Event.self, from: coded)
        XCTAssertEqual(decoded.kind, .listen)
        XCTAssertEqual(decoded.blind, true)
        XCTAssertEqual(decoded.storyID, "s1")
    }
}
