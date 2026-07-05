import XCTest
import ZhuwenPacks
@testable import ZhuwenCore

final class ReaderModelTests: XCTestCase {
    // MARK: - Unit (synthetic story, no pack)

    private func makeLexicon() -> [WordRecord] {
        [
            WordRecord(id: 1, simp: "山", pinyin: "shān", hsk3Level: 1, freqRank: 300),
            WordRecord(id: 11, simp: "坚持", pinyin: "jiānchí", hsk3Level: 4, freqRank: 1600),
        ]
    }

    func testTokensMapWordsAndFrontier() {
        let body = [
            BodyToken(w: 1, lit: nil, s: 0, pn: nil),
            BodyToken(w: 11, lit: nil, s: 0, pn: nil),   // frontier (in newTypeIDs)
            BodyToken(w: -1, lit: "后羿", s: 1, pn: true), // proper noun
            BodyToken(w: -1, lit: "太", s: 1, pn: nil),   // literal
        ]
        let story = StoryRecord(id: "s1", titleZH: "甲", titleEN: "A", band: "A2",
                                hsk3Level: 2, tokenCount: 4, typeCount: 2, coverImageID: "img",
                                canonID: "c", tier: "C5", origin: "canon", newTypeIDs: [11], body: body)
        let m = ReaderModel(story: story, lexicon: makeLexicon())
        let toks = m.tokens()

        XCTAssertEqual(toks.count, 4)
        XCTAssertEqual(toks[0].text, "山")
        XCTAssertEqual(toks[0].wordID, 1)
        XCTAssertEqual(toks[0].pinyin, "shān")
        XCTAssertFalse(toks[0].isFrontier)

        XCTAssertEqual(toks[1].text, "坚持")
        XCTAssertTrue(toks[1].isFrontier)

        XCTAssertTrue(toks[2].isProperNoun)
        XCTAssertNil(toks[2].wordID)
        XCTAssertEqual(toks[2].text, "后羿")

        XCTAssertEqual(toks[3].text, "太")
        XCTAssertNil(toks[3].wordID)

        XCTAssertEqual(m.sentenceCount(), 2)
    }

    func testGlossResolution() {
        let story = StoryRecord(id: "s1", titleZH: "甲", titleEN: "A", band: "A2",
                                hsk3Level: 2, tokenCount: 0, typeCount: 0, coverImageID: "img",
                                canonID: "c", tier: "C5", origin: "canon", newTypeIDs: [11], body: [])
        let m = ReaderModel(story: story, lexicon: makeLexicon())
        let g = m.gloss(for: 11)
        XCTAssertEqual(g?.simp, "坚持")
        XCTAssertEqual(g?.pinyin, "jiānchí")
        XCTAssertEqual(g?.hsk3Level, 4)
        XCTAssertNil(m.gloss(for: 999)) // unknown word
    }

    // MARK: - E2E (real vendored pack -> render + tap-to-gloss)

    func testRendersRealFixtureStoryWithGlosses() throws {
        let store = try Fixtures.store()
        let stories = try store.stories()
        let lex = try store.lexicon()
        let story = try XCTUnwrap(stories.first)

        let m = ReaderModel(story: story, lexicon: lex)
        let toks = m.tokens()
        XCTAssertFalse(toks.isEmpty, "reader produced no tokens")

        // Every in-lexicon word resolves to a gloss (tap-to-gloss from pack data).
        for t in toks where t.wordID != nil {
            XCTAssertNotNil(m.gloss(for: t.wordID!), "no gloss for word \(t.wordID!) (\(t.text))")
        }
        // The fixture introduces exactly the frontier word 坚持.
        XCTAssertTrue(toks.contains { $0.isFrontier && $0.text == "坚持" })
    }
}
