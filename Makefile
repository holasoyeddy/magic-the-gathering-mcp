.PHONY: build build-check check check-full fuzz lint race run test vet

PACKAGES ?= ./...
GO_TEST_FLAGS ?=

check: lint vet test build-check

check-full: check race fuzz

test:
	go test $(GO_TEST_FLAGS) $(PACKAGES)

lint:
	golangci-lint run ./...

vet:
	go vet ./...

race:
	go test -race $(GO_TEST_FLAGS) $(PACKAGES)

fuzz:
	go test ./internal/cardcache -run=^$$ -fuzz=FuzzCacheRecordDecodeNeverPanics -fuzztime=2s
	go test ./internal/scryfall -run=^$$ -fuzz=FuzzRepresentativeCardJSONMissingOptionalFieldsNeverPanics -fuzztime=2s
	go test ./internal/server -run=^$$ -fuzz=FuzzCursorDecodeNeverPanics -fuzztime=2s

run:
	go run ./cmd/magic-the-gathering-mcp

build:
	mkdir -p ./bin
	go build -o ./bin/magic-the-gathering-mcp ./cmd/magic-the-gathering-mcp

build-check:
	@build_dir=$$(mktemp -d); \
	trap 'rm -rf "$$build_dir"' EXIT; \
	go build -o "$$build_dir/magic-the-gathering-mcp" ./cmd/magic-the-gathering-mcp
