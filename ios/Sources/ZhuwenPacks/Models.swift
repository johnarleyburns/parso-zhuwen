import Foundation

/// The signed pack manifest (mirrors factory `pack.Manifest`).
public struct PackManifest: Codable, Equatable {
    public let id: String
    public let semver: String
    public let lexiconVersion: String
    public let createdAt: String
    public let schemaVersion: Int
    public let files: [String: String] // path -> sha256 hex

    enum CodingKeys: String, CodingKey {
        case id, semver
        case lexiconVersion = "lexicon_version"
        case createdAt = "created_at"
        case schemaVersion = "schema_version"
        case files
    }
}

/// One token of a story's segmented body (mirrors factory `pack.BodyToken`: {w,lit,s,pn}).
public struct BodyToken: Codable, Equatable {
    public let w: Int        // word_id, or -1 for literal / proper noun
    public let lit: String?  // surface text for literal / proper noun
    public let s: Int        // sentence index
    public let pn: Bool?     // proper noun

    public init(w: Int, lit: String?, s: Int, pn: Bool?) {
        self.w = w; self.lit = lit; self.s = s; self.pn = pn
    }

    public var isProperNoun: Bool { pn ?? false }
    public var isLiteral: Bool { w < 0 && !(pn ?? false) }
}

/// A story row plus its parsed body.
public struct StoryRecord: Equatable {
    public let id: String
    public let titleZH: String
    public let titleEN: String
    public let band: String
    public let hsk3Level: Int
    public let tokenCount: Int
    public let typeCount: Int
    public let coverImageID: String
    public let canonID: String
    public let tier: String
    public let origin: String
    public let newTypeIDs: [Int]
    public let body: [BodyToken]
    public let audioFile: String? // pack-relative path, e.g. audio/s1.opus (nil = no pack audio)

    public init(id: String, titleZH: String, titleEN: String, band: String, hsk3Level: Int,
                tokenCount: Int, typeCount: Int, coverImageID: String, canonID: String,
                tier: String, origin: String, newTypeIDs: [Int], body: [BodyToken],
                audioFile: String? = nil) {
        self.id = id; self.titleZH = titleZH; self.titleEN = titleEN; self.band = band
        self.hsk3Level = hsk3Level; self.tokenCount = tokenCount; self.typeCount = typeCount
        self.coverImageID = coverImageID; self.canonID = canonID; self.tier = tier
        self.origin = origin; self.newTypeIDs = newTypeIDs; self.body = body
        self.audioFile = audioFile
    }
}

/// One word-level timing window (handoff §3 `alignment` table; FR-5.1 karaoke). `tokenIdx`
/// indexes into the story body stream; the app highlights the matching token during playback.
public struct AlignmentToken: Codable, Equatable {
    public let tokenIdx: Int
    public let t0ms: Int
    public let t1ms: Int

    enum CodingKeys: String, CodingKey {
        case tokenIdx = "i"
        case t0ms = "t0"
        case t1ms = "t1"
    }

    public init(tokenIdx: Int, t0ms: Int, t1ms: Int) {
        self.tokenIdx = tokenIdx; self.t0ms = t0ms; self.t1ms = t1ms
    }
}

/// One comprehension MC question (handoff §3 `question` table; FR-6.1). `options` are shown in
/// order; `answerIdx` is the single key. Question text is factory-gated to the story band.
public struct QuestionRecord: Codable, Equatable, Identifiable {
    public let id: String
    public let storyID: String
    public let promptZH: String
    public let options: [String]
    public let answerIdx: Int
    public let band: String

    public init(id: String, storyID: String, promptZH: String, options: [String],
                answerIdx: Int, band: String) {
        self.id = id; self.storyID = storyID; self.promptZH = promptZH
        self.options = options; self.answerIdx = answerIdx; self.band = band
    }
}

/// A lexicon entry.
public struct WordRecord: Equatable {
    public let id: Int
    public let simp: String
    public let pinyin: String
    public let hsk3Level: Int
    public let freqRank: Int

    public init(id: Int, simp: String, pinyin: String, hsk3Level: Int, freqRank: Int) {
        self.id = id; self.simp = simp; self.pinyin = pinyin
        self.hsk3Level = hsk3Level; self.freqRank = freqRank
    }
}
