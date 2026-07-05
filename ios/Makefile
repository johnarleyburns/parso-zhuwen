.PHONY: test bench build build-ios clean

# Fast host tests (no simulator): ZhuwenPacks + ZhuwenCore vs vendored Fixtures/.
test:
	swift test

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
