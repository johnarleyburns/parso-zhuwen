import XCTest
@testable import ZhuwenPacks

/// CP-08a: the vendored pack ships Foundations F0 cards (FR-11) that resolve to fully
/// provenanced Commons images (I6, extended to `foundations_card`), read by the on-device
/// Foundations engine + M14 UI.
final class FoundationsPackTests: XCTestCase {
    func testFixturePackShipsFoundationsCards() throws {
        let store = try Fixtures.store()
        let cards = try store.foundationsCards()
        XCTAssertFalse(cards.isEmpty, "fixture pack should ship F0 cards")
        for c in cards {
            XCTAssertEqual(c.stage, "F0")
            XCTAssertFalse(c.imageID.isEmpty)
            XCTAssertFalse(c.setID.isEmpty)
        }
    }

    func testFoundationsCardsOrderedByWordID() throws {
        let cards = try Fixtures.store().foundationsCards()
        XCTAssertEqual(cards.map(\.wordID), cards.map(\.wordID).sorted())
    }

    func testDistractorsAreAlreadyTaughtPredecessors() throws {
        // FR-11.3: each card's distractors are drawn only from earlier (already-taught) words.
        let cards = try Fixtures.store().foundationsCards()
        var taught = Set<Int>()
        for c in cards {
            for d in c.distractorIDs {
                XCTAssertTrue(taught.contains(d), "distractor \(d) for word \(c.wordID) not yet taught")
                XCTAssertNotEqual(d, c.wordID)
            }
            taught.insert(c.wordID)
        }
    }

    func testEveryFoundationsImageIsFullyProvenanced() throws {
        let store = try Fixtures.store()
        let cards = try store.foundationsCards()
        let images = Dictionary(uniqueKeysWithValues: try store.images().map { ($0.id, $0) })
        for c in cards {
            let image = try XCTUnwrap(images[c.imageID], "card \(c.wordID) references missing image")
            XCTAssertFalse(image.license.isEmpty)
            XCTAssertFalse(image.author.isEmpty)
            XCTAssertFalse(image.sourceURL.isEmpty)
            XCTAssertFalse(image.attribution.isEmpty)
        }
    }

    func testVerifyFoundationsI6Passes() throws {
        let store = try Fixtures.store()
        XCTAssertNoThrow(try store.database.verifyFoundationsI6())
    }
}
