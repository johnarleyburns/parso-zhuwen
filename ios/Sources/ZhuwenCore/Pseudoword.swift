import Foundation
import ZhuwenPacks

/// Generates **pseudoword foils** (FR-1.1): plausible-but-nonexistent 2-character compounds
/// built from the lexicon's own characters. A learner who marks foils "known" is overclaiming;
/// the false-alarm rate they produce calibrates the placement estimate (`PlacementEstimator`).
///
/// Plausibility comes from drawing characters from a frequency-ordered pool (frequent real
/// characters recombined), and every candidate is rejected if it is actually a real lexicon
/// word. Generation is deterministic per seed so the placement test is reproducible.
public struct PseudowordGenerator {
    /// Characters ordered most- to least-frequent (by the best freq rank of any word using them).
    public let charPool: [Character]
    private let realSurfaces: Set<String>

    public init(lexicon: LexiconStore) {
        self.init(words: lexicon.words)
    }

    public init(words: [WordRecord]) {
        var real = Set<String>()
        var bestRank: [Character: Int] = [:]
        for w in words {
            real.insert(w.simp)
            for c in w.simp where Self.isCJK(c) {
                if let r = bestRank[c] { if w.freqRank < r { bestRank[c] = w.freqRank } }
                else { bestRank[c] = w.freqRank }
            }
        }
        self.realSurfaces = real
        self.charPool = bestRank.sorted { a, b in
            a.value != b.value ? a.value < b.value : a.key < b.key
        }.map { $0.key }
    }

    /// `count` distinct 2-character foils that are not real words. Draws from the top
    /// `poolLimit` most-frequent characters so the compounds look native. Returns fewer than
    /// requested only if the character pool cannot yield that many distinct non-words.
    public func generate(count: Int, seed: UInt64, poolLimit: Int = 400) -> [String] {
        guard count > 0, charPool.count >= 2 else { return [] }
        let pool = Array(charPool.prefix(max(2, poolLimit)))
        var rng = SplitMix64(seed: seed)
        var out: [String] = []
        var seen = Set<String>()
        var guard_ = 0
        let maxTries = count * 200 + 1000
        while out.count < count && guard_ < maxTries {
            guard_ += 1
            let a = pool[Int(rng.next() % UInt64(pool.count))]
            let b = pool[Int(rng.next() % UInt64(pool.count))]
            if a == b { continue }
            let s = String([a, b])
            if realSurfaces.contains(s) || seen.contains(s) { continue }
            seen.insert(s)
            out.append(s)
        }
        return out
    }

    static func isCJK(_ c: Character) -> Bool {
        for scalar in c.unicodeScalars {
            if !(0x4E00...0x9FFF).contains(scalar.value) { return false }
        }
        return !c.unicodeScalars.isEmpty
    }
}
