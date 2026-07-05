import Foundation
import ZhuwenPacks

/// One story indexed for selection: its word-type bitmap (the NFR-2 substrate), the
/// coverage denominator, and per-type token weights for exact coverage of survivors.
public struct LatticeStory {
    public let id: String
    public let bitmap: WordBitmap
    public let tokenCount: Int
    public let typeWeights: [Int: Int]
    public let newTypeIDs: [Int]
    // Compact parallel arrays (aligned) for the allocation-free exact-coverage pass.
    let typeIDs: [Int]
    let typeWeightList: [Int]

    public init(id: String, bitmap: WordBitmap, tokenCount: Int, typeWeights: [Int: Int], newTypeIDs: [Int]) {
        self.id = id
        self.bitmap = bitmap
        self.tokenCount = tokenCount
        self.typeWeights = typeWeights
        self.newTypeIDs = newTypeIDs
        var ids: [Int] = []; var ws: [Int] = []
        ids.reserveCapacity(typeWeights.count); ws.reserveCapacity(typeWeights.count)
        for (k, v) in typeWeights { ids.append(k); ws.append(v) }
        self.typeIDs = ids
        self.typeWeightList = ws
    }

    /// Builds a lattice entry from a pack story record.
    public init(record: StoryRecord, maxWordID: Int) {
        var weights: [Int: Int] = [:]
        var bm = WordBitmap(bitCount: maxWordID + 1)
        for t in record.body where !t.isProperNoun && t.w >= 0 {
            weights[t.w, default: 0] += 1
            bm.set(t.w)
        }
        self.init(id: record.id, bitmap: bm, tokenCount: record.tokenCount, typeWeights: weights, newTypeIDs: record.newTypeIDs)
    }
}

/// The pack library ("lattice") indexed by exact word-type set (FR-3.1).
public struct LatticeIndex {
    public let stories: [LatticeStory]
    public let maxWordID: Int

    public init(stories: [LatticeStory], maxWordID: Int) {
        self.stories = stories
        self.maxWordID = maxWordID
    }

    public init(records: [StoryRecord], maxWordID: Int) {
        self.stories = records.map { LatticeStory(record: $0, maxWordID: maxWordID) }
        self.maxWordID = maxWordID
    }
}

/// A gated, scored story recommendation.
public struct Recommendation: Equatable {
    public let storyID: String
    public let coverageBps: Int
    public let frontierPayload: Int // new frontier types the story introduces
    public let srsPayload: Int      // known types in `learning` the story re-exposes
    public let score: Double
}

/// Selection weights and budgets (FR-3.2).
public struct SelectorConfig: Equatable {
    public var minCoverageBps: Int
    public var maxNewTypes: Int
    public var targetTokenCount: Int
    public var weightFrontier: Double
    public var weightSRS: Double
    public var weightLength: Double
    public var excludeRead: Bool

    public init(minCoverageBps: Int = 9800, maxNewTypes: Int = 8, targetTokenCount: Int = 250,
                weightFrontier: Double = 1.0, weightSRS: Double = 0.5, weightLength: Double = 0.002,
                excludeRead: Bool = true) {
        self.minCoverageBps = minCoverageBps
        self.maxNewTypes = maxNewTypes
        self.targetTokenCount = targetTokenCount
        self.weightFrontier = weightFrontier
        self.weightSRS = weightSRS
        self.weightLength = weightLength
        self.excludeRead = excludeRead
    }
}

/// Selector gates the lattice against the learner's known set (I1) and scores survivors
/// (FR-3.2). The gate hot path is pure bitmap AND + popcount so a 5,000-story lattice
/// scores in well under 50 ms (NFR-2); exact token coverage is computed only for the
/// handful of stories that clear the fast gate.
public struct Selector {
    public let config: SelectorConfig
    public init(config: SelectorConfig = SelectorConfig()) { self.config = config }

    /// Gate + score the lattice. `known`/`frontier`/`learning` are bitmaps over word IDs.
    public func select(index: LatticeIndex, known: WordBitmap, frontier: WordBitmap,
                       learning: WordBitmap, readStoryIDs: Set<String> = [], limit: Int? = nil) -> [Recommendation] {
        var out: [Recommendation] = []
        for s in index.stories {
            if config.excludeRead && readStoryIDs.contains(s.id) { continue }

            // Fast gate (NFR-2 path): uncovered-type count + frontier subset, bitmap-only.
            let uncovered = s.bitmap.subtractingPopcount(known)
            if uncovered > config.maxNewTypes { continue }
            if uncovered > 0 && !s.bitmap.uncoveredIsSubset(known: known, of: frontier) { continue }

            // Exact token coverage — only for stories that cleared the fast gate. Sum the
            // weights of uncovered types directly from the compact type list (no allocation).
            var newTokens = 0
            if uncovered > 0 {
                let ids = s.typeIDs, ws = s.typeWeightList
                for j in 0..<ids.count where !known.test(ids[j]) { newTokens += ws[j] }
            }
            let bps = CoverageGate.coverageBps(denom: s.tokenCount, newTokens: newTokens)
            if bps < config.minCoverageBps { continue }

            let srs = s.bitmap.intersectingPopcount(learning)
            let lengthPenalty = config.weightLength * Double(abs(s.tokenCount - config.targetTokenCount))
            let score = config.weightFrontier * Double(uncovered)
                + config.weightSRS * Double(srs)
                - lengthPenalty

            out.append(Recommendation(storyID: s.id, coverageBps: bps,
                                      frontierPayload: uncovered, srsPayload: srs, score: score))
        }

        out.sort { a, b in
            if a.score != b.score { return a.score > b.score }
            if a.coverageBps != b.coverageBps { return a.coverageBps > b.coverageBps }
            return a.storyID < b.storyID
        }
        if let limit { return Array(out.prefix(limit)) }
        return out
    }

    /// Convenience: build the bitmaps from a `KnownWordModel` + `FrontierQueue` and select.
    public func recommend(index: LatticeIndex, model: KnownWordModel, frontierQueue: FrontierQueue,
                          frontierCount: Int = 20, readStoryIDs: Set<String> = [], limit: Int? = nil) -> [Recommendation] {
        let bits = index.maxWordID + 1
        let effectiveKnown = model.effectiveKnownSet()
        let frontierSet = frontierQueue.frontierSet(known: effectiveKnown, count: frontierCount)
        let known = WordBitmap(ids: effectiveKnown, bitCount: bits)
        let frontier = WordBitmap(ids: frontierSet, bitCount: bits)
        let learning = WordBitmap(ids: model.learningWords(), bitCount: bits)
        return select(index: index, known: known, frontier: frontier, learning: learning,
                      readStoryIDs: readStoryIDs, limit: limit)
    }
}
