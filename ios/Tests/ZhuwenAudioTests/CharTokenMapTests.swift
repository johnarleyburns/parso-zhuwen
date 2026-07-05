import XCTest
@testable import ZhuwenAudio

final class CharTokenMapTests: XCTestCase {
    // 我 爱 中国 你 好 → "我爱中国你好"; offsets 0,1,2,4,5
    private func map() -> CharTokenMap {
        CharTokenMap(tokenTexts: ["我", "爱", "中国", "你", "好"])
    }

    func testFullTextConcatenation() {
        XCTAssertEqual(map().fullText, "我爱中国你好")
        XCTAssertEqual(map().utf16Length, 6)
    }

    func testMapsRangeLocationToToken() {
        let m = map()
        XCTAssertEqual(m.tokenIndex(forCharacterOffset: 0), 0) // 我
        XCTAssertEqual(m.tokenIndex(forCharacterOffset: 1), 1) // 爱
        XCTAssertEqual(m.tokenIndex(forCharacterOffset: 2), 2) // 中 (first char of 中国)
        XCTAssertEqual(m.tokenIndex(forCharacterOffset: 3), 2) // 国 (second char of 中国)
        XCTAssertEqual(m.tokenIndex(forCharacterOffset: 4), 3) // 你
        XCTAssertEqual(m.tokenIndex(forCharacterOffset: 5), 4) // 好
    }

    func testNegativeOffsetIsNil() {
        XCTAssertNil(map().tokenIndex(forCharacterOffset: -1))
    }

    func testBeyondEndClampsToLastToken() {
        XCTAssertEqual(map().tokenIndex(forCharacterOffset: 99), 4)
    }

    func testEmpty() {
        let m = CharTokenMap(tokenTexts: [])
        XCTAssertEqual(m.tokenCount, 0)
        XCTAssertNil(m.tokenIndex(forCharacterOffset: 0))
    }
}
