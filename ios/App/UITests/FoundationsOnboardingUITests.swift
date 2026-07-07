import XCTest

/// CP-08a XCUITest: the first-run onboarding gate + Foundations persistence (plan Part C point 14,
/// FR-1.4/11.4/11.6). A fresh launch auto-presents placement; a complete beginner is routed into
/// the Foundations course; answering cards records events that survive a relaunch (I5), and
/// onboarding never re-presents once placement is done.
final class FoundationsOnboardingUITests: XCTestCase {

    override func setUp() {
        continueAfterFailure = false
    }

    func testOnboardingRoutesBeginnerToFoundationsAndPersists() {
        // 1. Fresh launch with a clean store → placement auto-presents (no manual button).
        let app = XCUIApplication()
        app.launchArguments = ["-uiTestReset"]
        app.launch()

        let probe = app.staticTexts["persistenceProbe"]
        XCTAssertTrue(probe.waitForExistence(timeout: 20))
        XCTAssertTrue(probe.label.contains("events 0"), "clean store should start with no events")

        // 2. Complete-beginner path (FR-1.4): skip the word check → Foundations course.
        let beginner = app.buttons["I'm a complete beginner"]
        XCTAssertTrue(beginner.waitForExistence(timeout: 10), "onboarding should auto-present placement")
        beginner.tap()

        let header = app.otherElements["foundationsHeader"].firstMatch
        let listen = app.buttons["foundationsListen"]
        XCTAssertTrue(listen.waitForExistence(timeout: 10) || header.waitForExistence(timeout: 10),
                      "a complete beginner should land in the Foundations course")

        // 3. Answer a few cards: introduce → recognize → read → bind, recording events.
        for _ in 0..<8 {
            if app.buttons["foundationsIntroduceNext"].exists {
                app.buttons["foundationsIntroduceNext"].tap()
            } else if app.buttons["foundationsBind"].exists {
                app.buttons["foundationsBind"].tap()
            } else if app.buttons["foundationsCorrectOption"].firstMatch.exists {
                app.buttons["foundationsCorrectOption"].firstMatch.tap()
            } else if app.buttons["foundationsNextSet"].exists {
                app.buttons["foundationsNextSet"].tap()
            }
        }
        XCTAssertFalse(probe.label.contains("events 0"), "Foundations interactions should record events")

        // 4. Relaunch (no reset): onboarding must NOT re-present (placement persisted), and the
        //    recorded Foundations events must survive (I5).
        app.terminate()
        let relaunched = XCUIApplication()
        relaunched.launch()

        let probe2 = relaunched.staticTexts["persistenceProbe"]
        XCTAssertTrue(probe2.waitForExistence(timeout: 20))
        XCTAssertFalse(probe2.label.contains("events 0"),
                       "Foundations progress must persist across relaunch; probe = \(probe2.label)")
        XCTAssertFalse(relaunched.buttons["I'm a complete beginner"].waitForExistence(timeout: 3),
                       "placement must not re-present once onboarding is done")
    }
}
