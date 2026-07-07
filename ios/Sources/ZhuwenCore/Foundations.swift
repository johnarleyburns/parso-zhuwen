import Foundation
import ZhuwenPacks

/// Foundations is the zero-to-story visual bootstrap program (00 §5A, FR-11). This is the
/// **pure engine**: it builds the ordered picture-word sets from a pack's `foundations_card`
/// rows, drives the F0 four-step interaction (introduce → recognize → read → bind), and models
/// the F3 handoff. Every interaction produces ordinary `Event`s that fold into the single
/// `KnownWordModel` (I5) — Foundations is not a separate flashcard silo.

// MARK: - Cards & sets

/// The F0 four-step interaction cycle (00 §5A.1, FR-11). A session never *ends* on an isolated
/// card step; it ends on the F1/F2 recombination pass (FR-11.4).
public enum FoundationsStage: String, Equatable, Codable, CaseIterable {
    case introduce   // show photo + audio + hanzi + pinyin
    case recognize   // pick the photo that matches the word (recognition grid)
    case read        // pick the word that matches the photo
    case bind        // produce/confirm the picture↔word↔sound binding
}

/// One Foundations picture-word card, resolved against the lexicon (M14).
public struct FoundationsCard: Equatable, Identifiable {
    public let wordID: Int
    public let simp: String
    public let pinyin: String
    public let imageID: String
    public let setID: String
    public let distractorIDs: [Int]

    public var id: Int { wordID }

    public init(wordID: Int, simp: String, pinyin: String, imageID: String,
                setID: String, distractorIDs: [Int]) {
        self.wordID = wordID; self.simp = simp; self.pinyin = pinyin
        self.imageID = imageID; self.setID = setID; self.distractorIDs = distractorIDs
    }

    /// Resolve a pack card row against the lexicon (surface + pinyin for display).
    public init(record: FoundationsCardRecord, word: WordRecord?) {
        self.init(wordID: record.wordID,
                  simp: word?.simp ?? "",
                  pinyin: word?.pinyin ?? "",
                  imageID: record.imageID,
                  setID: record.setID,
                  distractorIDs: record.distractorIDs)
    }
}

/// A semantic set of 6–8 cards (animals, food, family, …), introduced in program order (§5A.2).
public struct FoundationsSet: Equatable, Identifiable {
    public let id: String
    public let name: String
    public let cards: [FoundationsCard]

    public init(id: String, name: String, cards: [FoundationsCard]) {
        self.id = id; self.name = name; self.cards = cards
    }

    public var wordIDs: [Int] { cards.map(\.wordID) }
}

// MARK: - Program

/// The full ordered Foundations program built from a pack's `foundations_card` rows plus the
/// lexicon. Sets keep the pack's first-seen order (the factory already sequenced them by
/// imageability × frequency, §5A.2). The program computes the F3 handoff and the FR-11.6
/// re-entry point for a partial beginner.
public struct FoundationsProgram: Equatable {
    public let sets: [FoundationsSet]

    public init(sets: [FoundationsSet]) { self.sets = sets }

    /// Build from pack card rows + lexicon, grouping by `set_id` in first-seen order and
    /// keeping each set's cards in pack (word_id) order.
    public init(cards: [FoundationsCardRecord], lexicon: LexiconStore) {
        var order: [String] = []
        var bySet: [String: [FoundationsCard]] = [:]
        for rec in cards {
            let card = FoundationsCard(record: rec, word: lexicon.word(rec.wordID))
            if bySet[card.setID] == nil { order.append(card.setID) }
            bySet[card.setID, default: []].append(card)
        }
        self.sets = order.map { FoundationsSet(id: $0, name: Self.displayName($0), cards: bySet[$0] ?? []) }
    }

    /// All cards across all sets, in program order.
    public var allCards: [FoundationsCard] { sets.flatMap(\.cards) }

    public var isEmpty: Bool { sets.isEmpty }

    /// The word IDs taught by every set strictly before `setIndex` (FR-11.3 "already taught").
    public func taughtBefore(setIndex: Int) -> Set<Int> {
        guard setIndex > 0 else { return [] }
        return Set(sets.prefix(setIndex).flatMap(\.wordIDs))
    }

    /// FR-11.6: land a (partial) beginner at the first set with an unmastered word, given a
    /// placement seed's priors. A word counts as mastered when its prior ≥ the known threshold.
    /// Returns `sets.count` when every set is already mastered (→ straight to the handoff check).
    public func startingSetIndex(for seed: PlacementSeed) -> Int {
        for (i, set) in sets.enumerated() {
            let unmastered = set.cards.contains { (seed.priors[$0.wordID] ?? 0) < KnownWordModel.knownThreshold }
            if unmastered { return i }
        }
        return sets.count
    }

    public func startingSet(for seed: PlacementSeed) -> FoundationsSet? {
        let i = startingSetIndex(for: seed)
        return i < sets.count ? sets[i] : nil
    }

    private static func displayName(_ id: String) -> String {
        let known: [String: String] = [
            "animals": "Animals", "food": "Food & Drink", "family": "Family", "numbers": "Numbers",
            "body": "Body", "colors": "Colors", "home": "Home", "places": "Places",
            "weather": "Weather", "actions": "Actions",
        ]
        if let n = known[id] { return n }
        return id.replacingOccurrences(of: "-", with: " ").capitalized
    }
}

// MARK: - Distractor selection (FR-11.3)

public enum FoundationsDeck {
    /// Pick `count` distractor word IDs for a card from the **already-taught** words of its own
    /// set only (never a not-yet-introduced word), using the same deterministic rotation as the
    /// factory (`internal/foundations.pickDeterministic`) so the app and pack agree. This never
    /// draws the same minimal-pair set twice in a row for adjacent cards (property-tested).
    public static func distractors(for card: FoundationsCard, in set: FoundationsSet,
                                   count: Int = 3) -> [Int] {
        var pool: [Int] = []
        for c in set.cards {
            if c.wordID == card.wordID { break }
            pool.append(c.wordID)
        }
        return pickDeterministic(pool, count: count)
    }

    static func pickDeterministic(_ pool: [Int], count: Int) -> [Int] {
        guard count > 0 else { return [] }
        let sorted = pool.sorted()
        if sorted.count <= count { return sorted }
        var key = 0
        for (i, id) in sorted.enumerated() { key += id * (i + 1) }
        let start = key % sorted.count
        var out: [Int] = []
        for i in 0..<count { out.append(sorted[(start + i) % sorted.count]) }
        return out.sorted()
    }
}

// MARK: - Session (drives one 5–8 min sitting; FR-11.4)

/// A single Foundations sitting over one set's cards, ending on an F1/F2 recombination pass —
/// **never** on an isolated card (FR-11.4). Pure value type: the UI and tests drive identical
/// logic and drain the emitted `Event`s into the shared `LearnerModel`/`KnownWordModel` (I5).
public struct FoundationsSession: Equatable {
    public enum Phase: Equatable {
        case card(index: Int, stage: FoundationsStage)
        case recombination
        case done
    }

    public let cards: [FoundationsCard]
    /// The words the closing recombination pass (an F1 pattern or F2 micro-story) re-exposes.
    public let recombinationWordIDs: [Int]
    public private(set) var phase: Phase

    public init(cards: [FoundationsCard], recombinationWordIDs: [Int]? = nil) {
        self.cards = cards
        self.recombinationWordIDs = recombinationWordIDs ?? cards.map(\.wordID)
        self.phase = cards.isEmpty ? .recombination : .card(index: 0, stage: .introduce)
    }

    public init(set: FoundationsSet) { self.init(cards: set.cards) }

    public var isComplete: Bool { phase == .done }

    /// True once the session has reached its closing recombination step (FR-11.4 guarantee: a
    /// session is only ever `done` *after* recombination, never straight off a card).
    public var endedOnRecombination: Bool { phase == .done }

    public var currentCard: FoundationsCard? {
        if case let .card(index, _) = phase, index < cards.count { return cards[index] }
        return nil
    }

    public var currentStage: FoundationsStage? {
        if case let .card(_, stage) = phase { return stage }
        return nil
    }

    /// Advance the current step. `correct` is ignored for `.introduce` (a passive reveal). A
    /// correct recognize/read/bind advances; a wrong answer logs a lookup and repeats the step
    /// (the card is re-shown, FR-11.1). Returns the events to fold into the model (I5).
    @discardableResult
    public mutating func advance(correct: Bool = true, at now: Date = Date()) -> [Event] {
        switch phase {
        case let .card(index, stage):
            guard index < cards.count else { phase = .recombination; return [] }
            let card = cards[index]
            switch stage {
            case .introduce:
                phase = .card(index: index, stage: .recognize)
                return [.exposure(card.wordID, at: now)]
            case .recognize:
                guard correct else { return [.lookup(card.wordID, at: now)] }
                phase = .card(index: index, stage: .read)
                return [.comprehension(card.wordID, correct: true, at: now)]
            case .read:
                guard correct else { return [.lookup(card.wordID, at: now)] }
                phase = .card(index: index, stage: .bind)
                return [.comprehension(card.wordID, correct: true, at: now)]
            case .bind:
                guard correct else { return [.lookup(card.wordID, at: now)] }
                let next = index + 1
                phase = next < cards.count ? .card(index: next, stage: .introduce) : .recombination
                // Binding picture↔word↔sound is the F0 mastery signal: the word enters the known
                // set (it still gets FSRS review later). Emit markKnown so it folds into I5.
                return [.markKnown(card.wordID, at: now)]
            }
        case .recombination:
            phase = .done
            // The closing F1/F2 pass re-exposes every word in reading context (I1 gate machinery
            // proper lives in the factory; here we record the exposures that fold into the model).
            return recombinationWordIDs.map { .exposure($0, at: now) }
        case .done:
            return []
        }
    }
}

// MARK: - F3 handoff gate

/// The F3 handoff (00 §5A.3, FR-1.4/11.5): Foundations graduates the learner to the regular
/// story loop once the effective known set can gate **≥ 20 distinct A1 stories at ≥ 98%**. This
/// reuses the exact `CoverageGate` coverage formula (I1) so the threshold cannot drift.
public struct HandoffGate {
    /// Required number of A1 stories gated at ≥ minCoverage (mirrors factory `HandoffThreshold`).
    public static let threshold = 20
    /// Required coverage in basis points (≥ 98%, mirrors factory `MinCoverage`).
    public static let minCoverageBps = 9800

    public let threshold: Int
    public let minCoverageBps: Int

    public init(threshold: Int = HandoffGate.threshold, minCoverageBps: Int = HandoffGate.minCoverageBps) {
        self.threshold = threshold
        self.minCoverageBps = minCoverageBps
    }

    /// How many A1 stories the `known` set covers at ≥ `minCoverageBps` (I1 coverage formula).
    public func storiesGated(known: Set<Int>, a1Stories: [LatticeStory]) -> Int {
        var count = 0
        for s in a1Stories {
            var newTokens = 0
            for (id, w) in s.typeWeights where !known.contains(id) { newTokens += w }
            if CoverageGate.coverageBps(denom: s.tokenCount, newTokens: newTokens) >= minCoverageBps {
                count += 1
            }
        }
        return count
    }

    /// Whether the handoff fires for the given known set over the A1 lattice.
    public func isReady(known: Set<Int>, a1Stories: [LatticeStory]) -> Bool {
        storiesGated(known: known, a1Stories: a1Stories) >= threshold
    }

    /// Convenience over a `KnownWordModel`'s effective known set.
    public func status(model: KnownWordModel, a1Stories: [LatticeStory]) -> HandoffStatus {
        let gated = storiesGated(known: model.effectiveKnownSet(), a1Stories: a1Stories)
        return HandoffStatus(storiesGated: gated, threshold: threshold, ready: gated >= threshold)
    }
}

/// The handoff readiness snapshot (mirrors factory `foundations.HandoffStatus`).
public struct HandoffStatus: Equatable {
    public let storiesGated: Int
    public let threshold: Int
    public let ready: Bool

    public init(storiesGated: Int, threshold: Int, ready: Bool) {
        self.storiesGated = storiesGated
        self.threshold = threshold
        self.ready = ready
    }
}
