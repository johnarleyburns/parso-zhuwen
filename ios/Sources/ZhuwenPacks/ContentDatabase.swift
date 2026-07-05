import Foundation
import SQLite3

private let SQLITE_TRANSIENT = unsafeBitCast(-1, to: sqlite3_destructor_type.self)

/// Read-only accessor over an extracted `content.sqlite` (handoff §5: ZhuwenPacks owns
/// typed pack queries). Enforces the content-level half of I6.
public final class ContentDatabase {
    public enum DBError: Error, Equatable {
        case openFailed(Int32)
        case queryFailed(String)
        case i6(String)
    }

    private var db: OpaquePointer?

    public init(path: String) throws {
        var handle: OpaquePointer?
        let rc = sqlite3_open_v2(path, &handle, SQLITE_OPEN_READONLY, nil)
        guard rc == SQLITE_OK, handle != nil else {
            if let h = handle { sqlite3_close(h) }
            throw DBError.openFailed(rc)
        }
        db = handle
    }

    deinit { if let db { sqlite3_close(db) } }

    // MARK: - Queries

    public func meta(_ key: String) -> String? {
        var value: String?
        try? run("SELECT value FROM meta WHERE key = ?", binds: [key]) { stmt in
            value = column(stmt, 0)
        }
        return value
    }

    public func stories() throws -> [StoryRecord] {
        var out: [StoryRecord] = []
        try run("""
            SELECT id,title_zh,title_en,band,hsk3_level,token_count,type_count,
                   cover_image_id,canon_id,tier,origin,new_type_ids,body,audio_file
            FROM story ORDER BY id
            """) { stmt in
            let newTypes = decodeIntArray(column(stmt, 11) ?? "[]")
            let body = decodeBody(column(stmt, 12) ?? "[]")
            let audio = column(stmt, 13)
            out.append(StoryRecord(
                id: column(stmt, 0) ?? "",
                titleZH: column(stmt, 1) ?? "",
                titleEN: column(stmt, 2) ?? "",
                band: column(stmt, 3) ?? "",
                hsk3Level: Int(sqlite3_column_int64(stmt, 4)),
                tokenCount: Int(sqlite3_column_int64(stmt, 5)),
                typeCount: Int(sqlite3_column_int64(stmt, 6)),
                coverImageID: column(stmt, 7) ?? "",
                canonID: column(stmt, 8) ?? "",
                tier: column(stmt, 9) ?? "",
                origin: column(stmt, 10) ?? "",
                newTypeIDs: newTypes,
                body: body,
                audioFile: (audio?.isEmpty ?? true) ? nil : audio))
        }
        return out
    }

    /// The word-level alignment for a story, ordered by token index (FR-5.1). Empty if the
    /// story ships no audio. Read from the `alignment` table (authoritative over the
    /// denormalized `story.alignment` JSON).
    public func alignment(storyID: String) throws -> [AlignmentToken] {
        var out: [AlignmentToken] = []
        try run("SELECT token_idx,t0_ms,t1_ms FROM alignment WHERE story_id = ? ORDER BY token_idx", binds: [storyID]) { stmt in
            out.append(AlignmentToken(
                tokenIdx: Int(sqlite3_column_int64(stmt, 0)),
                t0ms: Int(sqlite3_column_int64(stmt, 1)),
                t1ms: Int(sqlite3_column_int64(stmt, 2))))
        }
        return out
    }

    /// The three comprehension questions for a story, ordered by id (FR-6.1). Empty if none.
    public func questions(storyID: String) throws -> [QuestionRecord] {
        var out: [QuestionRecord] = []
        try run("SELECT id,story_id,prompt_zh,options,answer_idx,band FROM question WHERE story_id = ? ORDER BY id",
                binds: [storyID]) { stmt in
            out.append(QuestionRecord(
                id: column(stmt, 0) ?? "",
                storyID: column(stmt, 1) ?? "",
                promptZH: column(stmt, 2) ?? "",
                options: decodeStringArray(column(stmt, 3) ?? "[]"),
                answerIdx: Int(sqlite3_column_int64(stmt, 4)),
                band: column(stmt, 5) ?? ""))
        }
        return out
    }

    public func lexicon() throws -> [WordRecord] {
        var out: [WordRecord] = []
        try run("SELECT word_id,simp,pinyin,hsk3_level,freq_rank FROM lexicon ORDER BY word_id") { stmt in
            out.append(WordRecord(
                id: Int(sqlite3_column_int64(stmt, 0)),
                simp: column(stmt, 1) ?? "",
                pinyin: column(stmt, 2) ?? "",
                hsk3Level: Int(sqlite3_column_int64(stmt, 3)),
                freqRank: Int(sqlite3_column_int64(stmt, 4))))
        }
        return out
    }

    /// Content-level I6: every story's cover_image_id must be non-empty and resolve to an
    /// image row with a complete provenance record (mirrors factory `verifyI6`).
    public func verifyI6() throws {
        var refs: [(String, String)] = []
        try run("SELECT id, cover_image_id FROM story") { stmt in
            refs.append((column(stmt, 0) ?? "", column(stmt, 1) ?? ""))
        }
        for (story, image) in refs {
            if image.isEmpty { throw DBError.i6("story \(story) has empty cover_image_id") }
            var ok = false
            var complete = false
            try run("SELECT license,license_url,author,source_url,retrieved_at FROM image WHERE id = ?", binds: [image]) { stmt in
                ok = true
                complete = !(column(stmt, 0) ?? "").isEmpty
                    && !(column(stmt, 1) ?? "").isEmpty
                    && !(column(stmt, 2) ?? "").isEmpty
                    && !(column(stmt, 3) ?? "").isEmpty
                    && !(column(stmt, 4) ?? "").isEmpty
            }
            if !ok { throw DBError.i6("story \(story) references missing image \(image)") }
            if !complete { throw DBError.i6("image \(image) missing provenance record") }
        }
    }

    // MARK: - Low-level

    private func run(_ sql: String, binds: [String] = [], _ each: (OpaquePointer) -> Void) throws {
        var stmt: OpaquePointer?
        guard sqlite3_prepare_v2(db, sql, -1, &stmt, nil) == SQLITE_OK else {
            throw DBError.queryFailed(String(cString: sqlite3_errmsg(db)))
        }
        defer { sqlite3_finalize(stmt) }
        for (i, b) in binds.enumerated() {
            sqlite3_bind_text(stmt, Int32(i + 1), b, -1, SQLITE_TRANSIENT)
        }
        while sqlite3_step(stmt) == SQLITE_ROW {
            each(stmt!)
        }
    }

    private func column(_ stmt: OpaquePointer, _ i: Int32) -> String? {
        guard let c = sqlite3_column_text(stmt, i) else { return nil }
        return String(cString: c)
    }

    private func decodeIntArray(_ json: String) -> [Int] {
        (try? JSONDecoder().decode([Int].self, from: Data(json.utf8))) ?? []
    }

    private func decodeStringArray(_ json: String) -> [String] {
        (try? JSONDecoder().decode([String].self, from: Data(json.utf8))) ?? []
    }

    private func decodeBody(_ json: String) -> [BodyToken] {
        (try? JSONDecoder().decode([BodyToken].self, from: Data(json.utf8))) ?? []
    }
}
