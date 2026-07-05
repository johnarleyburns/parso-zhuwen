import Foundation

/// LearnerArchive is the on-device learner state serialized to JSON for **export everything** and
/// re-imported to reproduce that state exactly (00 §4 FR-10.3). Because the `KnownWordModel` is a
/// pure projection of the ordered event log + placement seed (I5), the archive is just that log and
/// seed — nothing derived is stored, so `export → erase → import` is the identity on learner state.
///
/// There is no server and no account: this file is the *only* representation of a learner's history
/// (I2). Erase deletes the log; import replaces it.
public struct LearnerArchive: Codable, Equatable {
    /// Bumped only on a breaking archive-shape change; importers reject unknown majors.
    public static let currentSchemaVersion = 1

    public let schemaVersion: Int
    public let exportedAt: Date
    public let events: [Event]
    /// Placement seed priors (word id → P(known) prior), FR-1.5.
    public let seed: [Int: Double]

    public init(events: [Event], seed: [Int: Double] = [:],
                exportedAt: Date = Date(), schemaVersion: Int = LearnerArchive.currentSchemaVersion) {
        self.schemaVersion = schemaVersion
        self.exportedAt = exportedAt
        self.events = events
        self.seed = seed
    }

    public enum ArchiveError: Error, Equatable {
        case unsupportedSchema(Int)
        case malformed
    }

    // MARK: - Encode / decode (FR-10.3 "export everything (JSON)")

    private static func encoder() -> JSONEncoder {
        let e = JSONEncoder()
        e.outputFormatting = [.prettyPrinted, .sortedKeys]
        e.dateEncodingStrategy = .iso8601
        return e
    }

    private static func decoder() -> JSONDecoder {
        let d = JSONDecoder()
        d.dateDecodingStrategy = .iso8601
        return d
    }

    /// Serialize to pretty, stable JSON bytes (for a share sheet / Files export).
    public func encoded() throws -> Data {
        try Self.encoder().encode(self)
    }

    /// Parse an exported archive, rejecting a future major schema.
    public static func decoded(from data: Data) throws -> LearnerArchive {
        guard let archive = try? decoder().decode(LearnerArchive.self, from: data) else {
            throw ArchiveError.malformed
        }
        guard archive.schemaVersion <= currentSchemaVersion else {
            throw ArchiveError.unsupportedSchema(archive.schemaVersion)
        }
        return archive
    }

    // MARK: - Projection

    /// Reproduce the known-word model this archive represents (the round-trip target).
    public func projectedModel() -> KnownWordModel {
        KnownWordModel.project(events, seed: seed)
    }
}
