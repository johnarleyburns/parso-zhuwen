import Foundation
import XCTest
import ZhuwenPacks

/// Locates the vendored golden fixtures (ios/Fixtures) relative to this source file, so
/// tests run with `swift test` from the package root without bundling resources.
enum Fixtures {
    static var dir: URL {
        // .../ios/Tests/ZhuwenPacksTests/Fixtures.swift -> up 3 -> ios/
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Fixtures")
    }

    static func url(_ name: String) -> URL { dir.appendingPathComponent(name) }

    static var positivePack: URL { url("fixture-a2-v0.zpack") }
    static var unsignedPack: URL { url("golden-unsigned.zpack") }
    static var tamperedPack: URL { url("golden-tampered.zpack") }
    static var imagelessPack: URL { url("golden-imageless.zpack") }

    static func publicKey() throws -> Minisign.PublicKey {
        let text = try String(contentsOf: url("zhuwen-dev.pub"), encoding: .utf8)
        return try Minisign.PublicKey(file: text)
    }

    static func store() throws -> PackStore {
        try PackStore(url: positivePack, publicKey: try publicKey())
    }
}
