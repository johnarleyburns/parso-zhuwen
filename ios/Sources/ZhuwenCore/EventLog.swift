import Foundation

/// The kinds of learner event that feed the known-word model (00 §2 I5, FR-2.2).
public enum EventKind: String, Codable, Equatable {
    case exposure      // word seen in a story and NOT tapped → weak evidence of knowing
    case lookup        // dictionary lookup / tap → evidence of not knowing
    case reviewGrade   // FSRS review grade (payload: grade 0…3)
    case markKnown     // explicit "I know this"
    case markUnknown   // explicit "I don't know this"
    case comprehension // comprehension-question outcome on a sentence (payload: correct)
    case listen        // a listening pass completed (payload: blind) — listening-skill (CP-07)
}

/// One append-only learner event (00 §9 `Event`). The event log is the source of truth;
/// the known-word model is a pure, replayable projection of it (I5).
public struct Event: Equatable, Codable {
    public let ts: Date
    public let kind: EventKind
    public let wordID: Int?
    public let storyID: String?
    public let grade: Int?    // reviewGrade payload: 0 again · 1 hard · 2 good · 3 easy
    public let correct: Bool? // comprehension payload
    public let blind: Bool?   // listen payload: blind-listening pass (FR-5.2)

    public init(ts: Date, kind: EventKind, wordID: Int? = nil, storyID: String? = nil,
                grade: Int? = nil, correct: Bool? = nil, blind: Bool? = nil) {
        self.ts = ts
        self.kind = kind
        self.wordID = wordID
        self.storyID = storyID
        self.grade = grade
        self.correct = correct
        self.blind = blind
    }

    // Convenience builders.
    public static func exposure(_ wordID: Int, storyID: String? = nil, at ts: Date) -> Event {
        Event(ts: ts, kind: .exposure, wordID: wordID, storyID: storyID)
    }
    public static func lookup(_ wordID: Int, storyID: String? = nil, at ts: Date) -> Event {
        Event(ts: ts, kind: .lookup, wordID: wordID, storyID: storyID)
    }
    public static func reviewGrade(_ wordID: Int, grade: Int, at ts: Date) -> Event {
        Event(ts: ts, kind: .reviewGrade, wordID: wordID, grade: grade)
    }
    public static func markKnown(_ wordID: Int, at ts: Date) -> Event {
        Event(ts: ts, kind: .markKnown, wordID: wordID)
    }
    public static func markUnknown(_ wordID: Int, at ts: Date) -> Event {
        Event(ts: ts, kind: .markUnknown, wordID: wordID)
    }
    public static func comprehension(_ wordID: Int, correct: Bool, storyID: String? = nil, at ts: Date) -> Event {
        Event(ts: ts, kind: .comprehension, wordID: wordID, storyID: storyID, correct: correct)
    }
    /// A completed listening pass over a story (FR-5.1/5.2). Story-level (wordID nil) so the
    /// reading-oriented projection ignores it; CP-07 folds it into the listening-skill estimate.
    public static func listen(storyID: String, blind: Bool, at ts: Date) -> Event {
        Event(ts: ts, kind: .listen, storyID: storyID, blind: blind)
    }
}

/// A sink for appended events (I5). Lets `LearnerModel` mirror its in-memory log into a
/// durable store (`ZhuwenPersistence.PersistentEventLog`) without `ZhuwenUI` depending on
/// SwiftData. There is no per-event delete/mutate; `replaceAll` exists only for erase/import
/// (FR-10.3), which rewrites the whole log rather than editing history.
public protocol EventSink: AnyObject {
    /// Append one event (append-only; never mutates prior events).
    func append(_ event: Event)
    /// Replace the entire log (erase → empty, or import → a new archive's events). The store
    /// still only ever appends internally; this is the sole whole-log reset path.
    func replaceAll(_ events: [Event])
}

/// An append-only event log (I5). There is deliberately no delete/mutate API: the model is
/// rebuilt by replaying `events`, and persistence (SwiftData) simply stores the same list.
public final class EventLog: EventSink {
    public private(set) var events: [Event]

    public init(_ events: [Event] = []) { self.events = events }

    public var count: Int { events.count }

    public func append(_ event: Event) { events.append(event) }

    public func append(contentsOf newEvents: [Event]) { events.append(contentsOf: newEvents) }

    /// Whole-log reset (erase/import, FR-10.3). Not a per-event mutation.
    public func replaceAll(_ events: [Event]) { self.events = events }

    /// Events touching a given word, in order.
    public func events(for wordID: Int) -> [Event] { events.filter { $0.wordID == wordID } }
}
