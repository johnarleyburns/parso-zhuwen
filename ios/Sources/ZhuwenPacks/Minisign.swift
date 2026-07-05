import CryptoKit
import Foundation

/// minisign verification (legacy pure-ed25519 `Ed` variant), mirroring the factory's
/// `internal/minisign` (see PACK_FORMAT.md). Ed25519 via CryptoKit; no dependencies.
public enum Minisign {
    public enum MinisignError: Error, Equatable {
        case malformedKey
        case malformedSignature
        case unsupportedAlgorithm
        case keyIDMismatch
        case invalidSignature
        case invalidTrustedComment
    }

    public struct PublicKey {
        public let keyID: [UInt8] // 8 bytes
        public let key: Curve25519.Signing.PublicKey

        /// Parse a minisign public-key file (2-line: comment + base64(alg||keyID||pk)).
        public init(file text: String) throws {
            let lines = text.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
            guard lines.count >= 2, let blob = Data(base64Encoded: lines[1].trimmingCharacters(in: .whitespaces)) else {
                throw MinisignError.malformedKey
            }
            guard blob.count == 2 + 8 + 32 else { throw MinisignError.malformedKey }
            let b = [UInt8](blob)
            guard b[0] == UInt8(ascii: "E"), b[1] == UInt8(ascii: "d") else { throw MinisignError.unsupportedAlgorithm }
            self.keyID = Array(b[2 ..< 10])
            self.key = try Curve25519.Signing.PublicKey(rawRepresentation: Data(b[10 ..< 42]))
        }
    }

    /// Verify a minisign signature file over `message`.
    public static func verify(publicKey pub: PublicKey, message: Data, signatureFile text: String) throws {
        let lines = text.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        guard lines.count >= 4 else { throw MinisignError.malformedSignature }

        guard let blob = Data(base64Encoded: lines[1].trimmingCharacters(in: .whitespaces)) else {
            throw MinisignError.malformedSignature
        }
        let b = [UInt8](blob)
        guard b.count == 2 + 8 + 64 else { throw MinisignError.malformedSignature }
        guard b[0] == UInt8(ascii: "E"), b[1] == UInt8(ascii: "d") else { throw MinisignError.unsupportedAlgorithm }
        guard Array(b[2 ..< 10]) == pub.keyID else { throw MinisignError.keyIDMismatch }
        let sig = Data(b[10 ..< 74])
        guard pub.key.isValidSignature(sig, for: message) else { throw MinisignError.invalidSignature }

        let prefix = "trusted comment: "
        guard lines[2].hasPrefix(prefix) else { throw MinisignError.invalidTrustedComment }
        let trusted = String(lines[2].dropFirst(prefix.count))
        guard let global = Data(base64Encoded: lines[3].trimmingCharacters(in: .whitespaces)) else {
            throw MinisignError.malformedSignature
        }
        var globalMsg = sig
        globalMsg.append(Data(trusted.utf8))
        guard pub.key.isValidSignature(global, for: globalMsg) else { throw MinisignError.invalidTrustedComment }
    }
}
