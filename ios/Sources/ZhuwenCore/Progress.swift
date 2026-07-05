import Foundation
import ZhuwenPacks

/// The progress dashboard model (M10, FR-6.3): separate reading and listening CEFR band estimates,
/// lexicon growth, HSK-3.0 level + words-to-next, and a CEFR can-do line. Every field is an
/// *estimate* (I4/FR-6.3) — the UI labels it so and never makes a certification claim.
public struct ProgressReport: Equatable {
    public let readingBand: CEFRBand
    public let readingProgressToNext: Double   // 0…1 toward the next band
    public let readingConfidence: Double       // 0…1 display proxy
    public let listeningBand: CEFRBand
    public let listeningProgressToNext: Double
    public let listeningConfidence: Double
    public let wordsKnown: Int
    public let wordsKnownThisWeek: Int
    public let hskLevel: Int                    // 1…6; 0 before any coverage
    public let wordsToNextHSK: Int
    public let weeklyKnownSeries: [Int]         // cumulative words-known at each week boundary
    public let canDo: String

    public var readingLabel: String { readingBand.label }
    public var listeningLabel: String { listeningBand.label }
}

/// Computes a `ProgressReport` from the learner's known-word model + event log (both-skill, FR-6.3).
/// Reading is driven by the reading-oriented known set; **listening is computed independently** from
/// the `.listen` events (blind passes count double, FR-5.2) so the two skills never contaminate each
/// other — the both-skill separation the CP-07 acceptance checks. Pure and host-testable.
public struct ProgressEstimator {
    /// A word counts as "known" for coverage at this P(known) (FR-2.3).
    public static let knownThreshold = KnownWordModel.knownThreshold
    /// Fraction of a HSK level's words that must be known to claim that level.
    public static let hskCoverageTarget = 0.8

    public init() {}

    public func report(model: KnownWordModel, events: [Event], lexicon: [WordRecord],
                       seed: [Int: Double] = [:], now: Date, weeks: Int = 8) -> ProgressReport {
        let knownIDs = model.effectiveKnownSet()
        let wordsKnown = knownIDs.count

        // Reading band from HSK-level coverage.
        let (hskLevel, wordsToNext) = hskProgress(knownIDs: knownIDs, lexicon: lexicon)
        let readingBand = Self.cefr(forHSK: hskLevel)
        let readingProgress = hskFractionToNext(knownIDs: knownIDs, lexicon: lexicon, level: hskLevel)
        let readingConfidence = min(1, Double(wordsKnown) / 1000)

        // Listening band from the (skill-separated) listen events.
        let listen = listeningEstimate(events: events)

        // Weekly growth: re-project the log truncated at each week boundary (I5 replay).
        let series = weeklySeries(events: events, seed: seed, now: now, weeks: weeks)
        let weekAgo = now.addingTimeInterval(-7 * 86_400)
        let gainedThisWeek = max(0, wordsKnown - knownCount(at: weekAgo, events: events, seed: seed))

        return ProgressReport(
            readingBand: readingBand,
            readingProgressToNext: readingProgress,
            readingConfidence: readingConfidence,
            listeningBand: listen.band,
            listeningProgressToNext: listen.progress,
            listeningConfidence: listen.confidence,
            wordsKnown: wordsKnown,
            wordsKnownThisWeek: gainedThisWeek,
            hskLevel: hskLevel,
            wordsToNextHSK: wordsToNext,
            weeklyKnownSeries: series,
            canDo: Self.canDo(for: readingBand))
    }

    // MARK: - Reading / HSK

    private func hskProgress(knownIDs: Set<Int>, lexicon: [WordRecord]) -> (level: Int, toNext: Int) {
        var total: [Int: Int] = [:]
        var known: [Int: Int] = [:]
        for w in lexicon where w.hsk3Level > 0 {
            total[w.hsk3Level, default: 0] += 1
            if knownIDs.contains(w.id) { known[w.hsk3Level, default: 0] += 1 }
        }
        var level = 0
        for lvl in total.keys.sorted() {
            let cov = Double(known[lvl] ?? 0) / Double(max(1, total[lvl]!))
            if cov >= Self.hskCoverageTarget { level = lvl } else { break }
        }
        let next = level + 1
        let toNext = (total[next] ?? 0) - (known[next] ?? 0)
        return (level, max(0, toNext))
    }

    private func hskFractionToNext(knownIDs: Set<Int>, lexicon: [WordRecord], level: Int) -> Double {
        let next = level + 1
        var total = 0, known = 0
        for w in lexicon where w.hsk3Level == next {
            total += 1
            if knownIDs.contains(w.id) { known += 1 }
        }
        guard total > 0 else { return 0 }
        return min(1, Double(known) / Double(total) / Self.hskCoverageTarget)
    }

    // MARK: - Listening (skill-separated, folds `.listen`)

    private func listeningEstimate(events: [Event]) -> (band: CEFRBand, progress: Double, confidence: Double) {
        var sighted = Set<String>()
        var blind = Set<String>()
        for e in events where e.kind == .listen {
            guard let sid = e.storyID else { continue }
            if e.blind == true { blind.insert(sid) } else { sighted.insert(sid) }
        }
        // Blind passes are stronger listening evidence (FR-5.2): count them double.
        let ladder = Double(blind.count) * 2 + Double(sighted.subtracting(blind).count)
        let band: CEFRBand
        let progress: Double
        switch ladder {
        case ..<1:   band = .a0; progress = ladder
        case ..<4:   band = .a1; progress = (ladder - 1) / 3
        case ..<10:  band = .a2; progress = (ladder - 4) / 6
        case ..<20:  band = .b1; progress = (ladder - 10) / 10
        default:     band = .b2; progress = 1
        }
        let confidence = min(1, ladder / 10)
        return (band, min(1, max(0, progress)), confidence)
    }

    // MARK: - Growth series (replay)

    private func weeklySeries(events: [Event], seed: [Int: Double], now: Date, weeks: Int) -> [Int] {
        guard weeks > 0 else { return [] }
        return (0..<weeks).map { i in
            let boundary = now.addingTimeInterval(-Double(weeks - 1 - i) * 7 * 86_400)
            return knownCount(at: boundary, events: events, seed: seed)
        }
    }

    private func knownCount(at t: Date, events: [Event], seed: [Int: Double]) -> Int {
        let sliced = events.filter { $0.ts <= t }
        return KnownWordModel.project(sliced, seed: seed).effectiveKnownSet().count
    }

    // MARK: - Mappings (labeled estimates; I4)

    static func cefr(forHSK level: Int) -> CEFRBand {
        switch level {
        case 0: return .a0
        case 1: return .a1
        case 2: return .a1
        case 3: return .a2
        case 4: return .b1
        default: return .b2
        }
    }

    static func canDo(for band: CEFRBand) -> String {
        switch band {
        case .a0: return "Building your first words before stories begin."
        case .a1: return "Understand very short, simple texts a single phrase at a time."
        case .a2: return "Understand short, simple texts on familiar, concrete matters."
        case .b1: return "Understand texts on familiar topics encountered in daily life."
        case .b2: return "Read articles and reports on contemporary matters with ease."
        }
    }
}
