.DEFAULT_GOAL := help

BUF_VERSION ?= 1.61.0
BIN := bin
BUF := $(BIN)/buf

.PHONY: help fmt lint test check ui chat agent generate bench

help: ## Show commands
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  make %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go code with gofmt
	gofmt -w .

lint: ## Check gofmt and run golangci-lint
	@test -z "$$(gofmt -l .)" || { echo "gofmt would change:"; gofmt -l .; exit 1; }
	golangci-lint run ./...
	cd tui && golangci-lint run ./...

test: ## Run tests
	go test ./...
	cd tui && go test ./...

check: ## Lint and test. Run this before a push
	$(MAKE) lint
	$(MAKE) test

ui: ## Open the terminal chat
	cd tui && go run ./cmd/xai

chat: ## Run the chat example
	go run ./examples/chat

agent: ## Run the agent example
	go run ./examples/agent

generate: $(BUF) ## Regenerate code from ../xai-proto
	cd ../xai-proto && "$(CURDIR)/$(BUF)" generate --template "$(CURDIR)/buf.gen.yaml" --path proto/xai/api/v1

$(BUF):
	mkdir -p $(BIN)
	curl -fsSL -o $(BUF) "https://github.com/bufbuild/buf/releases/download/v$(BUF_VERSION)/buf-$$(uname -s)-$$(uname -m)"
	chmod +x $(BUF)

bench: ## Time the client and agent hot path
	go test -bench=. -benchtime=100ms -run '^$$' ./agent/test ./chat/test
