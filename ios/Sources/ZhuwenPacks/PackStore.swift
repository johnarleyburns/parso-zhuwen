import Foundation

/// PackStore verifies a `.zpack`, extracts its `content.sqlite`, and exposes typed
/// queries (handoff §5). It owns pack I/O; higher layers never touch the zip or SQLite
/// directly. The extracted database file lives for the store's lifetime; story audio
/// (FR-5.1) is extracted lazily to temp files on first request and cleaned up on deinit.
public final class PackStore {
    public let manifest: PackManifest
    public let database: ContentDatabase
    private let archive: ZipArchive
    private let tempURL: URL
    private var audioURLs: [String: URL] = [:]

    /// Opens and fully verifies a pack at `url` against `publicKey`.
    public init(url: URL, publicKey: Minisign.PublicKey, knownLexiconVersions: Set<String> = []) throws {
        let archive = try ZipArchive(url: url)

        // Extract content.sqlite to a temp file so SQLite can open it by path.
        let temp = FileManager.default.temporaryDirectory
            .appendingPathComponent("zhuwen-\(UUID().uuidString).sqlite")
        var extractedDB: ContentDatabase?

        let manifest = try PackVerifier.verify(
            archive: archive,
            publicKey: publicKey,
            knownLexiconVersions: knownLexiconVersions,
            contentDatabase: { bytes in
                try bytes.write(to: temp)
                let db = try ContentDatabase(path: temp.path)
                extractedDB = db
                return db
            })

        guard let db = extractedDB else {
            throw PackVerifier.VerifyError.fileAbsent("content.sqlite")
        }
        self.manifest = manifest
        self.database = db
        self.archive = archive
        self.tempURL = temp
    }

    deinit {
        try? FileManager.default.removeItem(at: tempURL)
        for url in audioURLs.values { try? FileManager.default.removeItem(at: url) }
    }

    public func stories() throws -> [StoryRecord] { try database.stories() }
    public func lexicon() throws -> [WordRecord] { try database.lexicon() }

    /// The Foundations F0 cards shipped in this pack (FR-11). Empty if none.
    public func foundationsCards() throws -> [FoundationsCardRecord] {
        try database.foundationsCards()
    }

    /// Every provenanced image row (FR-11.2 attribution / Credits screen).
    public func images() throws -> [ImageRecord] { try database.images() }

    /// Raw bytes of an image file inside the pack (the HEIC/stub the UI decodes), or nil if the
    /// entry is absent. Foundations cards + story covers resolve their art through this.
    public func imageData(for image: ImageRecord) -> Data? {
        image.file.isEmpty ? nil : archive.data(for: image.file)
    }

    /// The comprehension questions for a story (FR-6.1).
    public func questions(for storyID: String) throws -> [QuestionRecord] {
        try database.questions(storyID: storyID)
    }

    /// The word-level alignment for a story (FR-5.1). Empty if the story has no pack audio.
    public func alignment(storyID: String) throws -> [AlignmentToken] {
        try database.alignment(storyID: storyID)
    }

    /// Raw bytes of a story's audio, or nil if the story ships no audio / the entry is absent.
    public func audioData(for story: StoryRecord) -> Data? {
        guard let file = story.audioFile else { return nil }
        return archive.data(for: file)
    }

    /// A playable file URL for a story's audio (extracted lazily and cached), or nil when the
    /// story has no pack audio. Callers that get nil fall back to system TTS (FR-5.4).
    public func audioURL(for story: StoryRecord) throws -> URL? {
        guard let file = story.audioFile else { return nil }
        if let cached = audioURLs[file] { return cached }
        guard let data = archive.data(for: file) else { return nil }
        let ext = (file as NSString).pathExtension.isEmpty ? "opus" : (file as NSString).pathExtension
        let out = FileManager.default.temporaryDirectory
            .appendingPathComponent("zhuwen-audio-\(UUID().uuidString).\(ext)")
        try data.write(to: out)
        audioURLs[file] = out
        return out
    }
}
