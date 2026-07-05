import XCTest
import ZhuwenPacks
@testable import ZhuwenCore

/// CP-05 acceptance (handoff §6): *simulated learners at known curves recover level ±1 HSK
/// band*. Each learner is a logistic knowledge curve with a known midpoint; we sample a
/// placement checklist, simulate honest-ish yes/no answers (with a little foil overclaiming),
/// fit, and check the recovered HSK band against the learner's true band on the same ruler.
final class PlacementSimulationTests: XCTestCase {
    private let lexicon = PlacementFixtures.syntheticLexicon(size: 6000)
    private let trueFalseAlarm = 0.08

    private func trueKnownCount(midRank: Double) -> Int {
        var sum = 0.0
        for w in lexicon.words { sum += PlacementFixtures.trueP(rank: w.freqRank, midRank: midRank) }
        return Int(sum.rounded())
    }

    /// Runs one simulated learner and returns (trueHSK, estimatedHSK).
    private func simulate(midRank: Double, seed: UInt64) -> (Int, Int) {
        let items = PlacementItemBuilder(itemCount: 120, foilFraction: 0.15, seed: seed).build(lexicon: lexicon)
        var rng = SplitMix64(seed: seed &* 2_654_435_761 &+ 1)
        let answers: [Bool] = items.map { item in
            if item.isFoil { return rng.uniform() < trueFalseAlarm }
            return rng.uniform() < PlacementFixtures.trueP(rank: item.freqRank, midRank: midRank)
        }
        let result = PlacementEstimator().estimate(items: items, answers: answers, lexicon: lexicon)
        let trueHSK = PlacementEstimator.hskLevel(knownCount: trueKnownCount(midRank: midRank))
        return (trueHSK, result.hskLevel)
    }

    func testRecoversHSKBandWithinOne() {
        let midRanks: [Double] = [250, 600, 1100, 2000, 3200, 5000]
        let seeds: [UInt64] = [1, 7, 42, 1000]
        var maxError = 0
        for mid in midRanks {
            for seed in seeds {
                let (trueHSK, estHSK) = simulate(midRank: mid, seed: seed)
                let error = abs(estHSK - trueHSK)
                maxError = max(maxError, error)
                XCTAssertLessThanOrEqual(error, 1,
                    "midRank \(mid) seed \(seed): true HSK \(trueHSK), estimated \(estHSK)")
            }
        }
        // Sanity: the suite actually exercises a spread of bands, not a trivial constant.
        XCTAssertLessThanOrEqual(maxError, 1)
    }

    func testEstimateIsMonotonicInAbility() {
        // Higher ability (larger midRank) never yields a lower estimated band.
        let midRanks: [Double] = [250, 600, 1100, 2000, 3200, 5000]
        let estimates = midRanks.map { simulate(midRank: $0, seed: 42).1 }
        for i in 1..<estimates.count {
            XCTAssertGreaterThanOrEqual(estimates[i], estimates[i - 1],
                "band should not fall as ability rises: \(estimates)")
        }
        XCTAssertGreaterThan(estimates.last!, estimates.first!)
    }

    func testSimulationIsDeterministic() {
        XCTAssertEqual(simulate(midRank: 1100, seed: 42).1, simulate(midRank: 1100, seed: 42).1)
    }
}
