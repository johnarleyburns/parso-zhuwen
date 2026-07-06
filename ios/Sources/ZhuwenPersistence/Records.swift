import Foundation
import SwiftData
import ZhuwenCore

// The on-device learner store (00 §9, "Learner store (SwiftData)"). These are the durable
// @Model rows registered in the app's ModelContainer. The append-only `EventRecord` log is the
// source of truth (I5); every other row is either a projection cache or app bookkeeping and can
// be rebuilt or refetched. Nothing here is ever mutated in a way that rewrites event history.

/// One append-only learner event row (00 §9 `Event`). `seq` is a monotonic append counter that
/// preserves total order across relaunches (SwiftData fetches are otherwise unordered), so replay
/// reproduces the exact `KnownWordModel`. Rows are never updated or deleted except by a whole-log
/// erase/import reset (FR-10.3) — never a per-event mutation (I5).
@Model
public final class EventRecord {
    @Attribute(.unique) public var seq: Int
    public var ts: Date
    public var kind: String
    public var wordID: Int?
    public var storyID: String?
    public var grade: Int?
    public var correct: Bool?
    public var blind: Bool?

    public init(seq: Int, ts: Date, kind: String, wordID: Int? = nil, storyID: String? = nil,
                grade: Int? = nil, correct: Bool? = nil, blind: Bool? = nil) {
        self.seq = seq
        self.ts = ts
        self.kind = kind
        self.wordID = wordID
        self.storyID = storyID
        self.grade = grade
        self.correct = correct
        self.blind = blind
    }

    public convenience init(seq: Int, event: Event) {
        self.init(seq: seq, ts: event.ts, kind: event.kind.rawValue, wordID: event.wordID,
                  storyID: event.storyID, grade: event.grade, correct: event.correct,
                  blind: event.blind)
    }

    /// Rebuild the value-type `Event` this row persists. Unknown kinds are dropped by the caller.
    public var event: Event? {
        guard let k = EventKind(rawValue: kind) else { return nil }
        return Event(ts: ts, kind: k, wordID: wordID, storyID: storyID,
                     grade: grade, correct: correct, blind: blind)
    }
}

/// Per-story reading progress (00 §9 `StoryProgress`). Bookkeeping projection: seal/blind facts
/// also live in the event log; this row is a convenience index the UI reads without replaying.
@Model
public final class StoryProgressRecord {
    @Attribute(.unique) public var storyID: String
    public var position: Int
    public var completedAt: Date?
    public var sealEarned: Bool
    public var listenedBlind: Bool

    public init(storyID: String, position: Int = 0, completedAt: Date? = nil,
                sealEarned: Bool = false, listenedBlind: Bool = false) {
        self.storyID = storyID
        self.position = position
        self.completedAt = completedAt
        self.sealEarned = sealEarned
        self.listenedBlind = listenedBlind
    }
}

/// A placement run's fitted result (00 §9 `PlacementResult`, FR-1.2). `curveParams` is JSON.
@Model
public final class PlacementResultRecord {
    public var ts: Date
    public var curveParams: Data
    public var cefrEstimate: String
    public var hskEstimate: Int
    public var ci: Double

    public init(ts: Date, curveParams: Data, cefrEstimate: String, hskEstimate: Int, ci: Double) {
        self.ts = ts
        self.curveParams = curveParams
        self.cefrEstimate = cefrEstimate
        self.hskEstimate = hskEstimate
        self.ci = ci
    }
}

/// Single-row app preferences (00 §9 `Prefs`, FR-10.1). Stored as a JSON blob so the settings
/// shape can evolve without a schema migration; `id` pins the singleton.
@Model
public final class PrefsRecord {
    @Attribute(.unique) public var id: Int
    public var data: Data

    public init(id: Int = 0, data: Data = Data()) {
        self.id = id
        self.data = data
    }
}

/// An installed pack (00 §9 `PackRecord`, FR-9/FR-10). Refetchable from the CDN, so disposable.
@Model
public final class PackRecord {
    @Attribute(.unique) public var packID: String
    public var version: String
    public var installedAt: Date
    public var bytes: Int

    public init(packID: String, version: String, installedAt: Date = Date(), bytes: Int = 0) {
        self.packID = packID
        self.version = version
        self.installedAt = installedAt
        self.bytes = bytes
    }
}

/// A disposable projection cache (MC-1.5). Stores the `KnownWordModel` folded up to `lastSeq`
/// (over `seed`) so launch replays only the tail of the log instead of all of it, keeping cold
/// launch inside the NFR-1 budget for large histories. It is **disposable**: if it is missing,
/// corrupt, or built over a different seed, the log is replayed from scratch — the append-only
/// `EventRecord` log remains the sole source of truth (I5). Singleton row (`id == 0`).
@Model
public final class ProjectionCheckpoint {
    @Attribute(.unique) public var id: Int
    public var lastSeq: Int
    public var modelData: Data
    public var seedData: Data

    public init(id: Int = 0, lastSeq: Int, modelData: Data, seedData: Data) {
        self.id = id
        self.lastSeq = lastSeq
        self.modelData = modelData
        self.seedData = seedData
    }
}

/// The full learner-store schema, in one place so the app and tests build identical containers.
public enum LearnerStore {
    public static let models: [any PersistentModel.Type] = [
        EventRecord.self,
        StoryProgressRecord.self,
        PlacementResultRecord.self,
        PrefsRecord.self,
        PackRecord.self,
        ProjectionCheckpoint.self,
    ]

    public static let schema = Schema(models)

    /// A container backed by a file at `url` (durable across launches). Used by `@main` and the
    /// launch-replay test, which re-opens the same URL to simulate a relaunch.
    public static func container(url: URL) throws -> ModelContainer {
        try ModelContainer(for: schema,
                           configurations: ModelConfiguration(schema: schema, url: url))
    }

    /// An in-memory container (tests that don't need durability).
    public static func inMemoryContainer() throws -> ModelContainer {
        try ModelContainer(for: schema,
                           configurations: ModelConfiguration(schema: schema, isStoredInMemoryOnly: true))
    }
}
