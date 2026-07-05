import Foundation
import SwiftUI
import ZhuwenCore
import ZhuwenPacks

/// LearnerModel owns the append-only event log and the placement seed, re-projects the
/// `KnownWordModel` (+ FSRS memory) on every append, and vends the comprehension check (M8), the
/// review queue (M9), and the progress report (M10) to the loop-completion screens (handoff §5,
/// I5). It is the on-device "loop" glue; SwiftData persistence is assembled with the `@main` target
/// (the log it stores is exactly `events`).
@MainActor
public final class LearnerModel: ObservableObject {
    @Published public private(set) var model: KnownWordModel
    @Published public private(set) var events: [Event]
    @Published public private(set) var readStoryIDs: Set<String>
    @Published public private(set) var sealedStoryIDs: Set<String>

    private let stories: [StoryRecord]
    private let lexicon: [WordRecord]
    private let seed: [Int: Double]
    private let reviewScheduler = ReviewScheduler()
    private let progressEstimator = ProgressEstimator()
    private let questionsProvider: (String) -> [QuestionRecord]

    public init(stories: [StoryRecord], lexicon: [WordRecord],
                seed: PlacementSeed = .empty, events: [Event] = [],
                questions: @escaping (String) -> [QuestionRecord] = { _ in [] }) {
        self.stories = stories
        self.lexicon = lexicon
        self.seed = seed.priors
        self.events = events
        self.questionsProvider = questions
        self.model = KnownWordModel.project(events, seed: seed.priors)
        self.readStoryIDs = Set(events.compactMap { $0.storyID })
        self.sealedStoryIDs = []
    }

    /// Append one event and re-project (I5: state is a pure function of the log).
    public func record(_ event: Event) {
        events.append(event)
        model.apply(event)
        if let sid = event.storyID { readStoryIDs.insert(sid) }
    }

    private func record(_ newEvents: [Event]) {
        for e in newEvents { record(e) }
    }

    // MARK: - Reading (feeds the loop)

    /// Log a word exposure (word read, not tapped — weak positive evidence, FR-2.2).
    public func exposure(_ wordID: Int, storyID: String, at now: Date = Date()) {
        record(.exposure(wordID, storyID: storyID, at: now))
    }

    /// Log a dictionary lookup (evidence of not knowing, FR-2.2/FR-4.6).
    public func lookup(_ wordID: Int, storyID: String, at now: Date = Date()) {
        record(.lookup(wordID, storyID: storyID, at: now))
    }

    // MARK: - M8 Comprehension → seal

    public func comprehensionSession(for story: StoryRecord) -> ComprehensionSession {
        ComprehensionSession(
            storyID: story.id,
            questions: questionsProvider(story.id),
            exposedWordIDs: ComprehensionSession.exposedWordIDs(of: story))
    }

    /// Complete a comprehension check: appends its events (FR-6.2) and stamps the seal on a pass.
    public func completeComprehension(_ session: ComprehensionSession, at now: Date = Date()) {
        record(session.completionEvents(at: now))
        if session.sealEarned { sealedStoryIDs.insert(session.storyID) }
    }

    public func isSealed(_ storyID: String) -> Bool { sealedStoryIDs.contains(storyID) }

    // MARK: - M9 Review (FSRS)

    public func reviewQueue(now: Date = Date()) -> [ReviewCard] {
        reviewScheduler.dueCards(model: model, stories: stories, lexicon: lexicon,
                                 readStoryIDs: readStoryIDs, now: now)
    }

    public func dueCount(now: Date = Date()) -> Int {
        reviewScheduler.dueCount(model: model, now: now)
    }

    /// Grade a review card (FR-7.3): appends a `.reviewGrade` that advances the card's FSRS memory.
    public func grade(_ card: ReviewCard, _ rating: Rating, at now: Date = Date()) {
        record(.reviewGrade(card.wordID, grade: rating.rawValue, at: now))
    }

    // MARK: - M10 Progress

    public func progress(now: Date = Date()) -> ProgressReport {
        progressEstimator.report(model: model, events: events, lexicon: lexicon,
                                 seed: seed, now: now)
    }
}
