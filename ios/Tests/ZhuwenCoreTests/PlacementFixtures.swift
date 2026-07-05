import Foundation
import ZhuwenPacks
@testable import ZhuwenCore

/// Shared synthetic lexicon for the placement tests: a frequency-ranked word list whose
/// HSK-3.0 levels follow the same cumulative thresholds the estimator maps against, so a
/// simulated learner's "true" band and the estimated band are measured on one ruler.
enum PlacementFixtures {
    static func syntheticLexicon(size: Int = 6000) -> LexiconStore {
        var words: [WordRecord] = []
        words.reserveCapacity(size)
        for rank in 1...size {
            let a = Character(UnicodeScalar(0x4E00 + (rank % 2000))!)
            let b = Character(UnicodeScalar(0x4E00 + 2000 + ((rank / 2000) % 2000))!)
            words.append(WordRecord(id: rank, simp: String([a, b]), pinyin: "x",
                                    hsk3Level: hskLevel(forRank: rank), freqRank: rank))
        }
        return LexiconStore(words)
    }

    /// Level a word of a given frequency rank belongs to (levels grow with rarity).
    static func hskLevel(forRank rank: Int) -> Int {
        let cum = PlacementEstimator.hskCumulative
        for (i, t) in cum.enumerated() where rank <= t { return i + 1 }
        return 6
    }

    /// A "true" learner: a steep logistic over log-rank with midpoint `midRank`. Returns the
    /// probability the learner truly knows a word at that rank.
    static func trueP(rank: Int, midRank: Double, slope: Double = 2.2) -> Double {
        1 / (1 + exp(slope * (log(Double(rank)) - log(midRank))))
    }
}
