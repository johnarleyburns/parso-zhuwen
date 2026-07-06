// swift-tools-version: 5.9
import PackageDescription

// Zhuwen iOS local SPM packages (handoff §1, §5). No third-party dependencies (NFR-5):
// Ed25519 + SHA-256 via CryptoKit, SQLite via the system SQLite3 module, zip via a small
// in-package reader. Core logic is testable on the macOS host with `swift test`.
let package = Package(
    name: "Zhuwen",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [
        .library(name: "ZhuwenPacks", targets: ["ZhuwenPacks"]),
        .library(name: "ZhuwenCore", targets: ["ZhuwenCore"]),
        .library(name: "ZhuwenAudio", targets: ["ZhuwenAudio"]),
        .library(name: "ZhuwenUI", targets: ["ZhuwenUI"]),
        .library(name: "ZhuwenPersistence", targets: ["ZhuwenPersistence"]),
    ],
    targets: [
        .target(name: "ZhuwenPacks"),
        .target(name: "ZhuwenCore", dependencies: ["ZhuwenPacks"]),
        .target(name: "ZhuwenAudio", dependencies: ["ZhuwenPacks"]),
        .target(name: "ZhuwenUI", dependencies: ["ZhuwenCore", "ZhuwenPacks", "ZhuwenAudio"]),
        .target(name: "ZhuwenPersistence", dependencies: ["ZhuwenCore"]),
        .testTarget(name: "ZhuwenPacksTests", dependencies: ["ZhuwenPacks"]),
        .testTarget(name: "ZhuwenCoreTests", dependencies: ["ZhuwenCore", "ZhuwenPacks"]),
        .testTarget(name: "ZhuwenAudioTests", dependencies: ["ZhuwenAudio", "ZhuwenPacks"]),
        .testTarget(name: "ZhuwenUITests", dependencies: ["ZhuwenUI", "ZhuwenCore", "ZhuwenPacks"]),
        .testTarget(name: "ZhuwenPersistenceTests", dependencies: ["ZhuwenPersistence", "ZhuwenCore"]),
    ]
)
