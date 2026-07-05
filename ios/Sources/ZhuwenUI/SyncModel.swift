import Foundation
import ZhuwenCore

#if canImport(CloudKit)
import CloudKit
#endif

/// Pushes/pulls the learner archive to a backing store. The only production conformer syncs to the
/// user's **private** CloudKit database (00 §4 FR-10.2 / I2: learner state only, never content), and
/// only when the learner has explicitly enabled it. The default is a no-op so the app is fully
/// functional — and testable — with sync off.
public protocol LearnerSyncEngine: AnyObject {
    var isAvailable: Bool { get }
    func push(_ archive: LearnerArchive) async throws
    func pull() async throws -> LearnerArchive?
}

/// The default engine: does nothing (sync off, the FR-10.2 default). Keeps the app network-free.
public final class NoOpSyncEngine: LearnerSyncEngine {
    public init() {}
    public var isAvailable: Bool { false }
    public func push(_ archive: LearnerArchive) async throws {}
    public func pull() async throws -> LearnerArchive? { nil }
}

/// SyncModel exposes the opt-in iCloud sync toggle (FR-10.2) to `SettingsView`. It never runs unless
/// `enabled` is set by the learner, and it moves only the `LearnerArchive` (events + seed) — no
/// content, no identifiers beyond the user's own iCloud account (I2).
@MainActor
public final class SyncModel: ObservableObject {
    @Published public var enabled: Bool
    @Published public private(set) var lastSyncedAt: Date?
    @Published public private(set) var lastError: String?

    private let engine: LearnerSyncEngine

    public init(enabled: Bool = false, engine: LearnerSyncEngine = SyncModel.makeDefaultEngine()) {
        self.enabled = enabled
        self.engine = engine
    }

    public var isAvailable: Bool { engine.isAvailable }

    public func push(_ archive: LearnerArchive) async {
        guard enabled, engine.isAvailable else { return }
        do { try await engine.push(archive); lastSyncedAt = Date(); lastError = nil }
        catch { lastError = String(describing: error) }
    }

    public func pull() async -> LearnerArchive? {
        guard enabled, engine.isAvailable else { return nil }
        do { let a = try await engine.pull(); lastSyncedAt = Date(); lastError = nil; return a }
        catch { lastError = String(describing: error); return nil }
    }

    public nonisolated static func makeDefaultEngine() -> LearnerSyncEngine {
        #if canImport(CloudKit) && os(iOS)
        if #available(iOS 17.0, *) { return CloudKitSyncEngine() }
        #endif
        return NoOpSyncEngine()
    }
}

#if canImport(CloudKit) && os(iOS)
import CloudKit

/// Syncs the learner archive to the user's **private** CloudKit database (FR-10.2). One record holds
/// the exported JSON; push overwrites, pull reads the latest. Availability-guarded so the host build
/// (where CloudKit entitlements are absent) is unaffected. Production conflict policy is CP-10.
@available(iOS 17.0, *)
public final class CloudKitSyncEngine: LearnerSyncEngine {
    private let recordType = "LearnerArchive"
    private let recordID = CKRecord.ID(recordName: "learner-archive")
    private let database: CKDatabase

    public init(container: CKContainer = .default()) {
        self.database = container.privateCloudDatabase
    }

    public var isAvailable: Bool { true }

    public func push(_ archive: LearnerArchive) async throws {
        let record = CKRecord(recordType: recordType, recordID: recordID)
        record["json"] = try archive.encoded() as CKRecordValue
        record["exportedAt"] = archive.exportedAt as CKRecordValue
        _ = try await database.modifyRecords(saving: [record], deleting: [],
                                             savePolicy: .allKeys).saveResults
    }

    public func pull() async throws -> LearnerArchive? {
        do {
            let record = try await database.record(for: recordID)
            guard let data = record["json"] as? Data else { return nil }
            return try LearnerArchive.decoded(from: data)
        } catch let ckError as CKError where ckError.code == .unknownItem {
            return nil
        }
    }
}
#endif
