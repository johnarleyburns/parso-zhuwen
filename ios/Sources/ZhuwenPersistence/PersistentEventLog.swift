import Foundation
import SwiftData
import ZhuwenCore

/// A durable, append-only event log backed by SwiftData (I5). It presents the same surface as the
/// in-memory `ZhuwenCore.EventLog` — `append`, `append(contentsOf:)`, `count`, `events` — and
/// conforms to `EventSink` so `LearnerModel` can mirror its in-memory log into it transparently.
///
/// The store is the source of truth: `KnownWordModel` is never persisted as state, only rebuilt by
/// replaying `events` at launch. Rows are only ever appended; the sole non-append path is
/// `replaceAll`, the whole-log erase/import reset (FR-10.3), which is not a per-event mutation.
///
/// Not `Sendable`: like any `ModelContext`, an instance is bound to the thread/actor that created
/// it. The app creates it on the main actor; tests use it on the test thread.
public final class PersistentEventLog: EventSink {
    private let context: ModelContext
    private var nextSeq: Int

    /// Open the log over an existing container (e.g. the app's shared `ModelContainer`).
    public init(container: ModelContainer) {
        self.context = ModelContext(container)
        self.nextSeq = ((try? Self.maxSeq(in: context)) ?? nil).map { $0 + 1 } ?? 0
    }

    /// Convenience: open (or create) a file-backed store at `url`.
    public convenience init(url: URL) throws {
        self.init(container: try LearnerStore.container(url: url))
    }

    // MARK: - Read (replay source)

    public var count: Int {
        (try? context.fetchCount(FetchDescriptor<EventRecord>())) ?? 0
    }

    /// The full ordered event history, oldest first (by append `seq`). This is the replay input.
    public var events: [Event] {
        let descriptor = FetchDescriptor<EventRecord>(sortBy: [SortDescriptor(\.seq, order: .forward)])
        let rows = (try? context.fetch(descriptor)) ?? []
        return rows.compactMap(\.event)
    }

    /// Rebuild the known-word model by replaying the persisted log over a placement seed (I5).
    ///
    /// Fast path: if a `ProjectionCheckpoint` built over the *same* seed exists, start from it and
    /// fold only the events after its `lastSeq` (MC-1.5 — keeps cold launch inside NFR-1 for large
    /// logs). The checkpoint is disposable: a missing/corrupt/seed-mismatched cache falls back to a
    /// full replay, and the append-only log stays the source of truth.
    public func projectedModel(seed: [Int: Double] = [:]) -> KnownWordModel {
        if let cp = checkpoint(), cp.seed == seed {
            var model = cp.model
            for e in events(after: cp.lastSeq) { model.apply(e) }
            return model
        }
        return KnownWordModel.project(events, seed: seed)
    }

    /// Persist a projection cache at the current log head (over `seed`). Call sparingly (e.g. on
    /// background/quit or after N new events); it never affects correctness, only launch speed.
    @discardableResult
    public func saveCheckpoint(seed: [Int: Double] = [:]) -> Bool {
        guard let lastSeq = (try? Self.maxSeq(in: context)) ?? nil else { return false }
        let model = KnownWordModel.project(events, seed: seed)
        guard let modelData = try? JSONEncoder().encode(model),
              let seedData = try? JSONEncoder().encode(seed) else { return false }
        try? context.delete(model: ProjectionCheckpoint.self)
        context.insert(ProjectionCheckpoint(lastSeq: lastSeq, modelData: modelData, seedData: seedData))
        try? context.save()
        return true
    }

    // MARK: - Append-only writes (EventSink)

    public func append(_ event: Event) {
        context.insert(EventRecord(seq: nextSeq, event: event))
        nextSeq += 1
        try? context.save()
    }

    public func append(contentsOf newEvents: [Event]) {
        for e in newEvents {
            context.insert(EventRecord(seq: nextSeq, event: e))
            nextSeq += 1
        }
        try? context.save()
    }

    /// Whole-log reset for erase/import (FR-10.3). Deletes every event row and re-appends the new
    /// history from `seq` 0. This is the only path that removes rows, and it rewrites the *entire*
    /// log rather than editing history — preserving the "no per-event mutation" guarantee (I5).
    public func replaceAll(_ events: [Event]) {
        try? context.delete(model: EventRecord.self)
        try? context.delete(model: ProjectionCheckpoint.self)   // cache is invalid after a reset
        nextSeq = 0
        append(contentsOf: events)
    }

    // MARK: - Export (FR-10.3 groundwork)

    /// The raw event log as a portable archive (export payload; UI adds the placement seed).
    public func exportArchive(seed: [Int: Double] = [:], at now: Date = Date()) -> LearnerArchive {
        LearnerArchive(events: events, seed: seed, exportedAt: now)
    }

    /// Pretty JSON bytes of the raw event log (+ seed), for a share sheet / Files export.
    public func exportJSON(seed: [Int: Double] = [:], at now: Date = Date()) throws -> Data {
        try exportArchive(seed: seed, at: now).encoded()
    }

    // MARK: - Internals

    private static func maxSeq(in context: ModelContext) throws -> Int? {
        var d = FetchDescriptor<EventRecord>(sortBy: [SortDescriptor(\.seq, order: .reverse)])
        d.fetchLimit = 1
        return try context.fetch(d).first?.seq
    }

    /// Events with `seq > after`, in order (the checkpoint tail).
    private func events(after seq: Int) -> [Event] {
        let d = FetchDescriptor<EventRecord>(
            predicate: #Predicate { $0.seq > seq },
            sortBy: [SortDescriptor(\.seq, order: .forward)])
        return ((try? context.fetch(d)) ?? []).compactMap(\.event)
    }

    /// The decoded projection checkpoint, or nil if absent/corrupt (disposable).
    private func checkpoint() -> (model: KnownWordModel, seed: [Int: Double], lastSeq: Int)? {
        var d = FetchDescriptor<ProjectionCheckpoint>()
        d.fetchLimit = 1
        guard let row = try? context.fetch(d).first,
              let model = try? JSONDecoder().decode(KnownWordModel.self, from: row.modelData),
              let seed = try? JSONDecoder().decode([Int: Double].self, from: row.seedData)
        else { return nil }
        return (model, seed, row.lastSeq)
    }
}
