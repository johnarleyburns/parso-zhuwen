import XCTest
@testable import ZhuwenPacks

final class MinisignTests: XCTestCase {
    private func manifestAndSig() throws -> (Data, String, Minisign.PublicKey) {
        let zip = try ZipArchive(url: Fixtures.positivePack)
        let man = try XCTUnwrap(zip.data(for: "manifest.json"))
        let sig = String(decoding: try XCTUnwrap(zip.data(for: "manifest.sig")), as: UTF8.self)
        return (man, sig, try Fixtures.publicKey())
    }

    func testVerifiesRealSignature() throws {
        let (man, sig, pub) = try manifestAndSig()
        XCTAssertNoThrow(try Minisign.verify(publicKey: pub, message: man, signatureFile: sig))
    }

    func testRejectsTamperedMessage() throws {
        let (man, sig, pub) = try manifestAndSig()
        var bad = man
        bad.append(0x00)
        XCTAssertThrowsError(try Minisign.verify(publicKey: pub, message: bad, signatureFile: sig)) { err in
            XCTAssertEqual(err as? Minisign.MinisignError, .invalidSignature)
        }
    }

    func testRejectsTamperedTrustedComment() throws {
        let (man, sig, pub) = try manifestAndSig()
        let forged = sig.replacingOccurrences(of: "trusted comment: ", with: "trusted comment: forged ")
        XCTAssertThrowsError(try Minisign.verify(publicKey: pub, message: man, signatureFile: forged)) { err in
            XCTAssertEqual(err as? Minisign.MinisignError, .invalidTrustedComment)
        }
    }

    func testRejectsKeyIDMismatch() throws {
        let (man, sig, pub) = try manifestAndSig()
        // Flip a key_id byte inside the base64 signature blob (bytes 2..10).
        let lines = sig.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        var blob = [UInt8](Data(base64Encoded: lines[1])!)
        blob[3] ^= 0xFF
        var mutated = lines
        mutated[1] = Data(blob).base64EncodedString()
        let mutatedSig = mutated.joined(separator: "\n")
        XCTAssertThrowsError(try Minisign.verify(publicKey: pub, message: man, signatureFile: mutatedSig)) { err in
            XCTAssertEqual(err as? Minisign.MinisignError, .keyIDMismatch)
        }
    }

    func testParsesPublicKey() throws {
        let pub = try Fixtures.publicKey()
        XCTAssertEqual(pub.keyID.count, 8)
    }
}
