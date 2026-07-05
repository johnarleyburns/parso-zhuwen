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
    public let manifest: PackManifest?
    public let learner: LearnerModel
    public let store: StoreModel
    public let sync: SyncModel
    public let packManager: PackManagerModel
    @Published public var settings: LearnerSettings
    private let packStore: PackStore?

    public init(store packStore: PackStore,
                commerce: StoreModel? = nil,
                sync: SyncModel? = nil,
                packManager: PackManagerModel? = nil,
                settings: LearnerSettings = .default) {
        self.packStore = packStore
        self.manifest = packStore.manifest
        let stories = (try? packStore.stories()) ?? []
        let lexicon = (try? packStore.lexicon()) ?? []
        self.stories = stories
        self.lexicon = lexicon
        self.learner = LearnerModel(
            stories: stories, lexicon: lexicon,
            questions: { [weak packStore] id in (try? packStore?.questions(for: id)) ?? [] })
        self.store = commerce ?? StoreModel()
        self.sync = sync ?? SyncModel()
        self.packManager = packManager ?? PackManagerModel()
        self.settings = settings
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

    /// The settings screen (M13), wired to the shared commerce/sync/pack/learner state.
    public func makeSettingsView() -> SettingsView {
        SettingsView(learner: learner, store: store, sync: sync,
                     packs: packManager, settings: settingsBinding)
    }

    private var settingsBinding: Binding<LearnerSettings> {
        Binding(get: { self.settings }, set: { self.settings = $0 })
    }
}
