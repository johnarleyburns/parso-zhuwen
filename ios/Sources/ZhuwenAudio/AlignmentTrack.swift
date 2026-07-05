import Foundation
import ZhuwenPacks

/// A story's word-level alignment as a fast, pure resolver from playback position (ms) to the
/// token that should be highlighted (FR-5.1 karaoke). This mapping is the drift-critical core
/// of the listening feature: on device, highlight lag is bounded by (a) this resolver being
/// exact and (b) the UI's refresh cadence — both covered by `KaraokeDriftTests`.
///
/// Convention: within a token's `[t0, t1)` that token is active; in the silent gap *after* a
/// token (before the next begins) the just-finished token stays highlighted (natural karaoke);
/// before the first token (lead-in silence) nothing is highlighted (`nil`).
public struct AlignmentTrack: Equatable {
    /// One resolved interval; `tokenIdx` indexes into the story body stream.
    public struct Interval: Equatable {
        public let tokenIdx: Int
        public let t0ms: Int
        public let t1ms: Int
        public init(tokenIdx: Int, t0ms: Int, t1ms: Int) {
            self.tokenIdx = tokenIdx; self.t0ms = t0ms; self.t1ms = t1ms
        }
    }

    /// Intervals sorted by start time.
    public let intervals: [Interval]
    /// Total content duration (end of the last token). Trailing silence beyond this is the
    /// player's concern; the track ends when the last word ends.
    public let durationMillis: Int

    public init(_ tokens: [AlignmentToken]) {
        let sorted = tokens
            .map { Interval(tokenIdx: $0.tokenIdx, t0ms: $0.t0ms, t1ms: $0.t1ms) }
            .sorted { $0.t0ms < $1.t0ms }
        self.intervals = sorted
        self.durationMillis = sorted.last?.t1ms ?? 0
    }

    public init(intervals: [Interval]) {
        let sorted = intervals.sorted { $0.t0ms < $1.t0ms }
        self.intervals = sorted
        self.durationMillis = sorted.last?.t1ms ?? 0
    }

    public var isEmpty: Bool { intervals.isEmpty }
    public var tokenCount: Int { intervals.count }

    /// Start time (ms) of the token at body position `tokenIndex`, for tap-to-seek (FR-5.1).
    public func startMillis(ofTokenAt tokenIndex: Int) -> Int? {
        intervals.first(where: { $0.tokenIdx == tokenIndex })?.t0ms
    }

    /// The token highlighted at playback position `ms`, or nil during lead-in silence.
    public func index(atMillis ms: Int) -> Int? {
        activeInterval(atMillis: ms)?.tokenIdx
    }

    /// The interval highlighted at `ms` (see convention above). Binary search: O(log n).
    public func activeInterval(atMillis ms: Int) -> Interval? {
        guard let first = intervals.first else { return nil }
        if ms < first.t0ms { return nil } // lead-in silence

        // Largest interval whose t0 <= ms.
        var lo = 0, hi = intervals.count - 1, k = 0
        while lo <= hi {
            let mid = (lo + hi) / 2
            if intervals[mid].t0ms <= ms {
                k = mid
                lo = mid + 1
            } else {
                hi = mid - 1
            }
        }
        // Within [t0,t1) it's active; in the gap after it, it stays highlighted; after the
        // final token it also stays (playback tail).
        return intervals[k]
    }
}
