import XCTest
import ZhuwenPacks
@testable import ZhuwenCore

final class SelectorTests: XCTestCase {
    private struct LCG: RandomNumberGenerator {
        var s: UInt64
        mutating func next() -> UInt64 { s = s &* 6364136223846793005 &+ 1442695040888963407; return s }
    }

    private func story(_ id: String, _ weights: [Int: Int], maxWordID: Int, newTypeIDs: [Int] = []) -> LatticeStory {
        var bm = WordBitmap(bitCount: maxWordID + 1)
        var count = 0
        for (k, w) in weights { bm.set(k); count += w }
        return LatticeStory(id: id, bitmap: bm, tokenCount: count, typeWeights: weights, newTypeIDs: newTypeIDs)
    }

    // MARK: - I1 gate over the lattice

    func testGateAcceptsCoveredFrontierStoriesAndRejectsTheRest() {
        let maxID = 100
        let known = WordBitmap(ids: [1, 2, 3], bitCount: maxID + 1)
        let frontier = WordBitmap(ids: Array(10...18), bitCount: maxID + 1)
        let learning = WordBitmap(bitCount: maxID + 1)

        let stories = [
            story("A_all_known", [1: 100, 2: 100], maxWordID: maxID),          // 100% coverage
            story("B_one_frontier", [1: 197, 10: 3], maxWordID: maxID),        // 98.5%, uncovered ⊆ frontier
            story("C_non_frontier", [1: 197, 50: 3], maxWordID: maxID),        // 50 not frontier → reject
            story("D_below_98", [1: 97, 10: 3], maxWordID: maxID),             // 97% coverage → reject
            story("E_too_many_types", [1: 100, 10: 1, 11: 1, 12: 1, 13: 1,     // 9 new types → reject
                                       14: 1, 15: 1, 16: 1, 17: 1, 18: 1], maxWordID: maxID),
        ]
        let index = LatticeIndex(stories: stories, maxWordID: maxID)
        let recs = Selector().select(index: index, known: known, frontier: frontier, learning: learning)

        XCTAssertEqual(Set(recs.map { $0.storyID }), ["A_all_known", "B_one_frontier"])
        let b = recs.first { $0.storyID == "B_one_frontier" }
        XCTAssertEqual(b?.coverageBps, 9850)
        XCTAssertEqual(b?.frontierPayload, 1)
    }

    // MARK: - Scoring (FR-3.2)

    func testScoringPrefersFrontierThenSRSPayload() {
        let maxID = 100
        // Word 2 is a learning word that sits in the effective known set (FR-2.3), so it is in
        // both `known` and `learning`.
        let known = WordBitmap(ids: [1, 2], bitCount: maxID + 1)
        let frontier = WordBitmap(ids: [10, 11], bitCount: maxID + 1)
        let learning = WordBitmap(ids: [2], bitCount: maxID + 1)

        let stories = [
            story("P_frontier2", [1: 196, 10: 2, 11: 2], maxWordID: maxID), // 2 new frontier types
            story("Q_srs", [1: 100, 2: 100], maxWordID: maxID),            // re-exposes learning word 2
        ]
        let index = LatticeIndex(stories: stories, maxWordID: maxID)
        let recs = Selector().select(index: index, known: known, frontier: frontier, learning: learning)

        XCTAssertEqual(recs.map { $0.storyID }, ["P_frontier2", "Q_srs"])
        XCTAssertEqual(recs[0].frontierPayload, 2)
        XCTAssertEqual(recs[1].srsPayload, 1)
        XCTAssertGreaterThan(recs[0].score, recs[1].score)
    }

    func testExcludesAlreadyReadStories() {
        let maxID = 100
        let known = WordBitmap(ids: [1, 2], bitCount: maxID + 1)
        let frontier = WordBitmap(ids: [10, 11], bitCount: maxID + 1)
        let learning = WordBitmap(ids: [2], bitCount: maxID + 1)
        let index = LatticeIndex(stories: [
            story("P_frontier2", [1: 196, 10: 2, 11: 2], maxWordID: maxID),
            story("Q_srs", [1: 100, 2: 100], maxWordID: maxID),
        ], maxWordID: maxID)

        let recs = Selector().select(index: index, known: known, frontier: frontier, learning: learning,
                                     readStoryIDs: ["P_frontier2"])
        XCTAssertEqual(recs.map { $0.storyID }, ["Q_srs"])
    }

    // MARK: - E2E over the real vendored pack

    func testRecommendsRealFixtureStories() throws {
        let store = try Fixtures.store()
        let stories = try store.stories()
        let lex = try store.lexicon()
        let lexStore = LexiconStore(lex)
        let index = LatticeIndex(records: stories, maxWordID: lexStore.maxWordID)

        // A learner who knows every word except the frontier words the pack teaches. Building
        // the model from an event log exercises the full CP-04 stack: events → projection →
        // effective known set → frontier queue → selector gate over real pack bitmaps.
        let frontierIDs = Set(stories.flatMap { $0.newTypeIDs })
        let log = EventLog()
        let t0 = Date(timeIntervalSince1970: 1_700_000_000)
        for w in lex where !frontierIDs.contains(w.id) { log.append(.markKnown(w.id, at: t0)) }
        let model = KnownWordModel.project(log.events)

        let recs = Selector().recommend(index: index, model: model,
                                        frontierQueue: FrontierQueue(lexicon: lexStore), frontierCount: 20)
        XCTAssertEqual(recs.count, stories.count, "every gated fixture story should be recommendable")
        for r in recs { XCTAssertGreaterThanOrEqual(r.coverageBps, 9800, "\(r.storyID) below 98%") }
    }

    // MARK: - NFR-2 selector benchmark (5,000 stories < 50 ms)

    func testGates5kStoryLatticeUnder50ms() {
        let maxID = 11_000
        var rng = LCG(s: 0x5EED_1234_5678)
        let known = WordBitmap(ids: Set(1...5000), bitCount: maxID + 1)
        let frontierIDs = Array(5001...5020)
        let frontier = WordBitmap(ids: frontierIDs, bitCount: maxID + 1)
        let learning = WordBitmap(ids: Set(4900...5000), bitCount: maxID + 1)

        var lattice: [LatticeStory] = []
        lattice.reserveCapacity(5000)
        for s in 0..<5000 {
            var weights: [Int: Int] = [:]
            var bm = WordBitmap(bitCount: maxID + 1)
            var count = 0
            for _ in 0..<45 {
                let id = Int.random(in: 1...5000, using: &rng)
                bm.set(id); weights[id, default: 0] += 5; count += 5
            }
            let nf = Int.random(in: 0...2, using: &rng)
            let shuffled = frontierIDs.shuffled(using: &rng)
            for i in 0..<nf { let id = shuffled[i]; bm.set(id); weights[id, default: 0] += 3; count += 3 }
            lattice.append(LatticeStory(id: "s\(s)", bitmap: bm, tokenCount: count, typeWeights: weights, newTypeIDs: []))
        }
        let index = LatticeIndex(stories: lattice, maxWordID: maxID)
        let selector = Selector()

        // Warm the caches once, then measure the scoring pass.
        _ = selector.select(index: index, known: known, frontier: frontier, learning: learning)
        let start = CFAbsoluteTimeGetCurrent()
        let recs = selector.select(index: index, known: known, frontier: frontier, learning: learning)
        let ms = (CFAbsoluteTimeGetCurrent() - start) * 1000
        XCTAssertFalse(recs.isEmpty)

        #if DEBUG
        // `swift test` builds unoptimized, so the wall-clock here (~1 s) is not comparable to
        // NFR-2's 50 ms target, which is defined for an optimized/device build. The real gate
        // runs under `make bench` (swift test -c release); here we only guard against gross
        // algorithmic regressions and surface the number.
        print(String(format: "NFR-2 selector [DEBUG, unoptimized]: %d/5000 in %.1f ms — run `make bench` for the 50 ms gate", recs.count, ms))
        XCTAssertLessThan(ms, 5_000, "gross-regression guard (real NFR-2 gate is in release: make bench)")
        #else
        print(String(format: "NFR-2 selector [release]: %d/5000 in %.2f ms", recs.count, ms))
        XCTAssertLessThan(ms, 50, "selector must score a 5k-story lattice in < 50 ms (NFR-2)")
        #endif
    }
}
