.PHONY: test test-unit bench build build-ios app app-test audit clean

# Fast host tests (no simulator): ZhuwenPacks + ZhuwenCore + ZhuwenPersistence vs vendored Fixtures/.
test:
	swift test

# Unit tests only (GitHub Actions): skips the simulation / drift acceptance suites, which are
# integration/e2e and are guarded by the pre-push hook (.githooks/pre-push). The 50k-event replay
# perf suite (ReplayPerfTests) is likewise heavy and pre-push-gated; the launch-replay correctness
# test (LaunchReplayTests) stays in CI so I5 durability is verified on every push.
test-unit:
	swift test --skip PlacementSimulationTests --skip KaraokeDriftTests --skip ReplayPerfTests

# Network-surface CI gate (handoff §8, I2): URLSession only in PackClient, no third-party
# analytics/crash/ads SDKs, no cleartext http://, no secrets in UserDefaults.
audit:
	./grep-audit.sh

# NFR-2 selector benchmark: the 50 ms gate is only meaningful on an optimized build,
# so it is asserted under -c release (debug `swift test` applies a coarse regression guard).
bench:
	swift test -c release --filter SelectorTests/testGates5kStoryLatticeUnder50ms

# Compile all library targets on the host.
build:
	swift build

# Cross-compile the SwiftUI shell for the iOS 17 simulator (validates the app layer).
build-ios:
	xcodebuild -scheme ZhuwenUI -destination 'generic/platform=iOS Simulator' build

# MC-1: generate the @main app .xcodeproj from App/project.yml (XcodeGen, build-time only) and
# build it for the iOS 17 simulator — agent-buildable from a clean clone, no manual Xcode steps.
APP_DIR := App
APP_DEST ?= platform=iOS Simulator,name=iPhone 16
app:
	cd $(APP_DIR) && xcodegen generate
	xcodebuild -project $(APP_DIR)/ZhuwenApp.xcodeproj -scheme ZhuwenApp \
		-destination 'generic/platform=iOS Simulator' build

# MC-1 XCUITest smoke (place → read → lookup → relaunch persists). Needs a booted simulator.
app-test:
	cd $(APP_DIR) && xcodegen generate
	xcodebuild -project $(APP_DIR)/ZhuwenApp.xcodeproj -scheme ZhuwenApp \
		-destination '$(APP_DEST)' test

clean:
	swift package clean
	rm -rf .build
