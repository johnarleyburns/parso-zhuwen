import XCTest
@testable import ZhuwenCore

final class FSRSTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 1_700_000_000)
    private let s = FSRSScheduler.default

    // MARK: - Interval ordering

    func testFirstReviewIntervalsAreOrdered() {
        // Again < Hard < Good < Easy for the very first review (FR-7.1 button previews).
        let ivl = s.intervals(nil, at: now)
        XCTAssertLessThanOrEqual(ivl[.again]!, ivl[.hard]!)
        XCTAssertLessThanOrEqual(ivl[.hard]!, ivl[.good]!)
        XCTAssertLessThanOrEqual(ivl[.good]!, ivl[.easy]!)
        XCTAssertGreaterThanOrEqual(ivl[.again]!, 1)
    }

    func testSubsequentReviewIntervalsAreOrdered() {
        let first = s.review(nil, rating: .good, at: now)
        let later = now.addingTimeInterval(first.due.timeIntervalSince(now))
        let ivl = s.intervals(first, at: later)
        XCTAssertLessThanOrEqual(ivl[.again]!, ivl[.hard]!)
        XCTAssertLessThanOrEqual(ivl[.hard]!, ivl[.good]!)
        XCTAssertLessThanOrEqual(ivl[.good]!, ivl[.easy]!)
    }

    // MARK: - Stability dynamics

    func testGoodGradesGrowStability() {
        var card = s.review(nil, rating: .good, at: now)
        let s1 = card.stability
        let t1 = card.due
        card = s.review(card, rating: .good, at: t1)
        XCTAssertGreaterThan(card.stability, s1)
        XCTAssertGreaterThan(card.due, t1)
        XCTAssertEqual(card.reps, 2)
        XCTAssertEqual(card.lapses, 0)
    }

    func testAgainReducesStabilityAndCountsLapse() {
        let good = s.review(nil, rating: .good, at: now)
        let lapsed = s.review(good, rating: .again, at: good.due)
        XCTAssertLessThanOrEqual(lapsed.stability, good.stability)
        XCTAssertEqual(lapsed.lapses, 1)
        // The next interval collapses toward "soon".
        XCTAssertLessThan(lapsed.due.timeIntervalSince(good.due), good.due.timeIntervalSince(now))
    }

    func testDifficultyStaysInRange() {
        var card: FSRSCard? = nil
        var t = now
        for g in [Rating.again, .again, .again, .easy, .easy, .good, .hard] {
            card = s.review(card, rating: g, at: t)
            t = card!.due
            XCTAssertGreaterThanOrEqual(card!.difficulty, 1)
            XCTAssertLessThanOrEqual(card!.difficulty, 10)
        }
    }

    // MARK: - Determinism (I5 replay)

    func testDeterministicForSameGradeSequence() {
        func run() -> FSRSCard {
            var card: FSRSCard? = nil
            var t = now
            for g in [Rating.good, .good, .hard, .easy] {
                card = s.review(card, rating: g, at: t)
                t = card!.due
            }
            return card!
        }
        XCTAssertEqual(run(), run())
    }

    func testRetrievabilityDecaysWithTime() {
        let card = s.review(nil, rating: .good, at: now)
        let rFresh = s.retrievability(elapsedDays: 0, stability: card.stability)
        let rStale = s.retrievability(elapsedDays: card.stability * 4, stability: card.stability)
        XCTAssertGreaterThan(rFresh, rStale)
        XCTAssertEqual(rFresh, 1, accuracy: 1e-9)
    }
}
