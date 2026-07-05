import Foundation

/// A fixed-width bit vector indexed by word ID (00 §9: "known set → 11k-bit bitmap"), the
/// substrate for the selector's NFR-2 hot path (bitmap AND + popcount). Backed by `UInt64`
/// words so coverage math is a handful of native `nonzeroBitCount` intrinsics per story.
public struct WordBitmap: Equatable {
    public private(set) var words: [UInt64]

    public init(bitCount: Int) {
        words = [UInt64](repeating: 0, count: (Swift.max(0, bitCount) + 63) / 64)
    }

    public init(words: [UInt64]) { self.words = words }

    /// Builds a bitmap with the given word IDs set.
    public init<S: Sequence>(ids: S, bitCount: Int) where S.Element == Int {
        self.init(bitCount: bitCount)
        for id in ids { set(id) }
    }

    public var capacityBits: Int { words.count * 64 }

    public mutating func set(_ id: Int) {
        guard id >= 0 else { return }
        let w = id >> 6
        if w >= words.count { words.append(contentsOf: repeatElement(0, count: w - words.count + 1)) }
        words[w] |= 1 << UInt64(id & 63)
    }

    public func test(_ id: Int) -> Bool {
        guard id >= 0 else { return false }
        let w = id >> 6
        guard w < words.count else { return false }
        return words[w] & (1 << UInt64(id & 63)) != 0
    }

    public func popcount() -> Int {
        var n = 0
        for w in words { n += w.nonzeroBitCount }
        return n
    }

    /// popcount(self & ~other): how many of this bitmap's bits are *not* in `other`.
    /// This is the uncovered-type count when `self` is a story and `other` is the known set.
    public func subtractingPopcount(_ other: WordBitmap) -> Int {
        words.withUnsafeBufferPointer { a in
            other.words.withUnsafeBufferPointer { b in
                var n = 0
                let oc = b.count
                for i in 0..<a.count {
                    let o = i < oc ? b[i] : 0
                    n += (a[i] & ~o).nonzeroBitCount
                }
                return n
            }
        }
    }

    /// True iff (self & ~known) ⊆ frontier — i.e. every uncovered type is a frontier word.
    /// The I1 recommendation condition, evaluated word-wise with no allocation.
    public func uncoveredIsSubset(known: WordBitmap, of frontier: WordBitmap) -> Bool {
        words.withUnsafeBufferPointer { a in
            known.words.withUnsafeBufferPointer { k in
                frontier.words.withUnsafeBufferPointer { f in
                    let kc = k.count, fc = f.count
                    for i in 0..<a.count {
                        let kv = i < kc ? k[i] : 0
                        let fv = i < fc ? f[i] : 0
                        if a[i] & ~kv & ~fv != 0 { return false }
                    }
                    return true
                }
            }
        }
    }

    /// The word IDs set in (self & ~other), ascending. Off the hot path (scoring / listing).
    public func subtracting(_ other: WordBitmap) -> [Int] {
        var out: [Int] = []
        for i in 0..<words.count {
            let o = i < other.words.count ? other.words[i] : 0
            var bits = words[i] & ~o
            while bits != 0 {
                let b = bits.trailingZeroBitCount
                out.append(i * 64 + b)
                bits &= bits - 1
            }
        }
        return out
    }

    /// popcount(self & other): shared bits (e.g. story types that are due-for-review words).
    public func intersectingPopcount(_ other: WordBitmap) -> Int {
        words.withUnsafeBufferPointer { a in
            other.words.withUnsafeBufferPointer { b in
                var n = 0
                let count = Swift.min(a.count, b.count)
                for i in 0..<count { n += (a[i] & b[i]).nonzeroBitCount }
                return n
            }
        }
    }
}
