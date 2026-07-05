import Foundation

/// A review grade (FR-7.1). Maps 1:1 to `Event.grade` (0…3) and to the FSRS rating 1…4.
public enum Rating: Int, Equatable, CaseIterable {
    case again = 0, hard = 1, good = 2, easy = 3

    /// FSRS rating (1…4) used inside the algorithm.
    var fsrs: Int { rawValue + 1 }
    public var label: String {
        switch self {
        case .again: return "Again"
        case .hard: return "Hard"
        case .good: return "Good"
        case .easy: return "Easy"
        }
    }
}

/// The memory state of one review card (FR-2.1 "FSRS memory params"). It is *derived* — the app
/// never stores it out of band; it is reconstructed by folding the `.reviewGrade` events (I5),
/// so the whole learner state stays a replayable projection of the log.
public struct FSRSCard: Equatable {
    public var stability: Double   // days; larger ⇒ remembered longer
    public var difficulty: Double  // 1…10
    public var due: Date           // next review time
    public var lastReview: Date
    public var reps: Int
    public var lapses: Int

    public init(stability: Double, difficulty: Double, due: Date, lastReview: Date,
                reps: Int, lapses: Int) {
        self.stability = stability
        self.difficulty = difficulty
        self.due = due
        self.lastReview = lastReview
        self.reps = reps
        self.lapses = lapses
    }
}

/// The FSRS scheduler — an open, on-device spaced-repetition algorithm (FR-7.1, "open algorithm,
/// on-device"). This is FSRS-4.5: 17 weights, power-law forgetting curve. Pure and deterministic;
/// no network, no generation (I2/I3). Formulas per the published FSRS-4.5 spec; the default weights
/// are the community defaults (the app ships them; per-user optimization is out of v1 scope).
public struct FSRSScheduler: Equatable {
    /// Target retention: schedule so the learner has ~90% recall at review time.
    public var requestRetention: Double
    /// Cap on any single interval, in days (10 years). Keeps the review tab sane.
    public var maximumInterval: Double
    public var weights: [Double]

    /// FSRS-4.5 default weights (w0…w16).
    public static let defaultWeights: [Double] = [
        0.4072, 1.1829, 3.1262, 15.4722, 7.2102, 0.5316, 1.0651, 0.0234,
        1.6960, 0.1216, 1.0524, 1.9813, 0.0953, 0.2975, 2.2042, 0.2407, 2.9466,
    ]

    public static let `default` = FSRSScheduler()

    public init(requestRetention: Double = 0.9, maximumInterval: Double = 3650,
                weights: [Double] = FSRSScheduler.defaultWeights) {
        self.requestRetention = min(0.97, max(0.7, requestRetention))
        self.maximumInterval = max(1, maximumInterval)
        self.weights = weights.count == 17 ? weights : FSRSScheduler.defaultWeights
    }

    // Power-law forgetting curve constants (FSRS-4.5).
    private static let decay = -0.5
    private static let factor = 0.9.powered(1 / decay) - 1  // ≈ 0.2345679

    /// Retrievability after `elapsedDays` at stability `s`: R = (1 + FACTOR·t/s)^DECAY.
    public func retrievability(elapsedDays: Double, stability s: Double) -> Double {
        guard s > 0 else { return 0 }
        return (1 + Self.factor * max(0, elapsedDays) / s).powered(Self.decay)
    }

    /// The interval (days) that lands stability `s` at the requested retention.
    public func interval(stability s: Double) -> Double {
        let ivl = (s / Self.factor) * (requestRetention.powered(1 / Self.decay) - 1)
        return min(maximumInterval, max(1, ivl.rounded()))
    }

    /// Advance a card by a grade at time `now`. `card == nil` is the first review of the word.
    public func review(_ card: FSRSCard?, rating: Rating, at now: Date) -> FSRSCard {
        let w = weights
        guard let card else {
            // First review: seed stability/difficulty from the grade (FSRS-4.5 init).
            let s0 = max(0.1, w[rating.fsrs - 1])
            let d0 = clampDifficulty(w[4] - exp(w[5] * Double(rating.fsrs - 1)) + 1)
            let ivl = interval(stability: s0)
            return FSRSCard(stability: s0, difficulty: d0,
                            due: now.addingDays(ivl), lastReview: now,
                            reps: 1, lapses: rating == .again ? 1 : 0)
        }

        let elapsed = max(0, now.timeIntervalSince(card.lastReview) / 86_400)
        let r = retrievability(elapsedDays: elapsed, stability: card.stability)

        // Difficulty update with mean reversion toward D0(Easy) (FSRS-4.5).
        let d0Easy = w[4] - exp(w[5] * 3) + 1
        let dDelta = card.difficulty - w[6] * Double(rating.fsrs - 3)
        let difficulty = clampDifficulty(w[7] * d0Easy + (1 - w[7]) * dDelta)

        let stability: Double
        var lapses = card.lapses
        if rating == .again {
            // Post-lapse stability.
            let sMin = w[11] * card.difficulty.powered(-w[12])
                * ((card.stability + 1).powered(w[13]) - 1)
                * exp(w[14] * (1 - r))
            stability = max(0.1, min(card.stability, sMin))
            lapses += 1
        } else {
            let hardPenalty = rating == .hard ? w[15] : 1
            let easyBonus = rating == .easy ? w[16] : 1
            let growth = exp(w[8]) * (11 - difficulty) * card.stability.powered(-w[9])
                * (exp(w[10] * (1 - r)) - 1) * hardPenalty * easyBonus
            stability = max(0.1, card.stability * (1 + growth))
        }

        let ivl = interval(stability: stability)
        return FSRSCard(stability: stability, difficulty: difficulty,
                        due: now.addingDays(ivl), lastReview: now,
                        reps: card.reps + 1, lapses: lapses)
    }

    /// Projected next interval (days) for each grade, for the M9 grade buttons.
    public func intervals(_ card: FSRSCard?, at now: Date) -> [Rating: Int] {
        var out: [Rating: Int] = [:]
        for g in Rating.allCases {
            let next = review(card, rating: g, at: now)
            out[g] = Int(max(1, next.due.timeIntervalSince(now) / 86_400).rounded())
        }
        return out
    }

    private func clampDifficulty(_ d: Double) -> Double { min(10, max(1, d)) }
}

// MARK: - Small helpers

extension Double {
    @inline(__always) func powered(_ e: Double) -> Double { pow(self, e) }
}

extension Date {
    @inline(__always) func addingDays(_ d: Double) -> Date { addingTimeInterval(d * 86_400) }
}
