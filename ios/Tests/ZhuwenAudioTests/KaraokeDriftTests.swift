import XCTest
import ZhuwenPacks
@testable import ZhuwenAudio

/// CP-06 acceptance (handoff §6): **highlight drift < 120 ms across a 3-minute story.**
///
/// On device the highlighted-word lag has two sources: (1) the resolver picking the wrong
/// token, and (2) the UI refreshing the highlight only every display tick, so between ticks
/// the highlight is stale. `AlignmentTrack.index(atMillis:)` is exact (unit-tested), so this
/// suite pins the *end-to-end* drift: it plays a synthesized 3-minute alignment track through
/// the resolver at a UI refresh cadence and measures, at 1 ms resolution, the longest span for
/// which the displayed token differs from the truly-active token. That span is the highlight
/// drift. It must stay under 120 ms at every supported speed.
final class KaraokeDriftTests: XCTestCase {
    /// A deterministic ~3-minute track: sentences of 8 tokens (1–2 chars) at ~260 ms/char with
    /// a 320 ms pause between sentences and 250 ms lead-in — the factory aligner's shape.
    private func threeMinuteTrack() -> AlignmentTrack {
        let perChar = 260, minTok = 140, sentenceGap = 320, leadIn = 250
        var tokens: [AlignmentToken] = []
        var t = leadIn
        var idx = 0
        var inSentence = 0
        while t < 180_000 {
            if inSentence == 8 { inSentence = 0; t += sentenceGap }
            let chars = (idx % 3 == 0) ? 2 : 1
            let dur = max(minTok, chars * perChar)
            tokens.append(AlignmentToken(tokenIdx: idx, t0ms: t, t1ms: t + dur))
            t += dur
            idx += 1
            inSentence += 1
        }
        return AlignmentTrack(tokens)
    }

    /// Simulates playback: the display refreshes every `uiTickMs` of wall time (media advances
    /// `uiTickMs * speed` per refresh). Returns the max duration (media-ms) the highlight is stale.
    private func maxDrift(track: AlignmentTrack, speed: Double, uiTickMs: Double) -> Int {
        let end = track.durationMillis
        let step = uiTickMs * speed // media-ms advanced per display refresh
        var displayed = track.index(atMillis: 0)
        var nextRefresh = step
        var mismatchStart: Int? = nil
        var maxDrift = 0
        var m = 0
        while m <= end {
            while Double(m) >= nextRefresh {
                displayed = track.index(atMillis: Int(nextRefresh))
                nextRefresh += step
            }
            let active = track.index(atMillis: m)
            if active != displayed {
                if mismatchStart == nil { mismatchStart = m }
                maxDrift = max(maxDrift, m - mismatchStart!)
            } else {
                mismatchStart = nil
            }
            m += 1
        }
        return maxDrift
    }

    func testTrackSpansThreeMinutes() {
        let track = threeMinuteTrack()
        XCTAssertGreaterThanOrEqual(track.durationMillis, 180_000)
        XCTAssertGreaterThan(track.tokenCount, 400) // a real story's worth of words
    }

    func testHighlightDriftUnder120msOver3MinStory() {
        let track = threeMinuteTrack()
        // A conservative 20 Hz UI refresh (50 ms) — slower than a 60 Hz CADisplayLink, so this
        // is a worst-case bound on the real highlight cadence.
        let uiTickMs = 50.0
        for speed in [0.6, 0.8, 1.0, 1.2] {
            let drift = maxDrift(track: track, speed: speed, uiTickMs: uiTickMs)
            // Non-trivial: the simulation must actually observe refresh-cadence staleness…
            XCTAssertGreaterThan(drift, 0, "simulation observed no lag at \(speed)× — not exercising drift")
            // …yet stay under the 120 ms acceptance budget (handoff §6).
            XCTAssertLessThan(drift, 120, "drift \(drift) ms at \(speed)× exceeds the 120 ms budget")
        }
    }

    func testHighlightDriftAtDisplayLinkCadenceHasWideMargin() {
        // At a real 60 Hz CADisplayLink cadence the drift is a fraction of the budget.
        let track = threeMinuteTrack()
        let drift = maxDrift(track: track, speed: 1.2, uiTickMs: 1000.0 / 60.0)
        XCTAssertLessThan(drift, 40)
    }

    func testDriftSimulationIsDeterministic() {
        let track = threeMinuteTrack()
        let a = maxDrift(track: track, speed: 1.0, uiTickMs: 50)
        let b = maxDrift(track: track, speed: 1.0, uiTickMs: 50)
        XCTAssertEqual(a, b)
    }

    // MARK: - CP-09c: re-verify drift against a *real-render* alignment sample.

    /// Loads the vendored real-render alignment fixture (`ios/Fixtures/real-render-alignment.json`)
    /// — a ~3-minute story whose word timings come from the real CosyVoice + forced-aligner path
    /// (non-uniform per-word durations and variable sentence pauses, unlike the synthesized track).
    private func realRenderTrack() throws -> AlignmentTrack {
        let url = Fixtures.dir.appendingPathComponent("real-render-alignment.json")
        let data = try Data(contentsOf: url)
        struct Row: Decodable { let i: Int; let t0: Int; let t1: Int }
        let rows = try JSONDecoder().decode([Row].self, from: data)
        return AlignmentTrack(rows.map { AlignmentToken(tokenIdx: $0.i, t0ms: $0.t0, t1ms: $0.t1) })
    }

    func testRealRenderTrackSpansThreeMinutes() throws {
        let track = try realRenderTrack()
        XCTAssertGreaterThanOrEqual(track.durationMillis, 180_000)
        XCTAssertGreaterThan(track.tokenCount, 400)
    }

    /// CP-09c acceptance: with the *real* CosyVoice timings (not the synthesized track), highlight
    /// drift stays under the 120 ms budget at every supported speed. This is the pre-push gate that
    /// the karaoke feature still holds once real audio replaces the fixture stub.
    func testHighlightDriftUnder120msWithRealRenderAudio() throws {
        let track = try realRenderTrack()
        let uiTickMs = 50.0 // conservative 20 Hz worst case
        for speed in [0.6, 0.8, 1.0, 1.2] {
            let drift = maxDrift(track: track, speed: speed, uiTickMs: uiTickMs)
            XCTAssertGreaterThan(drift, 0, "no lag observed at \(speed)× — not exercising drift")
            XCTAssertLessThan(drift, 120, "real-audio drift \(drift) ms at \(speed)× exceeds the 120 ms budget")
        }
    }
}
