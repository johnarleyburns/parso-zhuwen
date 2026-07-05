.PHONY: test vet fmt build fixtures ci clean

test:
	go test ./...

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
