import Foundation
import SwiftUI
import ZhuwenPacks

/// PackManagerModel backs the pack-manager UI (M13, FR-8.3): it lists installed packs with sizes and
/// deletes / re-downloads them via the CDN `PackClient`. Downloads are anonymous and verified before
/// install (the `PackClient` guarantees). Tolerates the no-network host: `client` may be nil, in which
/// case only the vendored/embedded packs show and remote actions are disabled.
@MainActor
public final class PackManagerModel: ObservableObject {
    @Published public private(set) var installed: [InstalledPack] = []
    @Published public private(set) var available: [RemotePack] = []
    @Published public private(set) var busyPackID: String?
    @Published public var lastError: String?

    private let client: PackClient?

    public init(client: PackClient? = nil) {
        self.client = client
        refreshInstalled()
    }

    public var canDownload: Bool { client != nil }

    public func refreshInstalled() {
        installed = client?.installedPacks() ?? []
    }

    public func loadCatalog() async {
        guard let client else { return }
        do { available = try await client.catalog().packs; lastError = nil }
        catch { lastError = String(describing: error) }
    }

    public func download(_ pack: RemotePack) async {
        guard let client else { return }
        busyPackID = pack.id
        do { try await client.download(pack); lastError = nil }
        catch { lastError = String(describing: error) }
        busyPackID = nil
        refreshInstalled()
    }

    public func redownload(_ pack: RemotePack) async {
        guard let client else { return }
        busyPackID = pack.id
        do { try await client.redownload(pack); lastError = nil }
        catch { lastError = String(describing: error) }
        busyPackID = nil
        refreshInstalled()
    }

    public func delete(_ pack: InstalledPack) {
        do { try client?.delete(packID: pack.id) }
        catch { lastError = String(describing: error) }
        refreshInstalled()
    }

    /// Human-readable size for a pack row.
    public static func sizeLabel(_ bytes: Int) -> String {
        ByteCountFormatter.string(fromByteCount: Int64(bytes), countStyle: .file)
    }
}
