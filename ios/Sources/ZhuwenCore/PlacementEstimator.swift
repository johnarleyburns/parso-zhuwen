import Foundation
import ZhuwenPacks

/// Tuning for placement estimation (FR-1.2). Defaults chosen for a **conservative** seed
/// (risk §13): the reported estimate is honest, but the gate seed is discounted and floored so
/// a mis-seed starves rather than floods, correcting quickly from lookups (I5).
public struct PlacementConfig: Equatable {
    public var conservatism: Double       // multiplies P(known) for the gate seed only
    public var seedFloor: Double          // drop priors below this from the seed (stay sparse)
    public var ridge: Double              // IRLS L2 on the slope (finite fit under separation)
    public var iterations: Int
    public var maxFalseAlarm: Double      // cap the guessing correction denominator
    public var foundationsThreshold: Int  // known-count below this ⇒ route to Foundations

    public init(conservatism: Double = 0.9, seedFloor: Double = 0.15, ridge: Double = 1.0,
                iterations: Int = 40, maxFalseAlarm: Double = 0.9, foundationsThreshold: Int = 150) {
        self.conservatism = conservatism
        self.seedFloor = seedFloor
        self.ridge = ridge
        self.iterations = iterations
        self.maxFalseAlarm = maxFalseAlarm
        self.foundationsThreshold = foundationsThreshold
    }
}

/// Fits a logistic knowledge curve over frequency rank from placement answers and turns it
/// into a probabilistic seed of the known-word model plus CEFR/HSK estimates (FR-1.2, handoff
/// §5). Pure and deterministic — the acceptance simulation depends on it.
public struct PlacementEstimator {
    public let config: PlacementConfig
    public init(config: PlacementConfig = PlacementConfig()) { self.config = config }

    /// HSK-3.0 cumulative vocabulary thresholds (words) for levels 1…6.
    public static let hskCumulative = [500, 1272, 2245, 3245, 4316, 5456]

    // MARK: - Estimation

    public func estimate(items: [PlacementItem], answers: [Bool], lexicon: LexiconStore) -> PlacementResult {
        estimate(items: items, answers: answers, words: lexicon.words)
    }

    public func estimate(items: [PlacementItem], answers: [Bool], words: [WordRecord]) -> PlacementResult {
        var xs: [Double] = []
        var ys: [Double] = []
        var foilYes = 0, foilN = 0
        for (item, ans) in zip(items, answers) {
            if item.isFoil {
                foilN += 1
                if ans { foilYes += 1 }
            } else {
                xs.append(log(Double(max(1, item.freqRank))))
                ys.append(ans ? 1 : 0)
            }
        }
        let rawFalseAlarm = foilN > 0 ? Double(foilYes) / Double(foilN) : 0
        let falseAlarm = Swift.min(config.maxFalseAlarm, Swift.max(0, rawFalseAlarm))

        // Standardize the log-rank feature.
        let n = Swift.max(1, xs.count)
        let mean = xs.reduce(0, +) / Double(n)
        let varc = xs.reduce(0) { $0 + ($1 - mean) * ($1 - mean) } / Double(n)
        let std = varc > 1e-9 ? sqrt(varc) : 1
        let z = xs.map { ($0 - mean) / std }
        let (b0, b1) = fitIRLS(z, ys)

        let curve = LogisticCurve(intercept: b0, slope: b1, mean: mean, std: std,
                                  falseAlarm: falseAlarm, conservatism: config.conservatism)
        return result(curve: curve, words: words, itemCount: items.count, realItems: xs.count,
                      falseAlarm: rawFalseAlarm)
    }

    // MARK: - Reading-passage refinement (FR-1.3)

    /// Folds two short comprehension outcomes into the estimate: poor comprehension (strong
    /// spoken vocab, weak reading recognition) pulls the estimate/seed down; good comprehension
    /// does not pull it down.
    public func refine(_ base: PlacementResult, passages: [PassageOutcome], words: [WordRecord]) -> PlacementResult {
        guard base.route != .foundations || !base.seed.isEmpty else { return base }
        let totalCorrect = passages.reduce(0) { $0 + $1.correct }
        let totalItems = passages.reduce(0) { $0 + $1.total }
        guard totalItems > 0 else { return base }
        let score = Double(totalCorrect) / Double(totalItems)
        let factor = Swift.min(1.05, Swift.max(0.6, 0.6 + 0.5 * score))
        let c = base.curve
        let refined = LogisticCurve(intercept: c.intercept, slope: c.slope, mean: c.mean,
                                    std: c.std, falseAlarm: c.falseAlarm,
                                    conservatism: c.conservatism, readingFactor: factor)
        return result(curve: refined, words: words, itemCount: base.itemCount,
                      realItems: base.itemCount, falseAlarm: base.falseAlarmRate)
    }

    public func refine(_ base: PlacementResult, passages: [PassageOutcome], lexicon: LexiconStore) -> PlacementResult {
        refine(base, passages: passages, words: lexicon.words)
    }

    // MARK: - Band mapping (FR-1.2)

    /// HSK-3.0 level (1…6) for an estimated known-word count.
    public static func hskLevel(knownCount: Int) -> Int {
        var passed = 0
        for t in hskCumulative where knownCount >= t { passed += 1 }
        return Swift.min(6, Swift.max(1, passed))
    }

    /// CEFR reading band for an estimated known-word count.
    public static func cefr(knownCount: Int) -> CEFRBand {
        switch knownCount {
        case ..<300: return .a0
        case ..<800: return .a1
        case ..<1500: return .a2
        case ..<3000: return .b1
        default: return .b2
        }
    }

    // MARK: - Internals

    private func result(curve: LogisticCurve, words: [WordRecord], itemCount: Int,
                        realItems: Int, falseAlarm: Double) -> PlacementResult {
        var expectedKnown = 0.0
        var priors: [Int: Double] = [:]
        for w in words where w.freqRank > 0 {
            expectedKnown += curve.pKnown(rank: w.freqRank)
            let sp = curve.seedPrior(rank: w.freqRank)
            if sp >= config.seedFloor { priors[w.id] = sp }
        }
        let known = Int(expectedKnown.rounded())
        let route: PlacementRoute = known < config.foundationsThreshold ? .foundations : .lattice
        let confidence = clamp01((1 - falseAlarm) * Swift.min(1, Double(realItems) / 60))
        return PlacementResult(
            seed: PlacementSeed(priors),
            curve: curve,
            falseAlarmRate: falseAlarm,
            estimatedKnownCount: known,
            hskLevel: Self.hskLevel(knownCount: known),
            cefr: Self.cefr(knownCount: known),
            route: route,
            itemCount: itemCount,
            confidence: confidence)
    }

    /// Ridge-regularized IRLS (Newton) for 1-feature logistic regression. The ridge keeps the
    /// slope finite when the learner's answers are perfectly separable (all-yes / all-no).
    private func fitIRLS(_ x: [Double], _ y: [Double]) -> (Double, Double) {
        let n = x.count
        guard n > 0 else { return (0, -1) }
        var b0 = 0.0, b1 = 0.0
        for _ in 0..<config.iterations {
            var g0 = 0.0, g1 = 0.0
            var h00 = 0.0, h01 = 0.0, h11 = 0.0
            for i in 0..<n {
                let p = sigmoid(b0 + b1 * x[i])
                let w = Swift.max(1e-6, p * (1 - p))
                let r = y[i] - p
                g0 += r
                g1 += r * x[i]
                h00 += w
                h01 += w * x[i]
                h11 += w * x[i] * x[i]
            }
            // Ridge on the slope only (intercept gets a tiny numerical floor).
            g1 -= config.ridge * b1
            h00 += 1e-3
            h11 += config.ridge
            let det = h00 * h11 - h01 * h01
            if abs(det) < 1e-12 { break }
            let d0 = (h11 * g0 - h01 * g1) / det
            let d1 = (-h01 * g0 + h00 * g1) / det
            b0 += d0
            b1 += d1
            if abs(d0) + abs(d1) < 1e-9 { break }
        }
        return (b0, b1)
    }
}
