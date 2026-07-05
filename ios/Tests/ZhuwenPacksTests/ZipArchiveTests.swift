import XCTest
@testable import ZhuwenPacks

final class ZipArchiveTests: XCTestCase {
    func testReadsStoredEntriesFromRealPack() throws {
        let zip = try ZipArchive(url: Fixtures.positivePack)
        XCTAssertTrue(zip.contains("manifest.json"))
        XCTAssertTrue(zip.contains("manifest.sig"))
        XCTAssertTrue(zip.contains("content.sqlite"))

        let man = try XCTUnwrap(zip.data(for: "manifest.json"))
        let obj = try JSONSerialization.jsonObject(with: man) as? [String: Any]
        XCTAssertEqual(obj?["id"] as? String, "a2")
    }

    func testMissingEntryReturnsNil() throws {
        let zip = try ZipArchive(url: Fixtures.positivePack)
        XCTAssertNil(zip.data(for: "nope.bin"))
    }

    func testRejectsNonZip() {
        XCTAssertThrowsError(try ZipArchive(data: Data([0x01, 0x02, 0x03, 0x04])))
    }
}
