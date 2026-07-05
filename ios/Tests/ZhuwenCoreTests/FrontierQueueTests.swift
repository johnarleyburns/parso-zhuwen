import XCTest
import ZhuwenPacks
@testable import ZhuwenCore

final class FrontierQueueTests: XCTestCase {
    // A small lexicon: two single-char words (好, 山) and two HSK-2 compounds. 好山 is built
    // entirely from the single chars; 河流 from unknown chars.
    private func lexicon() -> LexiconStore {
        LexiconStore([
            WordRecord(id: 1, simp: "好", pinyin: "hǎo", hsk3Level: 1, freqRank: 5),
            WordRecord(id: 2, simp: "山", pinyin: "shān", hsk3Level: 1, freqRank: 50),
            WordRecord(id: 3, simp: "好山", pinyin: "hǎoshān", hsk3Level: 2, freqRank: 1000),
            WordRecord(id: 4, simp: "河流", pinyin: "héliú", hsk3Level: 2, freqRank: 990),
        ])
    }

    func testOrdersByHSKThenFrequency() {
        let q = FrontierQueue(lexicon: lexicon())
        // Nothing known: HSK-1 before HSK-2; within a level, lower freqRank first.
        XCTAssertEqual(q.candidates(known: []), [1, 2, 4, 3])
    }

    func testCharFamiliarityBonusPromotesKnownComponentWords() {
        let q = FrontierQueue(lexicon: lexicon())
        // Learner knows both single chars (1, 2). Among the HSK-2 compounds, 好山 (both chars
        // known) should now outrank 河流 despite its higher freqRank — the FR-2.4 bonus.
        XCTAssertEqual(q.candidates(known: [1, 2]), [3, 4])
    }

    func testHSKLevelDominatesTheBonus() {
        // Even a fully-familiar HSK-2 word never jumps ahead of an available HSK-1 word.
        let q = FrontierQueue(lexicon: lexicon())
        let ordered = q.candidates(known: [1]) // 1 known, 2 (HSK-1) still a candidate
        XCTAssertEqual(ordered.first, 2, "HSK-1 candidate must lead")
    }

    func testLimitAndFrontierSet() {
        let q = FrontierQueue(lexicon: lexicon())
        XCTAssertEqual(q.candidates(known: [], limit: 2), [1, 2])
        XCTAssertEqual(q.frontierSet(known: [1, 2], count: 1), [3])
    }
}
