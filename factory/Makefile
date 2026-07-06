.PHONY: test test-unit test-e2e vet fmt build fixtures ci clean

test:
	go test ./...

# Unit tests only (GitHub Actions): the package units under internal/. The cmd/ e2e suite is
# guarded by the pre-push hook (.githooks/pre-push), so only good builds get pushed.
test-unit:
	go test ./internal/...

# e2e (cmd/zhuwenctl): pipeline resumability + verify round-trip. Guarded on git push.
test-e2e:
	go test ./cmd/...

vet:
	go vet ./...

fmt:
	gofmt -w .
	gofmt -l .

# EC-2: build every command into ./bin, runnable as ./bin/<cmd>.
build:
	mkdir -p bin
	go build -o bin/ ./cmd/...

clean:
	rm -rf bin

# Regenerate the vendored iOS golden fixtures (DEV-signed, reproducible):
# positive pack + pubkey + golden-{unsigned,tampered,imageless}.zpack.
fixtures:
	go run ./cmd/genfixtures --out ../ios/Fixtures
	go run ./cmd/zhuwenctl verify ../ios/Fixtures/fixture-a2-v0.zpack --pub ../ios/Fixtures/zhuwen-dev.pub

ci: fmt vet test
