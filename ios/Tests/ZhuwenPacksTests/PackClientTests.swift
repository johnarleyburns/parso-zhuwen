import XCTest
import ZhuwenPacks

/// A `PackFetcher` that serves bytes from an in-memory map keyed by URL — no sockets. Records every
/// requested URL so the test can assert an anonymous, identifier-free GET.
final class StubFetcher: PackFetcher, @unchecked Sendable {
    private let responses: [URL: Data]
    private(set) var requested: [URL] = []

    init(_ responses: [URL: Data]) { self.responses = responses }

    func fetch(_ url: URL) async throws -> Data {
        requested.append(url)
        guard let data = responses[url] else { throw PackClient.ClientError.badResponse }
        return data
    }
}

final class PackClientTests: XCTestCase {
    private var tmp: URL!

    override func setUpWithError() throws {
        tmp = FileManager.default.temporaryDirectory
            .appendingPathComponent("packclient-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: tmp)
    }

    private func client(fetcher: PackFetcher, catalog: URL) throws -> PackClient {
        try PackClient(catalogURL: catalog, publicKey: Fixtures.publicKey(),
                       packsDirectory: tmp.appendingPathComponent("packs"), fetcher: fetcher)
    }

    func testCatalogParses() async throws {
        let catURL = URL(string: "https://cdn.zhuwen.ai/catalog.json")!
        let cat = PackCatalog(packs: [
            RemotePack(id: "a2-v0", band: "A2", semver: "0.1.0", sizeBytes: 123,
                       url: URL(string: "https://cdn.zhuwen.ai/a2-v0.zpack")!)
        ])
        let data = try JSONEncoder().encode(cat)
        let c = try client(fetcher: StubFetcher([catURL: data]), catalog: catURL)
        let got = try await c.catalog()
        XCTAssertEqual(got, cat)
    }

    func testDownloadVerifiesAndInstalls() async throws {
        let catURL = URL(string: "https://cdn.zhuwen.ai/catalog.json")!
        let packURL = URL(string: "https://cdn.zhuwen.ai/fixture-a2-v0.zpack")!
        let packBytes = try Data(contentsOf: Fixtures.positivePack)
        let c = try client(fetcher: StubFetcher([packURL: packBytes]), catalog: catURL)

        let pack = RemotePack(id: "fixture-a2-v0", band: "A2", semver: "0.1.0",
                              sizeBytes: packBytes.count, url: packURL)
        let installed = try await c.download(pack)

        XCTAssertTrue(FileManager.default.fileExists(atPath: installed.path))
        XCTAssertTrue(c.isInstalled("fixture-a2-v0"))
        // The installed pack opens & verifies in PackStore (usable offline, FR-8.2).
        let store = try PackStore(url: installed, publicKey: Fixtures.publicKey())
        XCTAssertFalse(try store.stories().isEmpty)
    }

    func testTamperedDownloadIsRejectedAndNothingInstalled() async throws {
        let packURL = URL(string: "https://cdn.zhuwen.ai/tampered.zpack")!
        let bytes = try Data(contentsOf: Fixtures.tamperedPack)
        let c = try client(fetcher: StubFetcher([packURL: bytes]),
                           catalog: URL(string: "https://cdn.zhuwen.ai/catalog.json")!)
        let pack = RemotePack(id: "tampered", band: "A2", semver: "0.1.0",
                              sizeBytes: bytes.count, url: packURL)
        do {
            _ = try await c.download(pack)
            XCTFail("tampered pack must be rejected before install")
        } catch let e as PackClient.ClientError {
            guard case .verificationFailed = e else { return XCTFail("wrong error: \(e)") }
        }
        XCTAssertFalse(c.isInstalled("tampered"))
        XCTAssertTrue(c.installedPacks().isEmpty)
    }

    func testInsecureURLRejected() async throws {
        let insecure = URL(string: "http://cdn.zhuwen.ai/catalog.json")!
        let c = try client(fetcher: StubFetcher([:]), catalog: insecure)
        do {
            _ = try await c.catalog()
            XCTFail("http:// must be refused (ATS/I2)")
        } catch let e as PackClient.ClientError {
            guard case .insecureURL = e else { return XCTFail("wrong error: \(e)") }
        }
    }

    func testAnonymousGetHasNoIdentifiers() async throws {
        let packURL = URL(string: "https://cdn.zhuwen.ai/fixture-a2-v0.zpack")!
        let bytes = try Data(contentsOf: Fixtures.positivePack)
        let fetcher = StubFetcher([packURL: bytes])
        let c = try client(fetcher: fetcher, catalog: URL(string: "https://cdn.zhuwen.ai/catalog.json")!)
        _ = try await c.download(RemotePack(id: "fixture-a2-v0", band: "A2", semver: "0.1.0",
                                            sizeBytes: bytes.count, url: packURL))
        // The fetched URL is exactly the pack URL — no query string / identifiers appended.
        XCTAssertEqual(fetcher.requested, [packURL])
        XCTAssertNil(fetcher.requested.first?.query)
    }

    func testDeleteRemovesInstalledPack() async throws {
        let packURL = URL(string: "https://cdn.zhuwen.ai/fixture-a2-v0.zpack")!
        let bytes = try Data(contentsOf: Fixtures.positivePack)
        let c = try client(fetcher: StubFetcher([packURL: bytes]),
                           catalog: URL(string: "https://cdn.zhuwen.ai/catalog.json")!)
        let pack = RemotePack(id: "fixture-a2-v0", band: "A2", semver: "0.1.0",
                              sizeBytes: bytes.count, url: packURL)
        _ = try await c.download(pack)
        XCTAssertEqual(c.installedPacks().count, 1)
        XCTAssertGreaterThan(c.sizeOnDisk("fixture-a2-v0"), 0)

        try c.delete(packID: "fixture-a2-v0")
        XCTAssertFalse(c.isInstalled("fixture-a2-v0"))
        XCTAssertTrue(c.installedPacks().isEmpty)
    }

    func testRedownloadReinstalls() async throws {
        let packURL = URL(string: "https://cdn.zhuwen.ai/fixture-a2-v0.zpack")!
        let bytes = try Data(contentsOf: Fixtures.positivePack)
        let c = try client(fetcher: StubFetcher([packURL: bytes]),
                           catalog: URL(string: "https://cdn.zhuwen.ai/catalog.json")!)
        let pack = RemotePack(id: "fixture-a2-v0", band: "A2", semver: "0.1.0",
                              sizeBytes: bytes.count, url: packURL)
        _ = try await c.download(pack)
        _ = try await c.redownload(pack)
        XCTAssertTrue(c.isInstalled("fixture-a2-v0"))
        XCTAssertEqual(c.installedPacks().count, 1)
    }
}
