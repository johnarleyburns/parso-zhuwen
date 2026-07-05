import Foundation

/// Stable, machine-readable gate failure codes. These MUST match the Go reference gate
/// (`internal/gate` `Code*` constants) — the shared vector suite (`gate-vectors.json`,
/// handoff §7) asserts both implementations emit the same sorted-unique code set.
public enum GateCode {
    public static let literalOutOfLexicon = "literal_out_of_lexicon"
    public static let typeBudget = "type_budget"
    public static let tokenBudget = "token_budget"
    public static let recurrence = "recurrence"
    public static let frontier = "frontier"
    public static let grammar = "grammar"
    public static let properNounGloss = "proper_noun_gloss"
}

/// Pedagogy budgets (00 §6, PED-001..003) — mirrors factory `gate.Config`.
public struct GateConfig: Equatable {
    public var maxNewTypes: Int
    public var maxNewTokenRatio: Double
    public var minRecurrence: Int
    public var minCoverage: Double

    public init(maxNewTypes: Int, maxNewTokenRatio: Double, minRecurrence: Int, minCoverage: Double) {
        self.maxNewTypes = maxNewTypes
        self.maxNewTokenRatio = maxNewTokenRatio
        self.minRecurrence = minRecurrence
        self.minCoverage = minCoverage
    }

    /// The locked spec budgets (must equal factory `gate.DefaultConfig`).
    public static let `default` = GateConfig(maxNewTypes: 8, maxNewTokenRatio: 0.02, minRecurrence: 3, minCoverage: 0.98)
}

/// The target lexicon slice a gate evaluation runs against — mirrors factory `gate.Band`.
public struct Band {
    public var known: Set<Int>
    public var frontier: Set<Int>
    public var grammar: Set<String>

    public init(known: Set<Int> = [], frontier: Set<Int> = [], grammar: Set<String> = []) {
        self.known = known
        self.frontier = frontier
        self.grammar = grammar
    }
}

/// One segmented token fed to the gate (mirrors factory `segment.Token`).
public struct GateToken: Equatable {
    public enum Kind: Equatable { case word, literal, properNoun }
    public var kind: Kind
    public var wordID: Int   // -1 for literal / proper noun
    public var text: String
    public var gloss: String // proper-noun gloss ("" if none)
    public var first: Bool   // first occurrence of this proper noun

    public init(kind: Kind, wordID: Int = -1, text: String = "", gloss: String = "", first: Bool = false) {
        self.kind = kind
        self.wordID = wordID
        self.text = text
        self.gloss = gloss
        self.first = first
    }

    public static func word(_ id: Int, _ text: String = "") -> GateToken { GateToken(kind: .word, wordID: id, text: text) }
    public static func literal(_ text: String) -> GateToken { GateToken(kind: .literal, text: text) }
    public static func proper(_ text: String, gloss: String, first: Bool) -> GateToken {
        GateToken(kind: .properNoun, text: text, gloss: gloss, first: first)
    }
}

/// A rule-based grammar-pattern detector (mirrors factory `grammar.MarkerDetector`).
public struct GrammarDetector {
    private static let markerRules: [(id: String, marker: String)] = [
        ("ba-construction", "把"),
        ("bei-construction", "被"),
        ("le-aspect", "了"),
        ("guo-aspect", "过"),
        ("de-attributive", "的"),
        ("ma-question", "吗"),
        ("bu-negation", "不"),
        ("zai-progressive", "在"),
    ]

    public init() {}

    public func detect(_ tokens: [GateToken]) -> [String] {
        var present = Set<String>()
        for t in tokens { present.insert(t.text) }
        return Self.markerRules.compactMap { present.contains($0.marker) ? $0.id : nil }
    }
}

/// Proof that a token stream passed the coverage gate — invariant I1. The initializer is
/// private, so only `CoverageGate.evaluate` can produce a populated candidate; every other
/// caller can name the type but only ever holds `nil`.
public struct StoryCandidate: Equatable {
    public let coverage: Double
    public let coverageBps: Int
    public let newTypeIDs: [Int]
    public let tokenCount: Int
    public let typeCount: Int
    public let coverageBitmap: WordBitmap

    fileprivate init(coverage: Double, coverageBps: Int, newTypeIDs: [Int],
                     tokenCount: Int, typeCount: Int, coverageBitmap: WordBitmap) {
        self.coverage = coverage
        self.coverageBps = coverageBps
        self.newTypeIDs = newTypeIDs
        self.tokenCount = tokenCount
        self.typeCount = typeCount
        self.coverageBitmap = coverageBitmap
    }
}

/// The outcome of a gate evaluation (mirrors factory `gate.Result`).
public struct GateResult {
    public let pass: Bool
    public let reasons: [String]
    public let codes: [String]        // sorted-unique machine codes (empty iff pass)
    public let candidate: StoryCandidate?
    public let coverage: Double
    public let coverageBps: Int
    public let denomTokens: Int
    public let newTokens: Int
}

/// CoverageGate is the on-device I1 gate. It shares its algorithm (and its golden vectors)
/// with the Go factory gate so the invariant cannot drift between the two implementations.
public enum CoverageGate {
    /// coverage in basis points, integer round-half-up — identical formula to `gatevec`.
    public static func coverageBps(denom: Int, newTokens: Int) -> Int {
        guard denom > 0 else { return 0 }
        let covered = denom - newTokens
        return (10_000 * covered + denom / 2) / denom
    }

    public static func evaluate(tokens: [GateToken], band: Band, detector: GrammarDetector = GrammarDetector(),
                                maxWordID: Int, config: GateConfig = .default) -> GateResult {
        var reasons: [String] = []
        var codeSet = Set<String>()
        func fail(_ code: String, _ reason: String) {
            codeSet.insert(code)
            reasons.append(reason)
        }

        // Denominator = running tokens excluding proper nouns (00 §6 proper-noun rule).
        var denom = 0
        var occ: [Int: Int] = [:]
        var typesSet = Set<Int>()
        var literals: [String] = []
        for t in tokens {
            switch t.kind {
            case .word:
                denom += 1
                occ[t.wordID, default: 0] += 1
                typesSet.insert(t.wordID)
            case .literal:
                denom += 1
                literals.append(t.text)
            case .properNoun:
                break // excluded from denominator
            }
        }

        if !literals.isEmpty {
            fail(GateCode.literalOutOfLexicon, "out-of-lexicon literal tokens (\(literals.count)): \(literals)")
        }

        let newTypes = typesSet.subtracting(band.known).sorted()

        if newTypes.count > config.maxNewTypes {
            fail(GateCode.typeBudget, "new type budget exceeded: \(newTypes.count) > \(config.maxNewTypes)")
        }

        var newTokenCount = 0
        for id in newTypes { newTokenCount += occ[id] ?? 0 }
        newTokenCount += literals.count
        var ratio = 0.0
        var coverage = 0.0
        if denom > 0 {
            ratio = Double(newTokenCount) / Double(denom)
            coverage = 1 - ratio
        }
        if ratio > config.maxNewTokenRatio {
            fail(GateCode.tokenBudget, String(format: "new-token ratio %.4f > %.4f", ratio, config.maxNewTokenRatio))
        }

        for id in newTypes where (occ[id] ?? 0) < config.minRecurrence {
            fail(GateCode.recurrence, "new type \(id) occurs \(occ[id] ?? 0) time(s), needs >= \(config.minRecurrence)")
        }

        for id in newTypes where !band.frontier.contains(id) {
            fail(GateCode.frontier, "new type \(id) not in frontier queue")
        }

        for p in detector.detect(tokens) where !band.grammar.contains(p) {
            fail(GateCode.grammar, "grammar pattern \"\(p)\" not in band whitelist")
        }

        for t in tokens where t.kind == .properNoun && t.first && t.gloss.isEmpty {
            fail(GateCode.properNounGloss, "proper noun \"\(t.text)\" lacks first-occurrence gloss")
        }

        let bps = coverageBps(denom: denom, newTokens: newTokenCount)

        if !reasons.isEmpty {
            return GateResult(pass: false, reasons: reasons, codes: codeSet.sorted(),
                              candidate: nil, coverage: coverage, coverageBps: bps,
                              denomTokens: denom, newTokens: newTokenCount)
        }

        var bitmap = WordBitmap(bitCount: maxWordID + 1)
        for id in typesSet { bitmap.set(id) }
        let candidate = StoryCandidate(coverage: coverage, coverageBps: bps, newTypeIDs: newTypes,
                                       tokenCount: denom, typeCount: typesSet.count, coverageBitmap: bitmap)
        return GateResult(pass: true, reasons: [], codes: [], candidate: candidate,
                          coverage: coverage, coverageBps: bps, denomTokens: denom, newTokens: newTokenCount)
    }
}
