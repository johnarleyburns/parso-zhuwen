import Foundation
import ZhuwenPacks

/// FrontierQueue orders the words the engine should teach next (FR-2.4): by HSK-3.0 level,
/// then corpus frequency, with a character-component familiarity bonus so words built from
/// characters the learner already knows surface earlier. Candidates are all lexicon words
/// not yet in the learner's known set.
public struct FrontierQueue {
    public let lexicon: LexiconStore
    /// Frequency-rank credit granted to a fully-familiar word (all component characters
    /// known). A CP-04 tuning constant; larger = the familiarity bonus outranks raw
    /// frequency more aggressively.
    public let familiarityCredit: Double

    public init(lexicon: LexiconStore, familiarityCredit: Double = 200) {
        self.lexicon = lexicon
        self.familiarityCredit = familiarityCredit
    }

    /// Effective ordering rank for a word given the known set (lower = teach sooner).
    func rank(_ w: WordRecord, known: Set<Int>) -> Double {
        Double(w.freqRank) - familiarityCredit * lexicon.knownCharFraction(of: w, known: known)
    }

    /// The ordered frontier candidates (words not in `known`).
    public func candidates(known: Set<Int>, limit: Int? = nil) -> [Int] {
        let ordered = lexicon.words
            .filter { !known.contains($0.id) }
            .sorted { a, b in
                if a.hsk3Level != b.hsk3Level { return a.hsk3Level < b.hsk3Level }
                let ra = rank(a, known: known), rb = rank(b, known: known)
                if ra != rb { return ra < rb }
                return a.id < b.id
            }
            .map { $0.id }
        if let limit { return Array(ordered.prefix(limit)) }
        return ordered
    }

    /// The next `count` frontier words as a set (the gate's allowed new-type set).
    public func frontierSet(known: Set<Int>, count: Int) -> Set<Int> {
        Set(candidates(known: known, limit: count))
    }
}
