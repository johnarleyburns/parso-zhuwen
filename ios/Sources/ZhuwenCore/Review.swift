import Foundation
import ZhuwenPacks

/// One sentence-context review card (M9, FR-7.1): the target word shown inside a sentence the
/// learner actually read, with source attribution and the projected FSRS intervals for each grade.
public struct ReviewCard: Equatable, Identifiable {
    public let wordID: Int
    public let simp: String
    public let pinyin: String
    public let hsk3Level: Int
    /// The context sentence tokens (the target word is `isTarget`), in reading order.
    public let sentence: [SentenceToken]
    public let storyID: String
    public let storyTitleZH: String
    public let due: Date
    /// Projected next interval in days per grade (Again/Hard/Good/Easy) for the buttons.
    public let intervals: [Rating: Int]

    public var id: Int { wordID }

    /// The context sentence as plain text (target word included).
    public var sentenceText: String { sentence.map(\.text).joined() }

    public struct SentenceToken: Equatable {
        public let text: String
        public let isTarget: Bool
        public init(text: String, isTarget: Bool) { self.text = text; self.isTarget = isTarget }
    }
}

/// Builds the daily review queue from the known-word model's FSRS state (FR-7.1/7.2). Cards are
/// **sentence-context only** — the word must appear in a story the learner has read — and the queue
/// is capped (default 20/day) and ordered soonest-due-first. Pure and host-testable.
public struct ReviewScheduler {
    /// Default daily cap (FR-7.2): review is optional maintenance, not the primary SRS mechanism.
    public static let defaultDailyCap = 20

    public var dailyCap: Int
    private let scheduler: FSRSScheduler

    public init(dailyCap: Int = ReviewScheduler.defaultDailyCap,
                scheduler: FSRSScheduler = .default) {
        self.dailyCap = max(1, dailyCap)
        self.scheduler = scheduler
    }

    /// The due cards at `now`. `readStoryIDs` are the stories the learner has read (their sentences
    /// are eligible for context); words with no usable sentence in a read story are skipped.
    public func dueCards(model: KnownWordModel, stories: [StoryRecord], lexicon: [WordRecord],
                         readStoryIDs: Set<String>, now: Date) -> [ReviewCard] {
        let byID = Dictionary(lexicon.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        let readStories = stories.filter { readStoryIDs.contains($0.id) }
        var out: [ReviewCard] = []

        for wordID in model.dueWordIDs(at: now) {
            guard let word = byID[wordID],
                  let (story, sentence) = context(for: wordID, in: readStories, lexicon: byID)
            else { continue }
            let card = model.state(for: wordID).fsrs
            out.append(ReviewCard(
                wordID: wordID, simp: word.simp, pinyin: word.pinyin, hsk3Level: word.hsk3Level,
                sentence: sentence, storyID: story.id, storyTitleZH: story.titleZH,
                due: card?.due ?? now, intervals: scheduler.intervals(card, at: now)))
            if out.count >= dailyCap { break }
        }
        return out
    }

    /// Number of cards due at `now` (before the cap), for the "N due" badge.
    public func dueCount(model: KnownWordModel, now: Date) -> Int {
        model.dueWordIDs(at: now).count
    }

    // Finds a sentence in a read story that contains the target word, returns its rendered tokens.
    private func context(for wordID: Int, in stories: [StoryRecord],
                         lexicon: [Int: WordRecord]) -> (StoryRecord, [ReviewCard.SentenceToken])? {
        for story in stories {
            guard let sentenceIdx = story.body.first(where: { $0.w == wordID })?.s else { continue }
            let tokens = story.body.filter { $0.s == sentenceIdx }.map { t -> ReviewCard.SentenceToken in
                let text: String
                if t.w >= 0 { text = lexicon[t.w]?.simp ?? (t.lit ?? "") }
                else { text = t.lit ?? "" }
                return ReviewCard.SentenceToken(text: text, isTarget: t.w == wordID)
            }
            if !tokens.isEmpty { return (story, tokens) }
        }
        return nil
    }
}
