.PHONY: test test-unit bench build build-ios audit clean

# Fast host tests (no simulator): ZhuwenPacks + ZhuwenCore vs vendored Fixtures/.
test:
	swift test

# Unit tests only (GitHub Actions): skips the simulation / drift acceptance suites, which are
# integration/e2e and are guarded by the pre-push hook (.githooks/pre-push).
test-unit:
	swift test --skip PlacementSimulationTests --skip KaraokeDriftTests

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

clean:
	swift package clean
	rm -rf .build
