import Foundation
import ZhuwenPacks

/// A read-only view over a pack's lexicon (handoff §5 `LexiconStore`). Indexes words by ID
/// and, for the frontier queue's character-familiarity bonus, maps single characters to the
/// single-character word that teaches them.
public struct LexiconStore {
    public let words: [WordRecord]
    private let byID: [Int: WordRecord]
    private let charWordID: [Character: Int]
    public let maxWordID: Int

    public init(_ words: [WordRecord]) {
        self.words = words
        self.byID = Dictionary(words.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        var chars: [Character: Int] = [:]
        var maxID = 0
        for w in words {
            if w.id > maxID { maxID = w.id }
            if w.simp.count == 1, let c = w.simp.first, chars[c] == nil { chars[c] = w.id }
        }
        self.charWordID = chars
        self.maxWordID = maxID
    }

    public func word(_ id: Int) -> WordRecord? { byID[id] }

    /// Fraction of a word's characters the learner already knows (single-character words in
    /// `known`). 1.0 if every component character is known; 0.0 if none. Used as the FR-2.4
    /// familiarity bonus.
    public func knownCharFraction(of word: WordRecord, known: Set<Int>) -> Double {
        let chars = Array(word.simp)
        guard !chars.isEmpty else { return 0 }
        var hit = 0
        for c in chars {
            if let id = charWordID[c], known.contains(id) { hit += 1 }
        }
        return Double(hit) / Double(chars.count)
    }
}
