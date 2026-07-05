import CryptoKit
import Foundation

/// The reference on-device pack verifier (invariant I2/I3/I6; handoff §3, §5). Order:
/// (1) minisign signature over manifest.json, (2) every file sha256, (3) lexicon_version
/// acceptance, (4) content-level I6. Mirrors the factory `pack.Verify`.
public enum PackVerifier {
    public enum VerifyError: Error, Equatable {
        case missingManifest
        case unsigned
        case manifestParse
        case fileAbsent(String)
        case hashMismatch(String)
        case unknownLexiconVersion(String)
    }

    /// Verifies an already-opened archive. `knownLexiconVersions` empty = accept any.
    /// The content database (for I6) is provided by the caller since it must extract
    /// content.sqlite to a file first.
    public static func verify(
        archive: ZipArchive,
        publicKey: Minisign.PublicKey,
        knownLexiconVersions: Set<String> = [],
        contentDatabase: (Data) throws -> ContentDatabase
    ) throws -> PackManifest {
        guard let manBytes = archive.data(for: "manifest.json") else { throw VerifyError.missingManifest }
        guard let sigBytes = archive.data(for: "manifest.sig") else { throw VerifyError.unsigned }

        try Minisign.verify(publicKey: publicKey, message: manBytes,
                            signatureFile: String(decoding: sigBytes, as: UTF8.self))

        guard let manifest = try? JSONDecoder().decode(PackManifest.self, from: manBytes) else {
            throw VerifyError.manifestParse
        }

        for (name, wantHex) in manifest.files {
            guard let bytes = archive.data(for: name) else { throw VerifyError.fileAbsent(name) }
            let got = SHA256.hash(data: bytes).map { String(format: "%02x", $0) }.joined()
            if got != wantHex { throw VerifyError.hashMismatch(name) }
        }

        if !knownLexiconVersions.isEmpty && !knownLexiconVersions.contains(manifest.lexiconVersion) {
            throw VerifyError.unknownLexiconVersion(manifest.lexiconVersion)
        }

        guard let dbBytes = archive.data(for: "content.sqlite") else {
            throw VerifyError.fileAbsent("content.sqlite")
        }
        let db = try contentDatabase(dbBytes)
        try db.verifyI6()

        return manifest
    }
}
