import Foundation

/// PackClient is the app's **single network chokepoint** (00 §2 I2). Additional band packs
/// (A1/A2/B1…) download from the CDN as an *anonymous* GET, are verified with the same signed
/// `PackVerifier` chain as vendored packs (minisign → per-file sha256 → lexicon → I6) *before*
/// they are installed, then are fully usable offline (FR-8.2). It also backs the pack-manager UI
/// (FR-8.3: list, size, delete, re-download).
///
/// `URLSession` is used **only** here, behind `URLSessionPackFetcher`; `grep-audit.sh` (handoff §8)
/// fails the build if `URLSession` appears anywhere else in the app.
public final class PackClient {
    public enum ClientError: Error, Equatable {
        case insecureURL(String)     // not https:// — refused (I2, ATS)
        case badResponse
        case catalogParse
        case verificationFailed(String)
    }

    private let catalogURL: URL
    private let publicKey: Minisign.PublicKey
    private let knownLexiconVersions: Set<String>
    private let packsDirectory: URL
    private let fetcher: PackFetcher
    private let fileManager: FileManager

    /// - Parameters:
    ///   - catalogURL: the signed pack catalog on the CDN (must be `https://`).
    ///   - publicKey: the pinned minisign key every downloaded pack is verified against.
    ///   - packsDirectory: where verified `.zpack`s are installed (created on demand).
    ///   - fetcher: network abstraction; defaults to an ephemeral, anonymous `URLSession`.
    public init(catalogURL: URL,
                publicKey: Minisign.PublicKey,
                packsDirectory: URL,
                knownLexiconVersions: Set<String> = [],
                fetcher: PackFetcher = URLSessionPackFetcher(),
                fileManager: FileManager = .default) {
        self.catalogURL = catalogURL
        self.publicKey = publicKey
        self.packsDirectory = packsDirectory
        self.knownLexiconVersions = knownLexiconVersions
        self.fetcher = fetcher
        self.fileManager = fileManager
    }

    // MARK: - Remote catalog

    /// Fetch and parse the CDN pack catalog (anonymous GET, no identifiers, FR-8.3).
    public func catalog() async throws -> PackCatalog {
        let data = try await fetch(catalogURL)
        guard let catalog = try? JSONDecoder().decode(PackCatalog.self, from: data) else {
            throw ClientError.catalogParse
        }
        return catalog
    }

    // MARK: - Download + verify + install

    /// Download a remote pack, **verify it before writing it to the install directory**, and return
    /// the installed URL. A tampered/unsigned/imageless download is rejected by `PackVerifier` and
    /// nothing is installed (I3/I6). The pack is fully usable offline thereafter (FR-8.2).
    @discardableResult
    public func download(_ pack: RemotePack) async throws -> URL {
        let data = try await fetch(pack.url)
        try verify(data)                       // verify BEFORE install
        let dest = installURL(for: pack.id)
        try ensurePacksDirectory()
        try data.write(to: dest, options: .atomic)
        return dest
    }

    /// Re-download a pack (FR-8.3): delete the local copy, then download fresh.
    @discardableResult
    public func redownload(_ pack: RemotePack) async throws -> URL {
        try? delete(packID: pack.id)
        return try await download(pack)
    }

    /// Verify raw pack bytes with the full signed chain, without installing. Throws on any failure.
    public func verify(_ data: Data) throws {
        let temp = fileManager.temporaryDirectory
            .appendingPathComponent("zhuwen-verify-\(UUID().uuidString).zpack")
        defer { try? fileManager.removeItem(at: temp) }
        do {
            try data.write(to: temp)
            let archive = try ZipArchive(url: temp)
            let dbTemp = fileManager.temporaryDirectory
                .appendingPathComponent("zhuwen-verify-\(UUID().uuidString).sqlite")
            defer { try? fileManager.removeItem(at: dbTemp) }
            _ = try PackVerifier.verify(
                archive: archive,
                publicKey: publicKey,
                knownLexiconVersions: knownLexiconVersions,
                contentDatabase: { bytes in
                    try bytes.write(to: dbTemp)
                    return try ContentDatabase(path: dbTemp.path)
                })
        } catch let e as PackVerifier.VerifyError {
            throw ClientError.verificationFailed(String(describing: e))
        }
    }

    // MARK: - Pack manager (FR-8.3)

    /// The installed packs on disk (id + byte size), sorted by id.
    public func installedPacks() -> [InstalledPack] {
        guard let names = try? fileManager.contentsOfDirectory(atPath: packsDirectory.path) else {
            return []
        }
        return names.filter { $0.hasSuffix(".zpack") }.map { name in
            let url = packsDirectory.appendingPathComponent(name)
            let size = (try? fileManager.attributesOfItem(atPath: url.path)[.size] as? Int) ?? 0
            let id = String(name.dropLast(".zpack".count))
            return InstalledPack(id: id, url: url, sizeBytes: size ?? 0)
        }
        .sorted { $0.id < $1.id }
    }

    public func isInstalled(_ packID: String) -> Bool {
        fileManager.fileExists(atPath: installURL(for: packID).path)
    }

    /// Byte size of an installed pack, or 0 if absent.
    public func sizeOnDisk(_ packID: String) -> Int {
        let url = installURL(for: packID)
        let size = (try? fileManager.attributesOfItem(atPath: url.path)[.size] as? Int) ?? 0
        return size ?? 0
    }

    public func delete(packID: String) throws {
        let url = installURL(for: packID)
        if fileManager.fileExists(atPath: url.path) {
            try fileManager.removeItem(at: url)
        }
    }

    public func installURL(for packID: String) -> URL {
        packsDirectory.appendingPathComponent("\(packID).zpack")
    }

    // MARK: - Internals

    private func ensurePacksDirectory() throws {
        if !fileManager.fileExists(atPath: packsDirectory.path) {
            try fileManager.createDirectory(at: packsDirectory, withIntermediateDirectories: true)
        }
    }

    /// Enforce https (I2/ATS) and delegate to the (anonymous) fetcher.
    private func fetch(_ url: URL) async throws -> Data {
        guard url.scheme?.lowercased() == "https" || url.isFileURL else {
            throw ClientError.insecureURL(url.absoluteString)
        }
        return try await fetcher.fetch(url)
    }
}

/// A remote pack entry in the CDN catalog (FR-8.2).
public struct RemotePack: Codable, Equatable, Identifiable {
    public let id: String
    public let band: String       // A1 / A2 / B1 …
    public let semver: String
    public let sizeBytes: Int
    public let url: URL

    public init(id: String, band: String, semver: String, sizeBytes: Int, url: URL) {
        self.id = id; self.band = band; self.semver = semver
        self.sizeBytes = sizeBytes; self.url = url
    }
}

/// The signed catalog of downloadable packs served from the CDN.
public struct PackCatalog: Codable, Equatable {
    public let packs: [RemotePack]
    public init(packs: [RemotePack]) { self.packs = packs }
}

/// An installed pack on disk (pack-manager row, FR-8.3).
public struct InstalledPack: Equatable, Identifiable {
    public let id: String
    public let url: URL
    public let sizeBytes: Int

    public init(id: String, url: URL, sizeBytes: Int) {
        self.id = id; self.url = url; self.sizeBytes = sizeBytes
    }
}

/// Abstracts pack byte retrieval so the client is testable without sockets. The only production
/// conformer, `URLSessionPackFetcher`, is the sole `URLSession` call site in the app (I2).
public protocol PackFetcher: Sendable {
    func fetch(_ url: URL) async throws -> Data
}

/// Anonymous CDN GET over an **ephemeral** `URLSession` (no cookies, cache, or credentials → no
/// identifiers leave the device, FR-8.3 / I2). This is the *only* place `URLSession` is used.
public struct URLSessionPackFetcher: PackFetcher {
    public init() {}

    public func fetch(_ url: URL) async throws -> Data {
        // Local files (tests / bundled catalogs) bypass the network entirely.
        if url.isFileURL { return try Data(contentsOf: url) }

        let config = URLSessionConfiguration.ephemeral
        config.httpCookieStorage = nil
        config.httpCookieAcceptPolicy = .never
        config.urlCredentialStorage = nil
        config.requestCachePolicy = .reloadIgnoringLocalCacheData
        let session = URLSession(configuration: config)
        defer { session.finishTasksAndInvalidate() }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.httpShouldHandleCookies = false

        let (data, response) = try await session.data(for: request)
        if let http = response as? HTTPURLResponse, !(200...299).contains(http.statusCode) {
            throw PackClient.ClientError.badResponse
        }
        return data
    }
}
