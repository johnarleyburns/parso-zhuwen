import Foundation

/// A small, dependency-free deterministic PRNG (SplitMix64). Placement item sampling, foil
/// generation, and the simulation tests all seed one of these so runs are byte-reproducible
/// (NFR-5: no third-party RNG; `SystemRandomNumberGenerator` is non-deterministic).
public struct SplitMix64: RandomNumberGenerator {
    private var state: UInt64

    public init(seed: UInt64) { self.state = seed }

    public mutating func next() -> UInt64 {
        state = state &+ 0x9E37_79B9_7F4A_7C15
        var z = state
        z = (z ^ (z >> 30)) &* 0xBF58_476D_1CE4_E5B9
        z = (z ^ (z >> 27)) &* 0x94D0_49BB_1331_11EB
        return z ^ (z >> 31)
    }

    /// A uniform Double in [0, 1) — used to simulate Bernoulli responses in tests and to
    /// sample within strata.
    public mutating func uniform() -> Double {
        Double(next() >> 11) * (1.0 / 9_007_199_254_740_992.0) // 2^53
    }
}
