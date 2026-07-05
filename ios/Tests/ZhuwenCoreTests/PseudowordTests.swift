import XCTest
import ZhuwenPacks
@testable import ZhuwenCore

final class PseudowordTests: XCTestCase {
    // A small lexicon of real 1- and 2-char words sharing characters.
    private func lexicon() -> LexiconStore {
        let words = [
            WordRecord(id: 1, simp: "医", pinyin: "yī", hsk3Level: 3, freqRank: 100),
            WordRecord(id: 2, simp: "院", pinyin: "yuàn", hsk3Level: 3, freqRank: 120),
            WordRecord(id: 3, simp: "医院", pinyin: "yīyuàn", hsk3Level: 3, freqRank: 300),
            WordRecord(id: 4, simp: "学", pinyin: "xué", hsk3Level: 1, freqRank: 40),
            WordRecord(id: 5, simp: "生", pinyin: "shēng", hsk3Level: 1, freqRank: 30),
            WordRecord(id: 6, simp: "学生", pinyin: "xuéshēng", hsk3Level: 1, freqRank: 200),
            WordRecord(id: 7, simp: "老", pinyin: "lǎo", hsk3Level: 2, freqRank: 60),
            WordRecord(id: 8, simp: "师", pinyin: "shī", hsk3Level: 2, freqRank: 80),
        ]
        return LexiconStore(words)
    }

    func testFoilsAreTwoRealCharactersButNotRealWords() {
        let gen = PseudowordGenerator(lexicon: lexicon())
        let foils = gen.generate(count: 10, seed: 42, poolLimit: 100)
        let real: Set<String> = ["医", "院", "医院", "学", "生", "学生", "老", "师"]
        XCTAssertFalse(foils.isEmpty)
        for f in foils {
            XCTAssertEqual(f.count, 2, "foil must be a 2-character compound")
            XCTAssertFalse(real.contains(f), "foil \(f) must not be a real lexicon word")
            for ch in f { XCTAssertTrue(PseudowordGenerator.isCJK(ch)) }
        }
    }

    func testFoilsAreDistinct() {
        let foils = PseudowordGenerator(lexicon: lexicon()).generate(count: 8, seed: 7)
        XCTAssertEqual(Set(foils).count, foils.count)
    }

    func testGenerationIsDeterministicPerSeed() {
        let gen = PseudowordGenerator(lexicon: lexicon())
        XCTAssertEqual(gen.generate(count: 6, seed: 99), gen.generate(count: 6, seed: 99))
        XCTAssertNotEqual(gen.generate(count: 6, seed: 1), gen.generate(count: 6, seed: 2))
    }

    func testCharPoolIsFrequencyOrdered() {
        // 生(30) and 学(40) are the most frequent characters, so they lead the pool.
        let pool = PseudowordGenerator(lexicon: lexicon()).charPool
        XCTAssertEqual(pool.first, "生")
        XCTAssertEqual(Array(pool.prefix(2)), ["生", "学"])
    }
}
