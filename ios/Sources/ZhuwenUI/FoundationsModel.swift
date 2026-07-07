import Foundation
import SwiftUI
import ZhuwenCore
import ZhuwenPacks
#if canImport(AVFoundation)
import AVFoundation
#endif

/// FoundationsModel drives the M14 picture-word course for SwiftUI. It owns the ordered
/// `FoundationsProgram`, the current `FoundationsSession`, and the F3 `HandoffGate`, and folds
/// every interaction into the shared `LearnerModel` event log (I5) — Foundations is not a
/// separate silo. Set-sequencing and the FR-11.6 re-entry point come straight from the engine.
@MainActor
public final class FoundationsModel: ObservableObject {
    @Published public private(set) var setIndex: Int
    @Published public private(set) var session: FoundationsSession
    @Published public private(set) var lastAnswerCorrect: Bool?

    public let program: FoundationsProgram
    private let learner: LearnerModel
    private let images: [String: ImageRecord]
    private let imageData: (ImageRecord) -> Data?
    private let a1Stories: [LatticeStory]
    private let handoff = HandoffGate()
    /// Speaks a hanzi string aloud on each interaction (system TTS for taps, FR-11 / §7).
    public var speak: (String) -> Void

    public init(program: FoundationsProgram,
                learner: LearnerModel,
                images: [ImageRecord] = [],
                imageData: @escaping (ImageRecord) -> Data? = { _ in nil },
                a1Stories: [LatticeStory] = [],
                startAt seed: PlacementSeed = .empty,
                speak: ((String) -> Void)? = nil) {
        self.program = program
        self.learner = learner
        self.images = Dictionary(images.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        self.imageData = imageData
        self.a1Stories = a1Stories
        let start = min(program.startingSetIndex(for: seed), max(0, program.sets.count - 1))
        self.setIndex = start
        self.session = FoundationsSession(cards: program.sets.isEmpty ? [] : program.sets[start].cards)
        self.speak = speak ?? FoundationsModel.systemSpeaker()
    }

    // MARK: - Presentation

    public var currentSet: FoundationsSet? {
        program.sets.indices.contains(setIndex) ? program.sets[setIndex] : nil
    }
    public var currentCard: FoundationsCard? { session.currentCard }
    public var currentStage: FoundationsStage? { session.currentStage }
    public var isSessionComplete: Bool { session.isComplete }

    /// The image record + bytes for a card's photo (nil bytes → the view shows a placeholder).
    public func image(for card: FoundationsCard) -> (record: ImageRecord?, data: Data?) {
        let rec = images[card.imageID]
        return (rec, rec.flatMap(imageData))
    }

    public func image(id: String) -> ImageRecord? { images[id] }

    /// The recognition-grid options for the current recognize/read step: the target plus its
    /// already-taught distractors, in a stable shuffled order (FR-11.3).
    public func gridOptions(for card: FoundationsCard) -> [FoundationsCard] {
        guard let set = currentSet else { return [card] }
        var ids = FoundationsDeck.distractors(for: card, in: set)
        if ids.isEmpty {
            // First card of the very first set: fall back to later cards as decoys so the grid
            // is never a single option (they are not yet "taught" but keep the UI sensible).
            ids = set.cards.filter { $0.wordID != card.wordID }.prefix(3).map(\.wordID)
        }
        let byID = Dictionary(program.allCards.map { ($0.wordID, $0) }, uniquingKeysWith: { a, _ in a })
        var options = ([card.wordID] + ids).compactMap { byID[$0] }
        options.sort { $0.wordID < $1.wordID }
        return options
    }

    /// Words known counter (from word #1, FR-11.5).
    public var wordsKnown: Int { learner.model.effectiveKnownSet().count }

    /// F3 handoff status over the A1 lattice, if one was supplied.
    public var handoffStatus: HandoffStatus {
        handoff.status(model: learner.model, a1Stories: a1Stories)
    }

    public var isHandoffReady: Bool { handoffStatus.ready }

    // MARK: - Interaction (folds into the one KnownWordModel, I5)

    /// Advance the four-step cycle. Speaks the current card, records the emitted events, and
    /// tracks the last answer for the grid's correct/incorrect feedback.
    public func advance(correct: Bool = true, at now: Date = Date()) {
        if let card = session.currentCard { speak(card.simp) }
        lastAnswerCorrect = session.currentStage == .introduce ? nil : correct
        let events = session.advance(correct: correct, at: now)
        for e in events { learner.record(e) }
    }

    /// Move to the next unmastered set after a session ends on its recombination pass (FR-11.4).
    public func startNextSet() {
        guard session.isComplete else { return }
        let next = setIndex + 1
        guard program.sets.indices.contains(next) else { return }
        setIndex = next
        session = FoundationsSession(cards: program.sets[next].cards)
        lastAnswerCorrect = nil
    }

    public var hasNextSet: Bool { program.sets.indices.contains(setIndex + 1) }

    // MARK: - Default speaker

    private static func systemSpeaker() -> (String) -> Void {
        #if canImport(AVFoundation)
        let synth = AVSpeechSynthesizer()
        return { text in
            guard !text.isEmpty else { return }
            let u = AVSpeechUtterance(string: text)
            u.voice = AVSpeechSynthesisVoice(language: "zh-CN")
            synth.speak(u)
        }
        #else
        return { _ in }
        #endif
    }
}
