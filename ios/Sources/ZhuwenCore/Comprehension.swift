import Foundation
import ZhuwenPacks

/// The comprehension check that closes the reading loop (M8, FR-6.1/6.2). Three factory-gated MC
/// questions; passing (≥2/3) stamps the story with a seal and boosts P(known) for every word the
/// story exposed. Pure value type so the UI (`LearnerModel`) and tests drive identical logic.
public struct ComprehensionSession: Equatable {
    /// Passing threshold: at least 2 of 3 correct (FR-6.2).
    public static let passThreshold = 2

    public let storyID: String
    public let questions: [QuestionRecord]
    /// Distinct content word IDs the story exposed (from its body); the P(known) boost targets these.
    public let exposedWordIDs: [Int]
    public private(set) var answers: [Int]  // chosen option index per question, in order
    public private(set) var index: Int

    public init(storyID: String, questions: [QuestionRecord], exposedWordIDs: [Int]) {
        self.storyID = storyID
        self.questions = questions
        self.exposedWordIDs = exposedWordIDs
        self.answers = []
        self.index = 0
    }

    public var currentQuestion: QuestionRecord? { index < questions.count ? questions[index] : nil }
    public var total: Int { questions.count }
    public var answeredCount: Int { answers.count }
    public var isComplete: Bool { !questions.isEmpty && index >= questions.count }

    /// Number correct so far.
    public var correctCount: Int {
        zip(answers, questions).reduce(0) { $0 + ($1.0 == $1.1.answerIdx ? 1 : 0) }
    }

    /// Whether the current run passed (only meaningful once complete). Empty question sets never pass.
    public var passed: Bool { isComplete && correctCount >= min(Self.passThreshold, total) && total > 0 }

    /// The seal (读完为证) is earned exactly when the learner passes (FR-6.2).
    public var sealEarned: Bool { passed }

    /// Answer the current question with a chosen option index; advances.
    public mutating func answer(optionIndex: Int) {
        guard index < questions.count else { return }
        answers.append(optionIndex)
        index += 1
    }

    /// The events to append when the check completes (I5). On a pass, every exposed word gets a
    /// positive `.comprehension` event (FR-6.2 "boosts P(known) for its words"); a fail records the
    /// negative outcome so the model stays honest. Nothing is emitted mid-session.
    public func completionEvents(at now: Date) -> [Event] {
        guard isComplete else { return [] }
        let correct = passed
        return exposedWordIDs.map {
            .comprehension($0, correct: correct, storyID: storyID, at: now)
        }
    }
}

extension ComprehensionSession {
    /// Convenience: the distinct content word IDs of a story body (skips literals / proper nouns).
    public static func exposedWordIDs(of story: StoryRecord) -> [Int] {
        var seen = Set<Int>()
        var out: [Int] = []
        for t in story.body where t.w >= 0 && !(t.pn ?? false) {
            if seen.insert(t.w).inserted { out.append(t.w) }
        }
        return out
    }
}
