import XCTest
import ZhuwenPacks
@testable import ZhuwenCore

final class FoundationsTests: XCTestCase {
    private let t0 = Date(timeIntervalSince1970: 1_700_000_000)

    // MARK: - Helpers

    /// A small program: two sets of animals/food cards over a synthetic lexicon.
    private func program() -> FoundationsProgram {
        var words: [WordRecord] = []
        for id in 1...8 {
            words.append(WordRecord(id: id, simp: "词\(id)", pinyin: "cí", hsk3Level: 1, freqRank: id))
        }
        let lex = LexiconStore(words)
        let cards = [
            fc(1, "animals"), fc(2, "animals"), fc(3, "animals"), fc(4, "animals"),
            fc(5, "food"), fc(6, "food"), fc(7, "food"), fc(8, "food"),
        ]
        return FoundationsProgram(cards: cards, lexicon: lex)
    }

    private func fc(_ id: Int, _ set: String, distractors: [Int] = []) -> FoundationsCardRecord {
        FoundationsCardRecord(wordID: id, imageID: "img-\(id)", setID: set, stage: "F0", distractorIDs: distractors)
    }

    private func lattice(_ stories: [(String, [Int: Int])], maxWordID: Int) -> [LatticeStory] {
        stories.map { (id, weights) in
            var bm = WordBitmap(bitCount: maxWordID + 1)
            var count = 0
            for (k, w) in weights { bm.set(k); count += w }
            return LatticeStory(id: id, bitmap: bm, tokenCount: count, typeWeights: weights, newTypeIDs: [])
        }
    }

    // MARK: - Program build & ordering

    func testProgramGroupsSetsInFirstSeenOrder() {
        let p = program()
        XCTAssertEqual(p.sets.map(\.id), ["animals", "food"])
        XCTAssertEqual(p.sets[0].name, "Animals")
        XCTAssertEqual(p.sets[0].cards.count, 4)
        XCTAssertEqual(p.allCards.count, 8)
    }

    // MARK: - Distractors (FR-11.3)

    func testDistractorsDrawnOnlyFromTaughtPredecessors() {
        let p = program()
        let animals = p.sets[0]
        // First card has no predecessors → no distractors.
        XCTAssertTrue(FoundationsDeck.distractors(for: animals.cards[0], in: animals).isEmpty)
        // Third card can only draw from cards 1 and 2 (already taught in this set).
        let d = FoundationsDeck.distractors(for: animals.cards[2], in: animals, count: 3)
        XCTAssertTrue(Set(d).isSubset(of: [1, 2]))
        XCTAssertFalse(d.contains(3)) // never the target
        XCTAssertFalse(d.contains(4)) // never a not-yet-taught word
    }

    func testDistractorsNeverRepeatSameSetTwiceInARow() {
        // Property: for a set larger than count, adjacent cards must not draw the identical
        // distractor set (FR-11.3 "never same minimal pair twice in a row").
        var words: [WordRecord] = (1...10).map { WordRecord(id: $0, simp: "词\($0)", pinyin: "cí", hsk3Level: 1, freqRank: $0) }
        let lex = LexiconStore(words)
        let cards = (1...10).map { fc($0, "big") }
        let set = FoundationsProgram(cards: cards, lexicon: lex).sets[0]
        var prev: [Int]? = nil
        for card in set.cards.dropFirst(4) { // once the pool exceeds `count`
            let d = FoundationsDeck.distractors(for: card, in: set, count: 3)
            XCTAssertEqual(d.count, 3)
            if let prev { XCTAssertNotEqual(prev, d) }
            prev = d
        }
        _ = words
    }

    func testDistractorsDeterministic() {
        let p = program()
        let s = p.sets[1]
        XCTAssertEqual(FoundationsDeck.distractors(for: s.cards.last!, in: s, count: 2),
                       FoundationsDeck.distractors(for: s.cards.last!, in: s, count: 2))
    }

    // MARK: - Session (FR-11.4)

    func testSessionEndsOnRecombinationNeverOnIsolatedCard() {
        var session = FoundationsSession(set: program().sets[0])
        XCTAssertFalse(session.isComplete)
        // Drive all four cards through the four-step cycle.
        var guardCount = 0
        while session.currentCard != nil {
            session.advance(correct: true, at: t0)
            guardCount += 1
            XCTAssertLessThan(guardCount, 100)
        }
        // After the last card's bind we are at recombination — NOT done yet.
        XCTAssertEqual(session.phase, .recombination)
        XCTAssertFalse(session.isComplete)
        // The recombination pass completes the session.
        session.advance(correct: true, at: t0)
        XCTAssertTrue(session.isComplete)
        XCTAssertTrue(session.endedOnRecombination)
    }

    func testWrongAnswerRepeatsStepAndLogsLookup() {
        var session = FoundationsSession(set: program().sets[0])
        _ = session.advance(at: t0) // introduce → recognize
        XCTAssertEqual(session.currentStage, .recognize)
        let events = session.advance(correct: false, at: t0)
        XCTAssertEqual(events.first?.kind, .lookup)
        XCTAssertEqual(session.currentStage, .recognize) // stayed put
    }

    // MARK: - Folds into the one KnownWordModel (I5)

    func testInteractionsFoldIntoKnownWordModelAndReachKnown() {
        var session = FoundationsSession(set: program().sets[0])
        var model = KnownWordModel()
        while !session.isComplete {
            for e in session.advance(correct: true, at: t0) { model.apply(e) }
        }
        // Every word that completed its bind step is in the known set.
        for id in program().sets[0].wordIDs {
            XCTAssertGreaterThanOrEqual(model.pKnown(id), KnownWordModel.knownThreshold, "word \(id)")
        }
    }

    // MARK: - startingSet (FR-11.6)

    func testStartingSetForBeginnerIsFirstSet() {
        let p = program()
        XCTAssertEqual(p.startingSetIndex(for: .empty), 0)
    }

    func testStartingSetSkipsMasteredSets() {
        let p = program()
        // Learner already knows all of set 0 (animals: 1–4).
        let seed = PlacementSeed([1: 0.95, 2: 0.95, 3: 0.95, 4: 0.95])
        XCTAssertEqual(p.startingSetIndex(for: seed), 1)
        XCTAssertEqual(p.startingSet(for: seed)?.id, "food")
    }

    func testStartingSetPastEndWhenAllMastered() {
        let p = program()
        var priors: [Int: Double] = [:]
        for id in 1...8 { priors[id] = 0.95 }
        XCTAssertEqual(p.startingSetIndex(for: PlacementSeed(priors)), p.sets.count)
        XCTAssertNil(p.startingSet(for: PlacementSeed(priors)))
    }

    // MARK: - Handoff gate (F3, reuses the I1 coverage formula)

    func testHandoffFiresAtThreshold() {
        let maxID = 5
        // 20 stories each fully covered by known words {1,2}. One story needs word 3.
        var stories: [(String, [Int: Int])] = []
        for i in 0..<20 { stories.append(("s\(i)", [1: 5, 2: 5])) }
        stories.append(("needs3", [1: 5, 3: 5])) // only 50% covered by {1,2}
        let a1 = lattice(stories, maxWordID: maxID)
        let gate = HandoffGate()

        XCTAssertEqual(gate.storiesGated(known: [1, 2], a1Stories: a1), 20)
        XCTAssertTrue(gate.isReady(known: [1, 2], a1Stories: a1))
        // Below threshold: knowing only word 1 covers nothing at ≥98%.
        XCTAssertFalse(gate.isReady(known: [1], a1Stories: a1))
    }

    func testHandoffStatusOverModel() {
        let maxID = 3
        let a1 = lattice((0..<25).map { ("s\($0)", [1: 10]) }, maxWordID: maxID)
        var model = KnownWordModel()
        model.apply(.markKnown(1, at: t0))
        let status = HandoffGate().status(model: model, a1Stories: a1)
        XCTAssertEqual(status.storiesGated, 25)
        XCTAssertTrue(status.ready)
        XCTAssertEqual(status.threshold, 20)
    }
}
