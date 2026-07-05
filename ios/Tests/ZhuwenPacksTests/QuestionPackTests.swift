import XCTest
@testable import ZhuwenPacks

/// CP-07: the vendored pack ships 3 comprehension questions per story (FR-6.1) that the M8
/// comprehension check reads. (Question text is a CP-01 stub; real generation is CP-09.)
final class QuestionPackTests: XCTestCase {
    func testEveryStoryHasThreeQuestions() throws {
        let store = try Fixtures.store()
        let stories = try store.stories()
        XCTAssertFalse(stories.isEmpty)
        for s in stories {
            let qs = try store.questions(for: s.id)
            XCTAssertEqual(qs.count, 3, "story \(s.id) should ship 3 questions")
            for q in qs {
                XCTAssertEqual(q.storyID, s.id)
                XCTAssertFalse(q.promptZH.isEmpty)
                XCTAssertGreaterThanOrEqual(q.options.count, 2)
                XCTAssertTrue(q.options.indices.contains(q.answerIdx),
                              "answerIdx \(q.answerIdx) out of range for \(q.id)")
            }
        }
    }

    func testQuestionsOrderedById() throws {
        let store = try Fixtures.store()
        let story = try XCTUnwrap(try store.stories().first)
        let qs = try store.questions(for: story.id)
        XCTAssertEqual(qs.map(\.id), qs.map(\.id).sorted())
    }

    func testUnknownStoryHasNoQuestions() throws {
        let store = try Fixtures.store()
        XCTAssertTrue(try store.questions(for: "does-not-exist").isEmpty)
    }
}
