import XCTest
@testable import ZhuwenCore
import ZhuwenPacks

final class ProgressTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 1_700_000_000)
    private func daysAgo(_ d: Double) -> Date { now.addingTimeInterval(-d * 86_400) }

    // A small lexicon: HSK1 words 1-4, HSK2 words 5-8.
    private let lexicon: [WordRecord] = (1...8).map {
        WordRecord(id: $0, simp: "w\($0)", pinyin: "p", hsk3Level: $0 <= 4 ? 1 : 2, freqRank: $0 * 100)
    }

    private let estimator = ProgressEstimator()

    // MARK: - Reading band tracks the known set

    func testReadingBandRisesWithCoverage() {
        // Know all four HSK1 words ⇒ HSK level 1, band A1.
        let seed = Dictionary(uniqueKeysWithValues: (1...4).map { ($0, 1.0) })
        let r = estimator.report(model: KnownWordModel.project([], seed: seed),
                                 events: [], lexicon: lexicon, seed: seed, now: now)
        XCTAssertEqual(r.hskLevel, 1)
        XCTAssertEqual(r.readingBand, .a1)
        XCTAssertEqual(r.wordsKnown, 4)
    }

    func testEmptyModelIsA0() {
        let r = estimator.report(model: KnownWordModel(), events: [], lexicon: lexicon, now: now)
        XCTAssertEqual(r.hskLevel, 0)
        XCTAssertEqual(r.readingBand, .a0)
        XCTAssertEqual(r.wordsKnown, 0)
    }

    func testWordsToNextHSKCountsUnknownNextLevel() {
        let seed = Dictionary(uniqueKeysWithValues: (1...4).map { ($0, 1.0) })
        let r = estimator.report(model: KnownWordModel.project([], seed: seed),
                                 events: [], lexicon: lexicon, seed: seed, now: now)
        // HSK1 done; next is HSK2 (words 5-8), none known ⇒ 4 to go.
        XCTAssertEqual(r.wordsToNextHSK, 4)
    }

    // MARK: - Both-skill separation (FR-6.3 / FR-5.2) — THE acceptance check for M10

    func testListeningEventsMoveListeningButNotReading() {
        let reading = estimator.report(model: KnownWordModel(), events: [], lexicon: lexicon, now: now)
        let listens: [Event] = [
            .listen(storyID: "s1", blind: true, at: daysAgo(1)),
            .listen(storyID: "s2", blind: true, at: daysAgo(1)),
            .listen(storyID: "s3", blind: false, at: daysAgo(1)),
        ]
        let withListen = estimator.report(model: KnownWordModel.project(listens),
                                          events: listens, lexicon: lexicon, now: now)
        // Reading is untouched by listening (skill separation)…
        XCTAssertEqual(withListen.readingBand, reading.readingBand)
        XCTAssertEqual(withListen.wordsKnown, reading.wordsKnown)
        // …but the listening band moved up from A0.
        XCTAssertGreaterThan(withListen.listeningBand, reading.listeningBand)
        XCTAssertGreaterThan(withListen.listeningConfidence, 0)
    }

    func testBlindListensCountMoreThanSighted() {
        let blind = (1...4).map { Event.listen(storyID: "b\($0)", blind: true, at: daysAgo(1)) }
        let sighted = (1...4).map { Event.listen(storyID: "s\($0)", blind: false, at: daysAgo(1)) }
        let rb = estimator.report(model: KnownWordModel(), events: blind, lexicon: lexicon, now: now)
        let rs = estimator.report(model: KnownWordModel(), events: sighted, lexicon: lexicon, now: now)
        XCTAssertGreaterThanOrEqual(rb.listeningBand, rs.listeningBand)
    }

    // MARK: - Growth series (replay)

    func testWeeklyGrowthIsMonotonicAndEndsAtKnownCount() {
        // Learn two words a few weeks apart via marks.
        let events: [Event] = [
            .markKnown(1, at: daysAgo(40)),
            .markKnown(2, at: daysAgo(20)),
            .markKnown(3, at: daysAgo(2)),
        ]
        let r = estimator.report(model: KnownWordModel.project(events),
                                 events: events, lexicon: lexicon, now: now, weeks: 8)
        XCTAssertEqual(r.weeklyKnownSeries.count, 8)
        // Cumulative ⇒ non-decreasing.
        for (a, b) in zip(r.weeklyKnownSeries, r.weeklyKnownSeries.dropFirst()) {
            XCTAssertLessThanOrEqual(a, b)
        }
        XCTAssertEqual(r.weeklyKnownSeries.last, r.wordsKnown)
        XCTAssertGreaterThan(r.wordsKnownThisWeek, 0)  // word 3 became known this week
    }

    // MARK: - Everything is an estimate (labels handled in UI; here we assert can-do exists)

    func testCanDoStatementPresent() {
        let r = estimator.report(model: KnownWordModel(), events: [], lexicon: lexicon, now: now)
        XCTAssertFalse(r.canDo.isEmpty)
    }
}
