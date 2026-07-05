import Foundation
import SwiftUI
import ZhuwenAudio
import ZhuwenCore
import ZhuwenPacks

/// AppModel loads a verified pack and exposes its stories/lexicon to the SwiftUI shell
/// (handoff §5). It holds no learner state yet — KnownWordModel/Selector arrive in CP-04.
@MainActor
public final class AppModel: ObservableObject {
    @Published public private(set) var stories: [StoryRecord] = []
    @Published public private(set) var lexicon: [WordRecord] = []
    public let manifest: PackManifest?
    public let learner: LearnerModel
    private let store: PackStore?

    public init(store: PackStore) {
        self.store = store
        self.manifest = store.manifest
        let stories = (try? store.stories()) ?? []
        let lexicon = (try? store.lexicon()) ?? []
        self.stories = stories
        self.lexicon = lexicon
        self.learner = LearnerModel(
            stories: stories, lexicon: lexicon,
            questions: { [weak store] id in (try? store?.questions(for: id)) ?? [] })
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
        let alignment = (try? store?.alignment(storyID: story.id)) ?? []
        let audioURL: URL? = store.flatMap { try? $0.audioURL(for: story) } ?? nil
        return ListeningModel(story: story, tokens: reader.tokens(),
                              alignment: alignment, audioURL: audioURL, blind: blind)
    }

    /// A fresh placement flow over the loaded pack lexicon (M1–M3; repeatable, FR-1.5).
    public func makePlacementFlow() -> PlacementFlowModel {
        PlacementFlowModel(lexicon: lexicon)
    }
}
