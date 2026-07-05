import Foundation
import ZhuwenPacks

/// A gloss shown when a learner taps a word (handoff FR-4.1). CP-03 surfaces the pack
/// data available today (simplified form, tone pinyin, HSK level); richer definitions and
/// character breakdowns arrive with later lexicon columns.
public struct Gloss: Equatable, Identifiable {
    public let wordID: Int
    public let simp: String
    public let pinyin: String
    public let hsk3Level: Int
    public let freqRank: Int
    public var id: Int { wordID }
}

/// A renderable token in the reader.
public struct DisplayToken: Equatable, Identifiable {
    public let id: Int // position index, stable per render
    public let text: String
    public let wordID: Int? // nil for literal / proper noun
    public let pinyin: String?
    public let isProperNoun: Bool
    public let isFrontier: Bool // first-encounter frontier word (FR-4.2 underline)
    public let sentenceIndex: Int
}

/// ReaderModel turns a story's segmented body into display tokens and resolves taps to
/// glosses using the pack lexicon (handoff FR-4). Pure and fully testable off-device.
public final class ReaderModel {
    public let story: StoryRecord
    private let byID: [Int: WordRecord]
    private let frontier: Set<Int>

    public init(story: StoryRecord, lexicon: [WordRecord]) {
        self.story = story
        self.byID = Dictionary(lexicon.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        self.frontier = Set(story.newTypeIDs)
    }

    /// The displayable token stream in reading order.
    public func tokens() -> [DisplayToken] {
        var out: [DisplayToken] = []
        out.reserveCapacity(story.body.count)
        for (i, t) in story.body.enumerated() {
            if t.isProperNoun {
                out.append(DisplayToken(id: i, text: t.lit ?? "", wordID: nil, pinyin: nil,
                                        isProperNoun: true, isFrontier: false, sentenceIndex: t.s))
            } else if t.isLiteral {
                out.append(DisplayToken(id: i, text: t.lit ?? "", wordID: nil, pinyin: nil,
                                        isProperNoun: false, isFrontier: false, sentenceIndex: t.s))
            } else {
                let w = byID[t.w]
                out.append(DisplayToken(id: i, text: w?.simp ?? (t.lit ?? "?"), wordID: t.w,
                                        pinyin: w?.pinyin, isProperNoun: false,
                                        isFrontier: frontier.contains(t.w), sentenceIndex: t.s))
            }
        }
        return out
    }

    /// Resolve a tapped word to its gloss (nil for literals / proper nouns / unknown IDs).
    public func gloss(for wordID: Int) -> Gloss? {
        guard let w = byID[wordID] else { return nil }
        return Gloss(wordID: w.id, simp: w.simp, pinyin: w.pinyin, hsk3Level: w.hsk3Level, freqRank: w.freqRank)
    }

    /// Number of sentences in the story body.
    public func sentenceCount() -> Int {
        (story.body.map { $0.s }.max() ?? -1) + 1
    }
}
