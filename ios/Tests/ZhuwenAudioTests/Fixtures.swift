import Foundation
import ZhuwenPacks

/// Locates the vendored golden fixtures (ios/Fixtures) relative to this source file.
enum Fixtures {
    static var dir: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Fixtures")
    }

    static func publicKey() throws -> Minisign.PublicKey {
        let text = try String(contentsOf: dir.appendingPathComponent("zhuwen-dev.pub"), encoding: .utf8)
        return try Minisign.PublicKey(file: text)
    }

    static func store() throws -> PackStore {
        try PackStore(url: dir.appendingPathComponent("fixture-a2-v0.zpack"), publicKey: try publicKey())
    }
}
