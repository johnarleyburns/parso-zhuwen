import Foundation
import ZhuwenCore

#if canImport(StoreKit)
import StoreKit
#endif

/// Supplies the current `Entitlement` and drives purchases (00 §4 FR-9). Abstracted so the app can
/// run on the host (and in tests) with an in-memory provider, while the device uses StoreKit 2. No
/// receipt server is ever involved (FR-9.3) — entitlement is derived from StoreKit's current
/// transactions only.
public protocol EntitlementProvider: AnyObject {
    var entitlement: Entitlement { get }
    var products: [StoreProduct] { get }
    func refresh() async
    func purchase(_ product: StoreProduct) async throws
    func restore() async throws
}

/// The default, network-free provider: everyone is `free`. Used on the host, in previews, and as the
/// pre-StoreKit fallback. Tests can subclass or use `InMemoryEntitlementProvider(.pro)` to flip tiers.
public final class InMemoryEntitlementProvider: EntitlementProvider {
    public private(set) var entitlement: Entitlement
    public let products: [StoreProduct]

    public init(_ entitlement: Entitlement = .free, products: [StoreProduct] = ProductCatalog.all) {
        self.entitlement = entitlement
        self.products = products
    }

    public func refresh() async {}
    public func purchase(_ product: StoreProduct) async throws { entitlement = .pro }
    public func restore() async throws {}
    public func set(_ entitlement: Entitlement) { self.entitlement = entitlement }
}

/// StoreModel is the `@MainActor` commerce hub the paywall and gates observe. It wraps an
/// `EntitlementProvider` (StoreKit 2 on device, in-memory elsewhere) and exposes a `FeatureGate`
/// derived from the live entitlement.
@MainActor
public final class StoreModel: ObservableObject {
    @Published public private(set) var entitlement: Entitlement
    @Published public private(set) var products: [StoreProduct]
    @Published public var lastError: String?

    private let provider: EntitlementProvider

    public init(provider: EntitlementProvider = StoreModel.makeDefaultProvider()) {
        self.provider = provider
        self.entitlement = provider.entitlement
        self.products = provider.products
    }

    public var gate: FeatureGate { FeatureGate(entitlement: entitlement) }
    public var isPro: Bool { entitlement == .pro }

    public func refresh() async {
        await provider.refresh()
        sync()
    }

    public func purchase(_ product: StoreProduct) async {
        do {
            try await provider.purchase(product)
            lastError = nil
        } catch {
            lastError = String(describing: error)
        }
        sync()
    }

    public func restore() async {
        do {
            try await provider.restore()
            lastError = nil
        } catch {
            lastError = String(describing: error)
        }
        sync()
    }

    private func sync() {
        entitlement = provider.entitlement
        products = provider.products
    }

    /// StoreKit 2 on device (iOS), in-memory free tier otherwise.
    public nonisolated static func makeDefaultProvider() -> EntitlementProvider {
        #if canImport(StoreKit) && os(iOS)
        if #available(iOS 17.0, *) { return StoreKitEntitlementProvider() }
        #endif
        return InMemoryEntitlementProvider(.free)
    }
}

#if canImport(StoreKit) && os(iOS)
import StoreKit

/// StoreKit 2 entitlement provider (FR-9.3, no receipt server). Loads the three SKUs, runs the
/// purchase flow, and derives `.pro` from `Transaction.currentEntitlements`. Availability-guarded so
/// the package still builds/tests on the host, where this type is simply not compiled.
@available(iOS 17.0, *)
public final class StoreKitEntitlementProvider: EntitlementProvider {
    public private(set) var entitlement: Entitlement = .free
    public private(set) var products: [StoreProduct] = ProductCatalog.all
    private var storeProducts: [String: StoreKit.Product] = [:]

    public init() {}

    public func refresh() async {
        await loadProducts()
        await updateEntitlement()
    }

    private func loadProducts() async {
        guard let loaded = try? await StoreKit.Product.products(for: ProductCatalog.ids) else { return }
        storeProducts = Dictionary(uniqueKeysWithValues: loaded.map { ($0.id, $0) })
        products = ProductCatalog.all.map { known in
            guard let sk = storeProducts[known.id] else { return known }
            return StoreProduct(id: known.id, kind: known.kind,
                           displayPrice: sk.displayPrice, trialDays: known.trialDays)
        }
    }

    public func purchase(_ product: StoreProduct) async throws {
        guard let sk = storeProducts[product.id] else { return }
        let result = try await sk.purchase()
        switch result {
        case .success(let verification):
            if case .verified(let transaction) = verification {
                await transaction.finish()
            }
        default:
            break
        }
        await updateEntitlement()
    }

    public func restore() async throws {
        try await AppStore.sync()
        await updateEntitlement()
    }

    private func updateEntitlement() async {
        var pro = false
        for await result in Transaction.currentEntitlements {
            if case .verified(let transaction) = result, ProductCatalog.ids.contains(transaction.productID) {
                pro = true
            }
        }
        entitlement = pro ? .pro : .free
    }
}
#endif
