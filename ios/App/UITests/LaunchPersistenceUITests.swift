import XCTest

/// MC-1 XCUITest smoke: the durable-log guarantee (I5) end-to-end through the real app UI.
/// Fresh launch → open a story → tap words (lookups) → relaunch the app → the lookup count
/// persisted (it is read back from the SwiftData `PersistentEventLog` at launch, not held in
/// memory). Complements the host-side `LaunchReplayTests`, which prove the projection equality.
final class LaunchPersistenceUITests: XCTestCase {

    override func setUp() {
        continueAfterFailure = false
    }

    func testLookupsSurviveRelaunch() {
        // 1. Fresh launch with a clean store.
        let app = XCUIApplication()
        app.launchArguments = ["-uiTestReset"]
        app.launch()

        let probe = app.staticTexts["persistenceProbe"]
        XCTAssertTrue(probe.waitForExistence(timeout: 20))
        XCTAssertTrue(probe.label.contains("lookups 0"), "clean store should start at zero lookups")

        // 2. Open today's story.
        let storyLink = app.buttons["todayStory"]
        XCTAssertTrue(storyLink.waitForExistence(timeout: 10))
        storyLink.tap()

        // 3. Tap a Chinese word token (records a lookup) and dismiss the gloss sheet.
        let token = app.buttons.matching(identifier: "readerToken").firstMatch
        if !token.waitForExistence(timeout: 10) {
            print("UI-HIERARCHY-DUMP:\n\(app.debugDescription)")
        }
        XCTAssertTrue(token.exists, "expected a tappable word token")
        token.tap()
        app.swipeDown()   // dismiss the gloss sheet if it presented

        // 4. Relaunch (no reset) — state must be reloaded from the persistent store.
        app.terminate()
        let relaunched = XCUIApplication()
        relaunched.launch()

        let probe2 = relaunched.staticTexts["persistenceProbe"]
        XCTAssertTrue(probe2.waitForExistence(timeout: 20))
        XCTAssertFalse(probe2.label.contains("lookups 0"),
                       "lookups recorded before relaunch must persist (I5); probe = \(probe2.label)")
    }
}
