import Foundation
import SwiftData
import ZhuwenCore

/// A durable `PlacementStore` (00 §9 `PlacementResult`, FR-1.4 first-run gating) backed by
/// SwiftData. It persists the compact `PlacementSnapshot` so the app knows onboarding is done
/// (never re-presenting placement), can re-seed `KnownWordModel`, and can route beginners into
/// Foundations. Kept in `ZhuwenPersistence` so `ZhuwenUI` never imports SwiftData (the same seam
/// as `PersistentEventLog`/`EventSink`).
///
/// The snapshot JSON rides in `PlacementResultRecord.curveParams`; `cefrEstimate`/`hskEstimate`
/// mirror it for at-a-glance queries. Re-running placement inserts a new row and `load()` returns
/// the latest (by `ts`).
public final class PersistentPlacementStore: PlacementStore {
    private let context: ModelContext

    public init(container: ModelContainer) {
        self.context = ModelContext(container)
    }

    public convenience init(url: URL) throws {
        self.init(container: try LearnerStore.container(url: url))
    }

    public func load() -> PlacementSnapshot? {
        var d = FetchDescriptor<PlacementResultRecord>(sortBy: [SortDescriptor(\.ts, order: .reverse)])
        d.fetchLimit = 1
        guard let row = try? context.fetch(d).first,
              let snapshot = try? JSONDecoder().decode(PlacementSnapshot.self, from: row.curveParams)
        else { return nil }
        return snapshot
    }

    public func save(_ snapshot: PlacementSnapshot) {
        guard let data = try? JSONEncoder().encode(snapshot) else { return }
        context.insert(PlacementResultRecord(
            ts: Date(), curveParams: data, cefrEstimate: snapshot.cefr,
            hskEstimate: snapshot.hskLevel, ci: 0))
        try? context.save()
    }

    public func clear() {
        try? context.delete(model: PlacementResultRecord.self)
        try? context.save()
    }
}
