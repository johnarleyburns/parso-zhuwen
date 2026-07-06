import XCTest
import ZhuwenCore
import ZhuwenPacks
@testable import ZhuwenUI

@MainActor
final class StoreModelTests: XCTestCase {
    func testDefaultProviderIsFree() {
        let store = StoreModel(provider: InMemoryEntitlementProvider(.free))
        XCTAssertFalse(store.isPro)
        XCTAssertFalse(store.gate.canBrowseLattice)
        XCTAssertEqual(store.products.count, 3)
    }

    func testPurchaseFlipsToPro() async {
        let store = StoreModel(provider: InMemoryEntitlementProvider(.free))
        await store.purchase(ProductCatalog.annual)
        XCTAssertTrue(store.isPro)
        XCTAssertTrue(store.gate.canOpenStory(storiesOpenedToday: 5))
        XCTAssertNil(store.lastError)
    }
}

@MainActor
final class SyncModelTests: XCTestCase {
    func testDefaultIsOffAndNoOpDoesNothing() async {
        let sync = SyncModel(enabled: false, engine: NoOpSyncEngine())
        XCTAssertFalse(sync.enabled)
        await sync.push(LearnerArchive(events: [], seed: [:]))
        let pulled = await sync.pull()
        XCTAssertNil(pulled)
        XCTAssertNil(sync.lastSyncedAt)
    }
}

@MainActor
final class LearnerModelDataTests: XCTestCase {
    private func makeLearner() -> LearnerModel {
        LearnerModel(stories: [], lexicon: [], seed: PlacementSeed([9: 0.6]))
    }

    func testExportEraseImportRoundTripsThroughLearnerModel() throws {
        let learner = makeLearner()
        let t0 = Date(timeIntervalSince1970: 1_700_000_000)
        learner.exposure(1, storyID: "s1", at: t0)
        learner.lookup(2, storyID: "s1", at: t0.addingTimeInterval(1))
        learner.record(.reviewGrade(1, grade: Rating.good.rawValue, at: t0.addingTimeInterval(2)))

        let before = learner.model
        let json = try learner.exportJSON(at: t0)

        learner.eraseAll()
        XCTAssertTrue(learner.events.isEmpty)
        // Erase clears history; the placement seed prior is retained until an import replaces it.
        XCTAssertEqual(learner.model, KnownWordModel.project([], seed: [9: 0.6]))

        try learner.importJSON(json)
        XCTAssertEqual(learner.model, before)
        XCTAssertEqual(learner.events.count, 3)
    }
}
