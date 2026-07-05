import Foundation
import ZhuwenPacks

// MARK: - Items

/// One placement checklist item (FR-1.1). A real lexicon word, or a pseudoword foil
/// (`wordID == nil`) used to measure overclaiming.
public struct PlacementItem: Equatable {
    public let surface: String
    public let wordID: Int?
    public let freqRank: Int   // for real words; 0 for foils
    public let hsk3Level: Int  // for real words; 0 for foils

    public init(surface: String, wordID: Int?, freqRank: Int, hsk3Level: Int) {
        self.surface = surface
        self.wordID = wordID
        self.freqRank = freqRank
        self.hsk3Level = hsk3Level
    }

    public var isFoil: Bool { wordID == nil }
}

/// Builds a placement checklist: real words **stratified by HSK-3.0 level × frequency band**
/// plus a fraction of pseudoword foils (FR-1.1), 60–120 items, deterministic per seed.
public struct PlacementItemBuilder {
    public var itemCount: Int
    public var foilFraction: Double
    public var seed: UInt64

    public init(itemCount: Int = 90, foilFraction: Double = 0.2, seed: UInt64 = 0xF00D) {
        self.itemCount = min(120, max(60, itemCount))
        self.foilFraction = min(0.4, max(0.0, foilFraction))
        self.seed = seed
    }

    public func build(lexicon: LexiconStore) -> [PlacementItem] {
        let foilTarget = Int((Double(itemCount) * foilFraction).rounded())
        let realTarget = itemCount - foilTarget

        // Group real words by HSK level; within a level, order by frequency and sample evenly
        // across the frequency range so every band (common…rare) is probed.
        var byLevel: [Int: [WordRecord]] = [:]
        for w in lexicon.words where w.freqRank > 0 {
            byLevel[w.hsk3Level, default: []].append(w)
        }
        let levels = byLevel.keys.sorted()
        var real: [PlacementItem] = []
        if !levels.isEmpty {
            let perLevel = max(1, realTarget / levels.count)
            var remainder = realTarget - perLevel * levels.count
            for lvl in levels {
                var n = perLevel
                if remainder > 0 { n += 1; remainder -= 1 }
                let words = byLevel[lvl]!.sorted { $0.freqRank < $1.freqRank }
                real.append(contentsOf: stride(words, count: n).map {
                    PlacementItem(surface: $0.simp, wordID: $0.id, freqRank: $0.freqRank, hsk3Level: $0.hsk3Level)
                })
            }
        }

        let foils = PseudowordGenerator(lexicon: lexicon)
            .generate(count: foilTarget, seed: seed &+ 1)
            .map { PlacementItem(surface: $0, wordID: nil, freqRank: 0, hsk3Level: 0) }

        var items = real + foils
        var rng = SplitMix64(seed: seed)
        items.shuffle(using: &rng)
        return items
    }

    /// Evenly picks `count` elements across a sorted list (covers the whole frequency range).
    private func stride(_ words: [WordRecord], count: Int) -> [WordRecord] {
        guard count > 0, !words.isEmpty else { return [] }
        if count >= words.count { return words }
        var out: [WordRecord] = []
        out.reserveCapacity(count)
        for i in 0..<count {
            let idx = Int((Double(i) * Double(words.count - 1) / Double(count - 1)).rounded())
            out.append(words[min(words.count - 1, idx)])
        }
        return out
    }
}

// MARK: - Session (drives M1–M3)

/// The placement flow state machine (M1 welcome → M2 word check → M3 result). Pure value type
/// so the UI (`PlacementFlowModel`) and tests drive identical logic.
public struct PlacementSession: Equatable {
    public enum Phase: Equatable { case welcome, wordCheck, result }

    public let items: [PlacementItem]
    public private(set) var answers: [Bool]  // yes(known)/no, in item order
    public private(set) var index: Int
    public private(set) var phase: Phase
    public private(set) var beginner: Bool

    public init(items: [PlacementItem]) {
        self.items = items
        self.answers = []
        self.index = 0
        self.phase = .welcome
        self.beginner = false
    }

    public var currentItem: PlacementItem? { index < items.count ? items[index] : nil }
    public var answeredCount: Int { answers.count }
    public var total: Int { items.count }
    public var isComplete: Bool { phase == .result }

    /// M1 "Take the placement" → start the word check.
    public mutating func begin() {
        guard phase == .welcome else { return }
        phase = items.isEmpty ? .result : .wordCheck
    }

    /// M1 "I'm a complete beginner" → skip the test (FR-1.4).
    public mutating func skipAsBeginner() {
        beginner = true
        answers = []
        index = items.count
        phase = .result
    }

    /// M2 yes/no answer for the current word; advances, finishing at the last item.
    public mutating func answer(known: Bool) {
        guard phase == .wordCheck, index < items.count else { return }
        answers.append(known)
        index += 1
        if index >= items.count { phase = .result }
    }
}

// MARK: - Seed (feeds KnownWordModel; FR-1.2 / FR-1.5)

/// A probabilistic placement seed: `wordID → prior P(known)`. Fed to
/// `KnownWordModel.project(_:seed:)`. Re-placement merges seeds with `merged(with:)`, which
/// takes the max prior per word so a re-run never destroys prior knowledge (FR-1.5).
public struct PlacementSeed: Equatable {
    public let priors: [Int: Double]

    public init(_ priors: [Int: Double] = [:]) {
        self.priors = priors.mapValues { Swift.min(1, Swift.max(0, $0)) }
    }

    public static let empty = PlacementSeed()

    public var isEmpty: Bool { priors.isEmpty }

    /// Merge two placement seeds without destroying knowledge (FR-1.5): keep the higher prior
    /// for each word. Because every `KnownWordModel` per-event update is monotonic in its input
    /// prior, re-projecting the same event log over a merged seed cannot lower any word's
    /// P(known).
    public func merged(with other: PlacementSeed) -> PlacementSeed {
        var out = priors
        for (id, p) in other.priors { out[id] = Swift.max(out[id] ?? 0, p) }
        return PlacementSeed(out)
    }
}

// MARK: - Result (M3)

public enum PlacementRoute: String, Equatable {
    case foundations // absolute-beginner bootstrap (FR-1.4)
    case lattice     // story lattice takes over
}

/// CEFR reading-band estimate (A0–B2). Everything is labeled "estimate" in the UI (I4/FR-6.3).
public enum CEFRBand: String, Equatable, Comparable, CaseIterable {
    case a0, a1, a2, b1, b2
    public var label: String { rawValue.uppercased() }
    private var order: Int { Self.allCases.firstIndex(of: self)! }
    public static func < (l: CEFRBand, r: CEFRBand) -> Bool { l.order < r.order }
}

/// The fitted logistic knowledge curve over frequency rank (FR-1.2). `pYes` is the raw
/// yes-rate; `pKnown` corrects for guessing using the foil false-alarm rate; `seedPrior`
/// additionally applies conservatism for the gate seed (risk §13).
public struct LogisticCurve: Equatable {
    public let intercept: Double // β0 on standardized log(rank)
    public let slope: Double     // β1 (negative: rarer ⇒ less likely known)
    public let mean: Double
    public let std: Double
    public let falseAlarm: Double
    public let conservatism: Double
    public let readingFactor: Double // FR-1.3 passage refinement (1 = un-refined)

    public init(intercept: Double, slope: Double, mean: Double, std: Double,
                falseAlarm: Double, conservatism: Double, readingFactor: Double = 1) {
        self.intercept = intercept
        self.slope = slope
        self.mean = mean
        self.std = std
        self.falseAlarm = falseAlarm
        self.conservatism = conservatism
        self.readingFactor = readingFactor
    }

    func feature(rank: Int) -> Double { (log(Double(max(1, rank))) - mean) / std }

    public func pYes(rank: Int) -> Double { sigmoid(intercept + slope * feature(rank: rank)) }

    /// Best estimate of P(known) at a frequency rank, corrected for guessing and (if the
    /// reading passages ran) for a reading/spoken gap (FR-1.3).
    public func pKnown(rank: Int) -> Double {
        let denom = Swift.max(1e-6, 1 - falseAlarm)
        return clamp01((pYes(rank: rank) - falseAlarm) / denom) * readingFactor
    }

    /// The conservative gate seed prior (FR-1.2 seed; risk §13 "conservative seed").
    public func seedPrior(rank: Int) -> Double { pKnown(rank: rank) * conservatism }
}

/// The placement outcome (M3): the seed, the fitted curve, both scale estimates, and the route
/// (00 §9 `PlacementResult`).
public struct PlacementResult: Equatable {
    public let seed: PlacementSeed
    public let curve: LogisticCurve
    public let falseAlarmRate: Double
    public let estimatedKnownCount: Int
    public let hskLevel: Int      // 1…6; 0 for the absolute-beginner path
    public let cefr: CEFRBand
    public let route: PlacementRoute
    public let itemCount: Int
    public let confidence: Double // 0…1 display proxy (labeled estimate)

    /// The absolute-beginner result (FR-1.4): empty seed, Foundations route.
    public static let beginner = PlacementResult(
        seed: .empty,
        curve: LogisticCurve(intercept: -6, slope: -1, mean: 0, std: 1, falseAlarm: 0, conservatism: 1),
        falseAlarmRate: 0, estimatedKnownCount: 0, hskLevel: 0, cefr: .a0,
        route: .foundations, itemCount: 0, confidence: 0)

    /// Ten sampled points of the knowledge curve (for the M3 bar chart), P(known) by rank band.
    public func curveSamples(maxRank: Int, count: Int = 10) -> [Double] {
        guard count > 0 else { return [] }
        return (0..<count).map { i in
            let frac = Double(i) / Double(max(1, count - 1))
            let rank = Int(1 + frac * Double(max(1, maxRank - 1)))
            return curve.pKnown(rank: rank)
        }
    }
}

/// A short reading-passage comprehension outcome used to refine the estimate (FR-1.3).
public struct PassageOutcome: Equatable {
    public let correct: Int
    public let total: Int
    public init(correct: Int, total: Int) {
        self.correct = max(0, correct)
        self.total = max(0, total)
    }
}

// MARK: - Small math helpers

@inline(__always) func sigmoid(_ x: Double) -> Double { 1 / (1 + exp(-x)) }
@inline(__always) func clamp01(_ x: Double) -> Double { Swift.min(1, Swift.max(0, x)) }
