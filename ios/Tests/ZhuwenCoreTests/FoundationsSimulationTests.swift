import XCTest
import ZhuwenPacks
@testable import ZhuwenCore

/// The CP-08a headline acceptance (plan §Tests): a scripted learner starting from **zero known
/// words** runs the Foundations program (F0 four-step → F1/F2 recombination per set) and reaches
/// the **F3 handoff** — the effective known set gating ≥20 A1 stories at ≥98% — after ~300 words.
/// Everything folds into the single `KnownWordModel` (I5); the handoff reuses the I1 coverage
/// formula shared with the Go gate.
final class FoundationsSimulationTests: XCTestCase {
    private let t0 = Date(timeIntervalSince1970: 1_700_000_000)
    private let programWords = 300
    private let setSize = 8

    /// The Foundations program: `programWords` words bucketed into semantic sets of `setSize`.
    private func program() -> FoundationsProgram {
        let words = (1...programWords).map {
            WordRecord(id: $0, simp: "词\($0)", pinyin: "cí", hsk3Level: 1, freqRank: $0)
        }
        let lex = LexiconStore(words)
        let cards = (1...programWords).map { id -> FoundationsCardRecord in
            let set = "set\((id - 1) / setSize)"
            return FoundationsCardRecord(wordID: id, imageID: "img", setID: set, stage: "F0", distractorIDs: [])
        }
        return FoundationsProgram(cards: cards, lexicon: lex)
    }

    /// 25 A1 stories, each spanning the whole 1…300 range so a story only clears the 98% gate
    /// once essentially the whole program is learned — the handoff should not fire early.
    private func a1Lattice() -> [LatticeStory] {
        let maxID = programWords
        return (0..<25).map { i -> LatticeStory in
            let ids = [i + 1, i + 51, i + 101, i + 151, i + 201, i + 251]
            var weights: [Int: Int] = [:]
            var bm = WordBitmap(bitCount: maxID + 1)
            var count = 0
            for id in ids { weights[id] = 3; bm.set(id); count += 3 }
            return LatticeStory(id: "a1-\(i)", bitmap: bm, tokenCount: count, typeWeights: weights, newTypeIDs: [])
        }
    }

    func testZeroKnowledgeLearnerReachesHandoffAroundThreeHundredWords() throws {
        let program = program()
        let a1 = a1Lattice()
        let gate = HandoffGate()
        var model = KnownWordModel()

        // Start: a true beginner gates zero A1 stories.
        XCTAssertEqual(gate.storiesGated(known: model.effectiveKnownSet(), a1Stories: a1), 0)
        XCTAssertFalse(gate.isReady(known: model.effectiveKnownSet(), a1Stories: a1))

        var handoffAtKnownCount: Int?
        var midpointGated: Int?

        for (idx, set) in program.sets.enumerated() {
            var session = FoundationsSession(set: set)
            while !session.isComplete {
                for e in session.advance(correct: true, at: t0) { model.apply(e) }
            }
            let known = model.effectiveKnownSet()
            if idx == program.sets.count / 2 { midpointGated = gate.storiesGated(known: known, a1Stories: a1) }
            if handoffAtKnownCount == nil && gate.isReady(known: known, a1Stories: a1) {
                handoffAtKnownCount = known.count
            }
        }

        // Halfway through the program the learner is still inside Foundations (handoff not yet met).
        XCTAssertNotNil(midpointGated)
        XCTAssertLessThan(midpointGated!, HandoffGate.threshold)

        // By the end of the ~300-word program the handoff has fired.
        XCTAssertTrue(gate.isReady(known: model.effectiveKnownSet(), a1Stories: a1))
        let all = program.allCards.map(\.wordID)
        for id in all {
            XCTAssertGreaterThanOrEqual(model.pKnown(id), KnownWordModel.knownThreshold)
        }

        // The handoff is reached "in ~300 words" (acceptance): between the 250th and 300th word.
        let reached = try XCTUnwrap(handoffAtKnownCount)
        XCTAssertGreaterThanOrEqual(reached, 250)
        XCTAssertLessThanOrEqual(reached, programWords)
    }

    /// Replay guarantee (I5): re-projecting the same event log yields the identical model, so a
    /// relaunch mid-Foundations restores the learner's exact state.
    func testFoundationsEventsReplayDeterministically() {
        let program = program()
        var log: [Event] = []
        for set in program.sets.prefix(5) {
            var session = FoundationsSession(set: set)
            while !session.isComplete { log.append(contentsOf: session.advance(correct: true, at: t0)) }
        }
        let a = KnownWordModel.project(log)
        let b = KnownWordModel.project(log)
        XCTAssertEqual(a, b)
    }
}
