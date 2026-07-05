import Foundation

/// Maps character offsets in a concatenated narration string back to token indices, so the
/// system-TTS fallback (FR-5.4) can highlight the current word from the synthesizer's
/// `willSpeakRangeOfSpeechString` delegate (which reports NSRanges into the spoken string).
/// Pure and testable; the AVSpeechSynthesizer wiring lives in `SystemTTSNarrator`.
public struct CharTokenMap: Equatable {
    /// The full text handed to the synthesizer (token surfaces concatenated, no separators —
    /// Mandarin has no inter-word spaces).
    public let fullText: String
    /// Cumulative UTF-16 start offset of each token (parallel to the token array).
    private let starts: [Int]
    private let lengths: [Int]

    public init(tokenTexts: [String]) {
        var starts: [Int] = []
        var lengths: [Int] = []
        var cursor = 0
        var text = ""
        for t in tokenTexts {
            let len = (t as NSString).length // UTF-16 units, matching NSRange from the delegate
            starts.append(cursor)
            lengths.append(len)
            cursor += len
            text += t
        }
        self.starts = starts
        self.lengths = lengths
        self.fullText = text
    }

    public var tokenCount: Int { starts.count }
    public var utf16Length: Int { (fullText as NSString).length }

    /// The token index containing UTF-16 offset `offset`, or the nearest preceding token.
    public func tokenIndex(forCharacterOffset offset: Int) -> Int? {
        guard !starts.isEmpty else { return nil }
        if offset < starts[0] { return nil }

        // Largest token whose start <= offset (binary search).
        var lo = 0, hi = starts.count - 1, k = 0
        while lo <= hi {
            let mid = (lo + hi) / 2
            if starts[mid] <= offset {
                k = mid
                lo = mid + 1
            } else {
                hi = mid - 1
            }
        }
        return k
    }

    /// The token index for the start of a spoken range (delegate convenience).
    public func tokenIndex(forRangeLocation location: Int) -> Int? {
        tokenIndex(forCharacterOffset: location)
    }
}
