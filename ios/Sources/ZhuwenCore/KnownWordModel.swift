import Foundation

/// The lifecycle state of a lexicon word for the learner (00 §9, FR-2.1).
public enum LearningState: String, Equatable {
    case unseen, introduced, learning, known, mastered
}

/// The projected state of one word. A pure function of the ordered events that touched it
/// (plus any placement seed), so the whole model is replayable (I5).
public struct WordState: Equatable {
    public var wordID: Int
    public var state: LearningState
    public var pKnown: Double
    public var exposures: Int
    public var lookups: Int
    public var goodGrades: Int
    public var firstSeen: Date?
    public var lastSeen: Date?
    /// FSRS memory (FR-2.1). `nil` until the word's first `.reviewGrade`; reconstructed by folding
    /// the review events, so it stays a pure projection of the log (I5).
    public var fsrs: FSRSCard?

    public init(wordID: Int, state: LearningState = .unseen, pKnown: Double = 0,
                exposures: Int = 0, lookups: Int = 0, goodGrades: Int = 0,
                firstSeen: Date? = nil, lastSeen: Date? = nil, fsrs: FSRSCard? = nil) {
        self.wordID = wordID
        self.state = state
        self.pKnown = pKnown
        self.exposures = exposures
        self.lookups = lookups
        self.goodGrades = goodGrades
        self.firstSeen = firstSeen
        self.lastSeen = lastSeen
        self.fsrs = fsrs
    }

    /// Whether the card is due for review at `now` (has been graded at least once, FR-7.1).
    public func isDue(at now: Date) -> Bool {
        guard let fsrs else { return false }
        return fsrs.due <= now
    }
}

/// KnownWordModel is a **pure projection** of the append-only event log (I5). `project`
/// folds the events; `applying` folds one more. Rebuilding from the same events always
/// yields the same states — the replay guarantee tested in `KnownWordModelTests`.
///
/// The P(known) update rules are a documented CP-04 heuristic (v1): exposure is weak
/// positive evidence, a lookup is strong negative evidence, explicit marks are absolute,
/// grades and comprehension nudge proportionally. FSRS memory parameters and placement's
/// logistic seed arrive in CP-07 / CP-05 and feed the same model.
public struct KnownWordModel: Equatable {
    /// Effective-known threshold for the coverage gate (FR-2.3).
    public static let knownThreshold = 0.8
    public static let masteredThreshold = 0.95

    /// The FSRS scheduler folded into `.reviewGrade` events (default weights; FR-7.1).
    static let scheduler = FSRSScheduler.default

    public private(set) var states: [Int: WordState]

    public init(states: [Int: WordState] = [:]) { self.states = states }

    /// Projects a fresh model from an event list, optionally over a placement seed
    /// (`wordID → prior P(known)`; CP-05 will supply it).
    public static func project(_ events: [Event], seed: [Int: Double] = [:]) -> KnownWordModel {
        var model = KnownWordModel(states: seededStates(seed))
        for e in events { model.apply(e) }
        return model
    }

    /// Builds the model from a placement seed and the event log (FR-1.2 seeding, FR-1.5
    /// re-placement). The seed is the P(known) prior the events fold onto; re-running placement
    /// merges seeds (`PlacementSeed.merged`) and re-projects — never mutating the log (I5).
    public static func seeded(_ seed: PlacementSeed, events: [Event] = []) -> KnownWordModel {
        project(events, seed: seed.priors)
    }

    /// Returns a copy with one more event folded in (value semantics keep it side-effect free).
    public func applying(_ event: Event) -> KnownWordModel {
        var copy = self
        copy.apply(event)
        return copy
    }

    public mutating func apply(_ event: Event) {
        guard let id = event.wordID else { return }
        var s = states[id] ?? WordState(wordID: id)
        if s.firstSeen == nil { s.firstSeen = event.ts }
        s.lastSeen = event.ts

        switch event.kind {
        case .exposure:
            s.exposures += 1
            s.pKnown = s.pKnown + 0.05 * (1 - s.pKnown)
        case .lookup:
            s.lookups += 1
            s.pKnown = s.pKnown * 0.5
        case .markKnown:
            s.pKnown = 1.0
        case .markUnknown:
            s.pKnown = 0.0
        case .reviewGrade:
            let g = event.grade ?? 0
            // Advance the FSRS memory (FR-7.1/7.3) and nudge the P(known) heuristic (FR-2.2).
            let rating = Rating(rawValue: Swift.min(3, Swift.max(0, g))) ?? .again
            s.fsrs = Self.scheduler.review(s.fsrs, rating: rating, at: event.ts)
            if g >= 2 {
                s.goodGrades += 1
                s.pKnown = s.pKnown + 0.2 * (1 - s.pKnown)
            } else {
                s.pKnown = s.pKnown * 0.6
            }
        case .comprehension:
            if event.correct == true {
                s.pKnown = s.pKnown + 0.1 * (1 - s.pKnown)
            } else {
                s.pKnown = s.pKnown * 0.8
            }
        case .listen:
            // Listening evidence is tracked separately from reading (FR-5.2). CP-07's
            // both-skill estimate folds `.listen` events; the reading-oriented P(known)
            // projection deliberately leaves them untouched. (Story-level listens carry no
            // wordID and are filtered by the guard above; this keeps the switch exhaustive.)
            break
        }
        s.pKnown = Swift.min(1, Swift.max(0, s.pKnown))
        s.state = Self.deriveState(pKnown: s.pKnown, hasEvidence: true, goodGrades: s.goodGrades)
        states[id] = s
    }

    /// The word's state, or a default `unseen` entry if never touched.
    public func state(for id: Int) -> WordState { states[id] ?? WordState(wordID: id) }

    public func pKnown(_ id: Int) -> Double { states[id]?.pKnown ?? 0 }

    /// Effective known set for the coverage gate (FR-2.3): every word with P(known) ≥ 0.8,
    /// plus the frontier words currently being consolidated (`learning`).
    public func effectiveKnownSet(frontier: Set<Int> = []) -> Set<Int> {
        var out = Set<Int>()
        for (id, s) in states {
            if s.pKnown >= Self.knownThreshold { out.insert(id) }
            else if s.state == .learning && frontier.contains(id) { out.insert(id) }
        }
        return out
    }

    /// Words currently in `learning` (used by the selector as reading-as-SRS payload, FR-3.2).
    public func learningWords() -> Set<Int> {
        Set(states.compactMap { $0.value.state == .learning ? $0.key : nil })
    }

    /// Word IDs whose FSRS card is due at `now`, soonest-due first (FR-7.1 review queue).
    public func dueWordIDs(at now: Date) -> [Int] {
        states.values
            .filter { $0.fsrs.map { $0.due <= now } ?? false }
            .sorted { ($0.fsrs?.due ?? now) < ($1.fsrs?.due ?? now) }
            .map { $0.wordID }
    }

    // MARK: - Derivation

    static func deriveState(pKnown: Double, hasEvidence: Bool, goodGrades: Int) -> LearningState {
        if !hasEvidence && pKnown == 0 { return .unseen }
        if pKnown >= masteredThreshold && goodGrades >= 2 { return .mastered }
        if pKnown >= knownThreshold { return .known }
        if pKnown >= 0.3 { return .learning }
        return .introduced
    }

    private static func seededStates(_ seed: [Int: Double]) -> [Int: WordState] {
        var out: [Int: WordState] = [:]
        for (id, p) in seed {
            let clamped = Swift.min(1, Swift.max(0, p))
            out[id] = WordState(wordID: id, state: deriveState(pKnown: clamped, hasEvidence: clamped > 0, goodGrades: 0), pKnown: clamped)
        }
        return out
    }
}
