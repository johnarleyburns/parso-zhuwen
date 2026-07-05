import XCTest
@testable import ZhuwenPacks

/// Integration: the Swift reference verifier must accept the vendored positive pack and
/// reject all three golden negatives — mirroring the Go golden suite (handoff §7: one set
/// of vectors, two implementations that must agree).
final class PackVerifierTests: XCTestCase {
    func testAcceptsPositivePack() throws {
        let store = try Fixtures.store()
        XCTAssertEqual(store.manifest.id, "a2")
        XCTAssertEqual(store.manifest.schemaVersion, 1)
        XCTAssertEqual(store.manifest.lexiconVersion, "fixture-hsk3.0-v0")
    }

    func testRejectsUnsigned() throws {
        XCTAssertThrowsError(try PackStore(url: Fixtures.unsignedPack, publicKey: try Fixtures.publicKey())) { err in
            XCTAssertEqual(err as? PackVerifier.VerifyError, .unsigned)
        }
    }

    func testRejectsTamperedContent() throws {
        XCTAssertThrowsError(try PackStore(url: Fixtures.tamperedPack, publicKey: try Fixtures.publicKey())) { err in
            XCTAssertEqual(err as? PackVerifier.VerifyError, .hashMismatch("content.sqlite"))
        }
    }

    func testRejectsImageless() throws {
        // Valid signature + hashes, but content-level I6 fails.
        XCTAssertThrowsError(try PackStore(url: Fixtures.imagelessPack, publicKey: try Fixtures.publicKey())) { err in
            guard case ContentDatabase.DBError.i6 = err else {
                return XCTFail("expected I6 rejection, got \(err)")
            }
        }
    }

    func testRejectsUnknownLexiconVersion() throws {
        XCTAssertThrowsError(
            try PackStore(url: Fixtures.positivePack, publicKey: try Fixtures.publicKey(),
                          knownLexiconVersions: ["some-other-version"])
        ) { err in
            XCTAssertEqual(err as? PackVerifier.VerifyError, .unknownLexiconVersion("fixture-hsk3.0-v0"))
        }
    }

    func testAcceptsKnownLexiconVersion() throws {
        XCTAssertNoThrow(
            try PackStore(url: Fixtures.positivePack, publicKey: try Fixtures.publicKey(),
                          knownLexiconVersions: ["fixture-hsk3.0-v0"])
        )
    }

    func testStoreQueriesReturnContent() throws {
        let store = try Fixtures.store()
        let stories = try store.stories()
        XCTAssertEqual(stories.count, 10)
        XCTAssertTrue(stories.allSatisfy { !$0.coverImageID.isEmpty })
        XCTAssertTrue(stories.allSatisfy { !$0.body.isEmpty })

        let lex = try store.lexicon()
        XCTAssertEqual(lex.count, 30)
        XCTAssertTrue(lex.contains { $0.simp == "坚持" })
    }
}
