import XCTest
@testable import ZhuwenCore
import ZhuwenPacks

final class ReviewTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 1_700_000_000)
    private func daysAgo(_ d: Double) -> Date { now.addingTimeInterval(-d * 86_400) }

    private let lexicon = [
        WordRecord(id: 1, simp: "市场", pinyin: "shìchǎng", hsk3Level: 2, freqRank: 800),
        WordRecord(id: 2, simp: "热闹", pinyin: "rènao", hsk3Level: 3, freqRank: 1500),
        WordRecord(id: 3, simp: "非常", pinyin: "fēicháng", hsk3Level: 2, freqRank: 400),
    ]

    private func story() -> StoryRecord {
        let body = [
            BodyToken(w: 1, lit: nil, s: 0, pn: false),   // 市场
            BodyToken(w: -1, lit: "里人很多，", s: 0, pn: false),
            BodyToken(w: 3, lit: nil, s: 0, pn: false),   // 非常
            BodyToken(w: 2, lit: nil, s: 0, pn: false),   // 热闹
            BodyToken(w: 1, lit: nil, s: 1, pn: false),
        ]
        return StoryRecord(id: "market", titleZH: "周末的市场", titleEN: "The Weekend Market",
                           band: "A2", hsk3Level: 2, tokenCount: 5, typeCount: 3,
                           coverImageID: "img", canonID: "c", tier: "1", origin: "o",
                           newTypeIDs: [2], body: body)
    }

    // MARK: - Due selection & cap (FR-7.1/7.2)

    func testOnlyDueWordsSurface() {
        // Word 2 graded 2 days ago (short interval ⇒ due now); word 3 graded just now (not due).
        let events: [Event] = [
            .reviewGrade(2, grade: 0, at: daysAgo(2)),  // Again → ~1 day interval → due
            .reviewGrade(3, grade: 3, at: now),         // Easy just now → far future
        ]
        let model = KnownWordModel.project(events)
        let cards = ReviewScheduler().dueCards(model: model, stories: [story()], lexicon: lexicon,
                                               readStoryIDs: ["market"], now: now)
        XCTAssertEqual(cards.map(\.wordID), [2])
    }

    func testCapLimitsQueue() {
        let events = (1...3).map { Event.reviewGrade($0, grade: 0, at: daysAgo(5)) }
        let model = KnownWordModel.project(events)
        let cards = ReviewScheduler(dailyCap: 2).dueCards(model: model, stories: [story()],
                                                          lexicon: lexicon, readStoryIDs: ["market"], now: now)
        XCTAssertEqual(cards.count, 2)
    }

    func testDefaultCapIsTwenty() {
        XCTAssertEqual(ReviewScheduler.defaultDailyCap, 20)
    }

    // MARK: - Sentence-context (FR-7.1)

    func testCardSentenceContainsTargetInReadStory() {
        let model = KnownWordModel.project([.reviewGrade(2, grade: 0, at: daysAgo(2))])
        let cards = ReviewScheduler().dueCards(model: model, stories: [story()], lexicon: lexicon,
                                               readStoryIDs: ["market"], now: now)
        let card = try! XCTUnwrap(cards.first)
        XCTAssertEqual(card.simp, "热闹")
        XCTAssertEqual(card.storyTitleZH, "周末的市场")
        XCTAssertTrue(card.sentenceText.contains("热闹"))
        XCTAssertTrue(card.sentence.contains { $0.isTarget && $0.text == "热闹" })
    }

    func testSkipsWordsWithoutAReadStorySentence() {
        // The word is due but its story was never read ⇒ no context ⇒ no card.
        let model = KnownWordModel.project([.reviewGrade(2, grade: 0, at: daysAgo(2))])
        let cards = ReviewScheduler().dueCards(model: model, stories: [story()], lexicon: lexicon,
                                               readStoryIDs: [], now: now)
        XCTAssertTrue(cards.isEmpty)
    }

    // MARK: - Grading feeds the model (FR-7.3)

    func testGradeAdvancesFSRSDue() {
        var model = KnownWordModel.project([.reviewGrade(2, grade: 0, at: daysAgo(2))])
        let dueBefore = model.state(for: 2).fsrs!.due
        model.apply(.reviewGrade(2, grade: 2, at: now))  // Good
        let dueAfter = model.state(for: 2).fsrs!.due
        XCTAssertGreaterThan(dueAfter, dueBefore)
        XCTAssertGreaterThan(dueAfter, now)
    }

    func testDueCountReflectsAllDueRegardlessOfCap() {
        let events = (1...3).map { Event.reviewGrade($0, grade: 0, at: daysAgo(5)) }
        let model = KnownWordModel.project(events)
        XCTAssertEqual(ReviewScheduler(dailyCap: 1).dueCount(model: model, now: now), 3)
    }
}
