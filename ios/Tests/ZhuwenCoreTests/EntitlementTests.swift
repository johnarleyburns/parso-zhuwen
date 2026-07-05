import XCTest
@testable import ZhuwenCore

final class EntitlementTests: XCTestCase {
    func testFreeAllowsOneStoryPerDay() {
        let gate = FeatureGate(entitlement: .free)
        XCTAssertTrue(gate.canOpenStory(storiesOpenedToday: 0))
        XCTAssertFalse(gate.canOpenStory(storiesOpenedToday: 1))
        XCTAssertFalse(gate.canOpenStory(storiesOpenedToday: 3))
    }

    func testProIsUnthrottled() {
        let gate = FeatureGate(entitlement: .pro)
        XCTAssertTrue(gate.canOpenStory(storiesOpenedToday: 0))
        XCTAssertTrue(gate.canOpenStory(storiesOpenedToday: 99))
        XCTAssertTrue(gate.isPro)
    }

    func testProUnlocksSurfacesFreeDoesNot() {
        let free = FeatureGate(entitlement: .free)
        let pro = FeatureGate(entitlement: .pro)
        for kp: KeyPath<FeatureGate, Bool> in [
            \.canBrowseLattice, \.canUseListeningPacks, \.canUseBlindMode,
            \.canSeeFullDashboard, \.canSeeMonthlyCheckpoint, \.canDownloadAllPacks
        ] {
            XCTAssertFalse(free[keyPath: kp])
            XCTAssertTrue(pro[keyPath: kp])
        }
    }

    func testReviewCapTighterForFree() {
        XCTAssertEqual(FeatureGate(entitlement: .free).reviewCap, FeatureGate.freeReviewCap)
        XCTAssertEqual(FeatureGate(entitlement: .pro).reviewCap, FeatureGate.proReviewCap)
        XCTAssertLessThan(FeatureGate.freeReviewCap, FeatureGate.proReviewCap)
    }

    func testProductCatalogMatchesFR93() {
        // FR-9.3: $7.99/mo, $59.99/yr (30-day trial), $149.99 lifetime; StoreKit 2 only.
        XCTAssertEqual(ProductCatalog.monthly.displayPrice, "$7.99")
        XCTAssertEqual(ProductCatalog.annual.displayPrice, "$59.99")
        XCTAssertEqual(ProductCatalog.annual.trialDays, 30)
        XCTAssertEqual(ProductCatalog.lifetime.displayPrice, "$149.99")
        XCTAssertEqual(ProductCatalog.monthly.trialDays, 0)
        XCTAssertEqual(ProductCatalog.lifetime.trialDays, 0)
        XCTAssertEqual(ProductCatalog.all.count, 3)
        XCTAssertEqual(ProductCatalog.ids,
                       ["ai.zhuwen.pro.monthly", "ai.zhuwen.pro.annual", "ai.zhuwen.pro.lifetime"])
    }
}
