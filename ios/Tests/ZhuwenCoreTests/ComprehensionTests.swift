import XCTest
@testable import ZhuwenCore
import ZhuwenPacks

final class ComprehensionTests: XCTestCase {
    private let now = Date(timeIntervalSince1970: 1_700_000_000)

    private func questions(_ answerIdxs: [Int]) -> [QuestionRecord] {
        answerIdxs.enumerated().map { i, ans in
            QuestionRecord(id: "s1-q\(i)", storyID: "s1", promptZH: "问题\(i)",
                           options: ["甲", "乙", "丙", "丁"], answerIdx: ans, band: "A2")
        }
    }

    private func session(answerKey: [Int], exposed: [Int] = [10, 20, 30]) -> ComprehensionSession {
        ComprehensionSession(storyID: "s1", questions: questions(answerKey), exposedWordIDs: exposed)
    }

    // MARK: - Pass / fail thresholds (FR-6.2)

    func testPassesAtTwoOfThree() {
        var s = session(answerKey: [0, 0, 0])
        s.answer(optionIndex: 0)  // correct
        s.answer(optionIndex: 0)  // correct
        s.answer(optionIndex: 1)  // wrong
        XCTAssertTrue(s.isComplete)
        XCTAssertEqual(s.correctCount, 2)
        XCTAssertTrue(s.passed)
        XCTAssertTrue(s.sealEarned)
    }

    func testFailsAtOneOfThree() {
        var s = session(answerKey: [0, 0, 0])
        s.answer(optionIndex: 0)  // correct
        s.answer(optionIndex: 1)  // wrong
        s.answer(optionIndex: 2)  // wrong
        XCTAssertEqual(s.correctCount, 1)
        XCTAssertFalse(s.passed)
        XCTAssertFalse(s.sealEarned)
    }

    func testPerfectScoreSeals() {
        var s = session(answerKey: [1, 2, 3])
        s.answer(optionIndex: 1); s.answer(optionIndex: 2); s.answer(optionIndex: 3)
        XCTAssertEqual(s.correctCount, 3)
        XCTAssertTrue(s.sealEarned)
    }

    // MARK: - Events → model (FR-6.2 "boosts P(known) for its words")

    func testPassBoostsPKnownForExposedWords() {
        var s = session(answerKey: [0, 0, 0], exposed: [10, 20])
        s.answer(optionIndex: 0); s.answer(optionIndex: 0); s.answer(optionIndex: 0)
        let events = s.completionEvents(at: now)
        XCTAssertEqual(events.count, 2)
        XCTAssertTrue(events.allSatisfy { $0.kind == .comprehension && $0.correct == true })

        let model = KnownWordModel.project(events)
        XCTAssertGreaterThan(model.pKnown(10), 0)
        XCTAssertGreaterThan(model.pKnown(20), 0)
    }

    func testFailRecordsNegativeOutcome() {
        var s = session(answerKey: [0, 0, 0], exposed: [10])
        s.answer(optionIndex: 1); s.answer(optionIndex: 1); s.answer(optionIndex: 1)
        let events = s.completionEvents(at: now)
        XCTAssertEqual(events.count, 1)
        XCTAssertEqual(events.first?.correct, false)
    }

    func testNoEventsUntilComplete() {
        var s = session(answerKey: [0, 0, 0])
        s.answer(optionIndex: 0)
        XCTAssertTrue(s.completionEvents(at: now).isEmpty)
    }

    func testFrontierWordMovesTowardLearningOnPass() {
        // A frontier word seen once (introduced) that passes comprehension advances.
        var s = session(answerKey: [0, 0, 0], exposed: [42])
        s.answer(optionIndex: 0); s.answer(optionIndex: 0); s.answer(optionIndex: 0)
        let events: [Event] = [
            .exposure(42, at: now),
            .exposure(42, at: now),
        ] + s.completionEvents(at: now)
        let model = KnownWordModel.project(events)
        XCTAssertGreaterThanOrEqual(model.pKnown(42), 0.1)
    }

    // MARK: - Exposed-word extraction from a story body

    func testExposedWordIDsSkipsLiteralsAndProperNouns() {
        let body = [
            BodyToken(w: 1, lit: nil, s: 0, pn: false),
            BodyToken(w: -1, lit: "，", s: 0, pn: false),   // literal
            BodyToken(w: -1, lit: "李明", s: 0, pn: true),   // proper noun
            BodyToken(w: 2, lit: nil, s: 0, pn: false),
            BodyToken(w: 1, lit: nil, s: 1, pn: false),      // repeat
        ]
        let story = StoryRecord(id: "s1", titleZH: "t", titleEN: "t", band: "A2", hsk3Level: 2,
                                tokenCount: 5, typeCount: 2, coverImageID: "img", canonID: "c",
                                tier: "1", origin: "o", newTypeIDs: [], body: body)
        XCTAssertEqual(ComprehensionSession.exposedWordIDs(of: story), [1, 2])
    }
}
