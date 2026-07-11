import Foundation
import SwiftUI
import ZhuwenAudio
import ZhuwenCore
import ZhuwenPacks

/// AppModel loads a verified pack and exposes its stories/lexicon to the SwiftUI shell (handoff §5).
/// It also owns the commerce (`StoreModel`), optional-sync (`SyncModel`), pack-manager, and settings
/// state added at CP-08 — the loop state itself lives in `LearnerModel`.
@MainActor
public final class AppModel: ObservableObject {
    @Published public private(set) var stories: [StoryRecord] = []
    @Published public private(set) var lexicon: [WordRecord] = []
    /// Where first-run onboarding + the Foundations gate route the learner (FR-1.4/11.5/11.6).
    @Published public private(set) var onboardingRoute: OnboardingRoute
    public let manifest: PackManifest?
    public let learner: LearnerModel
    public let store: StoreModel
    public let sync: SyncModel
    public let packManager: PackManagerModel
    @Published public var settings: LearnerSettings
    private let packStore: PackStore?
    private let placementStore: PlacementStore?
    private let foundationsCards: [FoundationsCardRecord]
    private let images: [ImageRecord]

    /// The three first-run/loop states (plan Part C point 14 + FR-11.5). Onboarding auto-presents
    /// placement when no result is persisted; complete/partial beginners route to Foundations at
    /// their first unmastered set; the regular loop activates at the F3 handoff.
    public enum OnboardingRoute: Equatable {
        case needsPlacement
        case foundations
        case loop
    }

    public init(store packStore: PackStore,
                events: [Event] = [],
                eventSink: EventSink? = nil,
                placementStore: PlacementStore? = nil,
                forceRoute: OnboardingRoute? = nil,
                commerce: StoreModel? = nil,
                sync: SyncModel? = nil,
                packManager: PackManagerModel? = nil,
                settings: LearnerSettings = .default) {
        self.packStore = packStore
        self.placementStore = placementStore
        self.manifest = packStore.manifest
        let stories = (try? packStore.stories()) ?? []
        let lexicon = (try? packStore.lexicon()) ?? []
        self.stories = stories
        self.lexicon = lexicon
        self.foundationsCards = (try? packStore.foundationsCards()) ?? []
        self.images = (try? packStore.images()) ?? []
        let snapshot = placementStore?.load()
        self.learner = LearnerModel(
            stories: stories, lexicon: lexicon,
            seed: snapshot?.seed ?? .empty,
            events: events, sink: eventSink,
            questions: { [weak packStore] id in (try? packStore?.questions(for: id)) ?? [] })
        self.store = commerce ?? StoreModel()
        self.sync = sync ?? SyncModel()
        self.packManager = packManager ?? PackManagerModel()
        self.settings = settings
        self.onboardingRoute = forceRoute ?? AppModel.route(for: snapshot, cards: self.foundationsCards,
                                                            lexicon: lexicon, learner: self.learner)
    }

    /// Convenience: verify + open a pack from disk, then load it.
    public convenience init(packURL: URL, publicKeyFile: URL) throws {
        let pubText = try String(contentsOf: publicKeyFile, encoding: .utf8)
        let pub = try Minisign.PublicKey(file: pubText)
        let store = try PackStore(url: packURL, publicKey: pub)
        self.init(store: store)
    }

    public func readerModel(for story: StoryRecord) -> ReaderModel {
        ReaderModel(story: story, lexicon: lexicon)
    }

    /// The comprehension check (M8) for a story, wired to the shared learner log.
    public func makeComprehensionView(for story: StoryRecord,
                                      onDone: @escaping () -> Void = {}) -> ComprehensionView {
        ComprehensionView(learner: learner, story: story, onDone: onDone)
    }

    /// A listening flow for a story (M7, FR-5): karaoke over pack alignment, system-TTS fallback
    /// when the pack has no (decodable) audio.
    public func makeListeningModel(for story: StoryRecord, blind: Bool = false) -> ListeningModel {
        let reader = readerModel(for: story)
        let alignment = (try? packStore?.alignment(storyID: story.id)) ?? []
        let audioURL: URL? = packStore.flatMap { try? $0.audioURL(for: story) } ?? nil
        return ListeningModel(story: story, tokens: reader.tokens(),
                              alignment: alignment, audioURL: audioURL, blind: blind)
    }

    /// A fresh placement flow over the loaded pack lexicon (M1–M3; repeatable, FR-1.5).
    public func makePlacementFlow() -> PlacementFlowModel {
        PlacementFlowModel(lexicon: lexicon)
    }

    // MARK: - Foundations (FR-11) + onboarding routing

    /// The ordered Foundations program built from the pack's `foundations_card` rows.
    public var foundationsProgram: FoundationsProgram {
        FoundationsProgram(cards: foundationsCards, lexicon: LexiconStore(lexicon))
    }

    /// The A1 story lattice the F3 handoff gate measures against (I1 coverage of ≥20 A1 stories).
    public var a1Lattice: [LatticeStory] {
        let maxWordID = lexicon.map(\.id).max() ?? 0
        return stories
            .filter { $0.hsk3Level <= 1 || $0.band.uppercased().contains("A1") }
            .map { LatticeStory(record: $0, maxWordID: maxWordID) }
    }

    /// The Foundations course model, seeded to resume a partial beginner at their first
    /// unmastered set (FR-11.6).
    public func makeFoundationsModel() -> FoundationsModel {
        let seed = PlacementSeed(learner.model.states.compactMapValues { $0.pKnown > 0 ? $0.pKnown : nil })
        return FoundationsModel(
            program: foundationsProgram,
            learner: learner,
            images: images,
            imageData: { [weak packStore] rec in packStore?.imageData(for: rec) },
            a1Stories: a1Lattice,
            startAt: seed)
    }

    /// The image record + bytes for a story's cover, or nil if no cover is available.
    public func coverImage(for story: StoryRecord) -> (record: ImageRecord?, data: Data?) {
        let rec = images.first { $0.id == story.coverImageID }
        return (rec, rec.flatMap { packStore?.imageData(for: $0) })
    }

    /// Persist a completed placement (FR-1.2/1.4), re-seed the model (FR-1.5 merge), and route the
    /// learner: absolute/partial beginners → Foundations; otherwise the regular loop.
    public func completePlacement(_ result: PlacementResult) {
        learner.applyPlacement(result.seed)
        placementStore?.save(PlacementSnapshot(result: result))
        onboardingRoute = (result.route == .foundations) ? .foundations : .loop
        advancePastFoundationsIfReady()
    }

    /// Called when the F3 handoff fires (FR-11.5): the regular Today/lattice loop activates.
    public func finishHandoff() { onboardingRoute = .loop }

    /// If the learner already gates enough A1 stories (e.g. after a re-placement), skip Foundations.
    private func advancePastFoundationsIfReady() {
        guard onboardingRoute == .foundations else { return }
        if HandoffGate().isReady(known: learner.model.effectiveKnownSet(), a1Stories: a1Lattice) {
            onboardingRoute = .loop
        }
    }

    private static func route(for snapshot: PlacementSnapshot?, cards: [FoundationsCardRecord],
                              lexicon: [WordRecord], learner: LearnerModel) -> OnboardingRoute {
        guard let snapshot else { return .needsPlacement }
        if snapshot.placementRoute == .foundations && !cards.isEmpty { return .foundations }
        return .loop
    }

    /// The settings screen (M13), wired to the shared commerce/sync/pack/learner state. Also
    /// exposes the manual re-run placement affordance (FR-1.5), the methodology page (I4), and the
    /// image credits (FR-11.2).
    public func makeSettingsView() -> SettingsView {
        SettingsView(learner: learner, store: store, sync: sync,
                     packs: packManager, settings: settingsBinding,
                     placementFlow: { [weak self] in self?.makePlacementFlow() ?? PlacementFlowModel(lexicon: []) },
                     onPlacementComplete: { [weak self] result in self?.completePlacement(result) },
                     creditsImages: images)
    }

    private var settingsBinding: Binding<LearnerSettings> {
        Binding(get: { self.settings }, set: { self.settings = $0 })
    }
}
