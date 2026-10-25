GO ?= go
BENCHTIME ?= 1s

.PHONY: all test test-purego bench fuzz build-static lint

all: lint test test-purego

lint:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	$(GO) vet ./...

## test: run all tests (cgo and pure Go) with the race detector.
test:
	$(GO) test -race ./...

## test-purego: prove the port and the switch build without a C toolchain.
test-purego:
	CGO_ENABLED=0 $(GO) test ./purego/... ./codec/...
	$(GO) test -tags purego ./codec/...

## bench: side-by-side cgo vs pure Go benchmarks.
bench:
	$(GO) test -run '^$$' -bench . -benchmem -benchtime $(BENCHTIME) ./cgowrap/

## fuzz: run every differential fuzz target for FUZZTIME each.
FUZZTIME ?= 30s
fuzz:
	@for f in FuzzDiffCRC32C FuzzDiffAppendUvarint FuzzDiffUvarint FuzzDiffRLE FuzzDiffDecodeRLE; do \
		$(GO) test -run '^$$' -fuzz "^$$f$$" -fuzztime $(FUZZTIME) ./cgowrap/ || exit 1; \
	done

## build-static: build a fully static linux/amd64 binary with no cgo.
build-static:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags='-s -w' -o bin/crc32c-linux-amd64 ./cmd/crc32c
	@file bin/crc32c-linux-amd64 2>/dev/null || true
