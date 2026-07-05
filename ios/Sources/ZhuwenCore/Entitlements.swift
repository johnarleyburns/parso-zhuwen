import Foundation

/// The learner's access tier, supplied by StoreKit 2 (00 §4 FR-9). The engine is the upsell — free
/// users get the full method at one story/day; Pro removes the throttle and opens the lattice
/// (00 §6.6). This type is deliberately *pure*: the StoreKit layer (ZhuwenUI) only vends the case.
public enum Entitlement: String, Codable, Equatable, Sendable {
    case free
    case pro
}

/// A purchasable product (FR-9.3). StoreKit is the source of truth for live prices/trials; this
/// catalog pins the product **ids** the app knows about and the display defaults used before
/// StoreKit responds (and in host tests, where StoreKit is unavailable).
public struct StoreProduct: Equatable, Identifiable, Sendable {
    public enum Kind: String, Equatable, Sendable { case monthly, annual, lifetime }

    public let id: String
    public let kind: Kind
    public let displayPrice: String
    public let trialDays: Int   // 0 = none

    public init(id: String, kind: Kind, displayPrice: String, trialDays: Int) {
        self.id = id; self.kind = kind; self.displayPrice = displayPrice; self.trialDays = trialDays
    }
}

/// The three v1 SKUs, no ads, StoreKit 2 only, no receipt server (FR-9.3).
public enum ProductCatalog {
    public static let monthly  = StoreProduct(id: "ai.zhuwen.pro.monthly",  kind: .monthly,  displayPrice: "$7.99",   trialDays: 0)
    public static let annual   = StoreProduct(id: "ai.zhuwen.pro.annual",   kind: .annual,   displayPrice: "$59.99",  trialDays: 30)
    public static let lifetime = StoreProduct(id: "ai.zhuwen.pro.lifetime", kind: .lifetime, displayPrice: "$149.99", trialDays: 0)

    /// Display order on the paywall (annual first — it carries the trial).
    public static let all: [StoreProduct] = [annual, monthly, lifetime]

    public static let ids: Set<String> = Set(all.map(\.id))
}

/// FeatureGate answers the free/Pro questions (FR-9.1/9.2) for a given entitlement and the
/// learner's stories-opened-today count. Pure and deterministic; UI reads it, StoreKit sets the
/// entitlement. Free is the *full method* — placement, Foundations, one engine-selected story/day,
/// dictionary, capped review, progress basics — the throttle, not the wall, is the upsell.
public struct FeatureGate: Equatable, Sendable {
    public let entitlement: Entitlement

    /// Free users get one engine-selected story per day (FR-9.1).
    public static let freeDailyStoryLimit = 1
    /// Free review is capped tighter than Pro's 20/day (FR-9.1 "review (capped)").
    public static let freeReviewCap = 10
    public static let proReviewCap = 20

    public init(entitlement: Entitlement) { self.entitlement = entitlement }

    public var isPro: Bool { entitlement == .pro }

    /// May the learner open another story today, given how many they've already opened?
    public func canOpenStory(storiesOpenedToday: Int) -> Bool {
        isPro || storiesOpenedToday < Self.freeDailyStoryLimit
    }

    /// The daily review cap for this tier (FR-7.2 vs FR-9.1).
    public var reviewCap: Int { isPro ? Self.proReviewCap : Self.freeReviewCap }

    /// Pro-only surfaces (FR-9.2): full lattice browsing, listening packs, blind mode, full dashboard,
    /// monthly checkpoints, all future packs. Free keeps placement/Foundations/dictionary/progress-basics.
    public var canBrowseLattice: Bool { isPro }
    public var canUseListeningPacks: Bool { isPro }
    public var canUseBlindMode: Bool { isPro }
    public var canSeeFullDashboard: Bool { isPro }
    public var canSeeMonthlyCheckpoint: Bool { isPro }
    public var canDownloadAllPacks: Bool { isPro }
}
