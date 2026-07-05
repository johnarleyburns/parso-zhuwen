import XCTest
import ZhuwenPacks
@testable import ZhuwenCore

final class PlacementTests: XCTestCase {
    private let t0 = Date(timeIntervalSince1970: 1_700_000_000)

    // MARK: - Item builder (FR-1.1)

    func testBuilderProducesRequestedCountWithFoilFraction() {
        let lex = PlacementFixtures.syntheticLexicon()
        let items = PlacementItemBuilder(itemCount: 90, foilFraction: 0.2, seed: 1).build(lexicon: lex)
        XCTAssertEqual(items.count, 90)
        let foils = items.filter { $0.isFoil }.count
        XCTAssertEqual(Double(foils), 18, accuracy: 2) // ~20%
    }

    func testBuilderStratifiesAcrossHSKLevels() {
        let lex = PlacementFixtures.syntheticLexicon()
        let items = PlacementItemBuilder(itemCount: 90, seed: 1).build(lexicon: lex)
        let levels = Set(items.filter { !$0.isFoil }.map { $0.hsk3Level })
        XCTAssertGreaterThanOrEqual(levels.count, 5, "items should span the HSK strata")
    }

    func testBuilderIsDeterministic() {
        let lex = PlacementFixtures.syntheticLexicon()
        let a = PlacementItemBuilder(itemCount: 80, seed: 5).build(lexicon: lex)
        let b = PlacementItemBuilder(itemCount: 80, seed: 5).build(lexicon: lex)
        XCTAssertEqual(a, b)
    }

    func testBuilderClampsItemCountToRange() {
        XCTAssertEqual(PlacementItemBuilder(itemCount: 10).itemCount, 60)
        XCTAssertEqual(PlacementItemBuilder(itemCount: 500).itemCount, 120)
    }

    // MARK: - Session state machine (M1–M3)

    func testSessionAdvancesThroughItemsToResult() {
        let items = (1...3).map { PlacementItem(surface: "w\($0)", wordID: $0, freqRank: $0 * 10, hsk3Level: 1) }
        var s = PlacementSession(items: items)
        XCTAssertEqual(s.phase, .welcome)
        s.begin()
        XCTAssertEqual(s.phase, .wordCheck)
        XCTAssertEqual(s.currentItem?.wordID, 1)
        s.answer(known: true)
        s.answer(known: false)
        XCTAssertEqual(s.answeredCount, 2)
        XCTAssertEqual(s.currentItem?.wordID, 3)
        s.answer(known: true)
        XCTAssertEqual(s.phase, .result)
        XCTAssertNil(s.currentItem)
        XCTAssertEqual(s.answers, [true, false, true])
    }

    func testBeginnerSkipSeedsEmptyFoundations() {
        // FR-1.4: absolute-beginner path skips the test; model seeds empty → Foundations.
        var s = PlacementSession(items: [PlacementItem(surface: "w", wordID: 1, freqRank: 5, hsk3Level: 1)])
        s.skipAsBeginner()
        XCTAssertTrue(s.beginner)
        XCTAssertEqual(s.phase, .result)

        let r = PlacementResult.beginner
        XCTAssertTrue(r.seed.isEmpty)
        XCTAssertEqual(r.route, .foundations)
        XCTAssertEqual(r.hskLevel, 0)
        XCTAssertEqual(r.cefr, .a0)
        XCTAssertTrue(KnownWordModel.seeded(r.seed).states.isEmpty)
    }

    // MARK: - Estimator (FR-1.2)

    func testAllYesPlacesHighAllNoPlacesLow() {
        let lex = PlacementFixtures.syntheticLexicon()
        let items = PlacementItemBuilder(itemCount: 100, foilFraction: 0, seed: 3).build(lexicon: lex)
        let est = PlacementEstimator()

        let high = est.estimate(items: items, answers: Array(repeating: true, count: items.count), lexicon: lex)
        let low = est.estimate(items: items, answers: Array(repeating: false, count: items.count), lexicon: lex)

        XCTAssertGreaterThan(high.estimatedKnownCount, low.estimatedKnownCount)
        XCTAssertGreaterThan(high.hskLevel, low.hskLevel)
        XCTAssertEqual(low.route, .foundations) // knows ~nothing
        XCTAssertEqual(high.hskLevel, 6)
    }

    func testGuessingCorrectionLowersInflatedTail() {
        // A learner who knows common words but only half-recognizes the rare tail. Answering
        // "yes" to the foils reveals that tail confidence is partly guessing, so the guessing
        // correction pulls the estimated known count down versus the same answers with no foils.
        let ranks = stride(from: 1, through: 6000, by: 100).map { $0 }
        let real = ranks.map { PlacementItem(surface: "w\($0)", wordID: $0, freqRank: $0, hsk3Level: 1) }
        let realAnswers = ranks.map { $0 <= 2500 || ($0 / 100) % 2 == 0 } // yes low, ~half in tail
        let foils = (0..<20).map { PlacementItem(surface: "f\($0)", wordID: nil, freqRank: 0, hsk3Level: 0) }
        let words = ranks.map { WordRecord(id: $0, simp: "w\($0)", pinyin: "x", hsk3Level: 1, freqRank: $0) }
        let est = PlacementEstimator()

        let honest = est.estimate(items: real, answers: realAnswers, words: words)
        let inflated = est.estimate(items: real + foils,
                                    answers: realAnswers + Array(repeating: true, count: foils.count), words: words)

        XCTAssertGreaterThan(inflated.falseAlarmRate, 0.5)
        XCTAssertLessThan(inflated.estimatedKnownCount, honest.estimatedKnownCount)
    }

    func testGuessingCorrectionLowersMidProbabilityRank() {
        // Direct check on the curve: a nonzero false-alarm rate discounts P(known) where the
        // raw yes-rate is uncertain, and cannot make it negative.
        let base = LogisticCurve(intercept: 0, slope: -1, mean: 0, std: 1, falseAlarm: 0, conservatism: 1)
        let guessy = LogisticCurve(intercept: 0, slope: -1, mean: 0, std: 1, falseAlarm: 0.4, conservatism: 1)
        XCTAssertLessThan(guessy.pKnown(rank: 100), base.pKnown(rank: 100))
        XCTAssertGreaterThanOrEqual(guessy.pKnown(rank: 100_000), 0)
    }

    // MARK: - Seed feeds the known-word model (FR-1.2)

    func testSeedFeedsKnownWordModel() {
        let lex = PlacementFixtures.syntheticLexicon()
        let items = PlacementItemBuilder(itemCount: 100, foilFraction: 0.1, seed: 8).build(lexicon: lex)
        // A learner who knows common words (rank small) and not rare ones.
        let answers = items.map { $0.isFoil ? false : $0.freqRank < 1500 }
        let result = PlacementEstimator().estimate(items: items, answers: answers, lexicon: lex)

        let model = KnownWordModel.seeded(result.seed)
        let known = model.effectiveKnownSet()
        XCTAssertFalse(known.isEmpty)
        XCTAssertGreaterThan(model.pKnown(1), model.pKnown(5500)) // common > rare
        XCTAssertTrue(known.contains(1))       // a very common word is seeded known
        XCTAssertFalse(known.contains(5900))   // a very rare word is not
    }

    // MARK: - Re-placement merge (FR-1.5)

    func testReplacementMergeNeverLowersPKnown() {
        let events: [Event] = [
            .exposure(1, at: t0), .exposure(2, at: t0.addingTimeInterval(1)),
            .lookup(3, at: t0.addingTimeInterval(2)),
        ]
        let first = PlacementSeed([1: 0.9, 2: 0.4, 3: 0.7, 4: 0.85])
        let second = PlacementSeed([2: 0.6, 3: 0.2, 5: 0.9]) // a re-placement result

        let before = KnownWordModel.seeded(first, events: events)
        let merged = first.merged(with: second)
        let after = KnownWordModel.seeded(merged, events: events)

        // Merge takes the max prior; re-projecting the same log never lowers any word's P(known).
        let ids = Set(first.priors.keys).union(second.priors.keys).union([1, 2, 3, 4, 5])
        for id in ids {
            XCTAssertGreaterThanOrEqual(after.pKnown(id) + 1e-9, before.pKnown(id),
                                        "re-placement lowered P(known) for word \(id)")
        }
        // Merge kept the higher of each prior.
        XCTAssertEqual(merged.priors[2] ?? 0, 0.6, accuracy: 1e-9)
        XCTAssertEqual(merged.priors[3] ?? 0, 0.7, accuracy: 1e-9)
        XCTAssertEqual(merged.priors[5] ?? 0, 0.9, accuracy: 1e-9)
    }

    // MARK: - Reading-passage refinement (FR-1.3)

    func testPassageRefinementCatchesOverclaim() {
        let lex = PlacementFixtures.syntheticLexicon()
        let items = PlacementItemBuilder(itemCount: 100, foilFraction: 0.1, seed: 2).build(lexicon: lex)
        let answers = items.map { $0.isFoil ? false : $0.freqRank < 2500 }
        let est = PlacementEstimator()
        let base = est.estimate(items: items, answers: answers, lexicon: lex)

        let failed = est.refine(base, passages: [PassageOutcome(correct: 0, total: 3),
                                                 PassageOutcome(correct: 1, total: 3)], lexicon: lex)
        let aced = est.refine(base, passages: [PassageOutcome(correct: 3, total: 3),
                                               PassageOutcome(correct: 3, total: 3)], lexicon: lex)

        // Poor reading comprehension (strong spoken vocab) must not raise the estimate…
        XCTAssertLessThanOrEqual(failed.estimatedKnownCount, base.estimatedKnownCount)
        XCTAssertLessThan(failed.estimatedKnownCount, aced.estimatedKnownCount)
        // …and acing it must not lower it.
        XCTAssertGreaterThanOrEqual(aced.estimatedKnownCount, base.estimatedKnownCount)
    }
}
