.DEFAULT_GOAL := help

# Load .env when present so live, ui, chat, and agent see the keys. See .env.example.
-include .env
export

BUF_VERSION ?= 1.61.0
BIN := bin
BUF := $(BIN)/buf

.PHONY: help fmt lint test cover live vuln check ui chat agent generate bench

help: ## Show commands
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  make %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go code with gofmt
	gofmt -w .

lint: ## Check gofmt and run golangci-lint
	@test -z "$$(gofmt -l .)" || { echo "gofmt would change:"; gofmt -l .; exit 1; }
	golangci-lint run ./...
	cd tui && golangci-lint run ./...
	cd xmcp && golangci-lint run ./...
	cd x && golangci-lint run ./...

test: ## Run tests with the race detector
	go test -race ./...
	cd tui && go test -race ./...
	cd xmcp && go test -race ./...
	cd x && go test -race ./...

cover: ## Coverage of the SDK packages. Tests live in test/ folders, so -coverpkg is required
	go test -coverpkg=./... -coverprofile=cover.out ./... >/dev/null && go tool cover -func=cover.out | tail -1

live: ## Call the real API with XAI_API_KEY from .env. About $0.10 a run
	go test -tags live -count=1 -v ./test/live/
	cd xmcp && go test -tags live -count=1 -v -run Live\|Docs ./test/
	cd x && go test -tags live -count=1 -v ./test/

vuln: ## Check dependencies and the toolchain for known vulnerabilities
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	cd tui && go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	cd xmcp && go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	cd x && go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: ## Lint, test, and vulnerability check. Run this before a push
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) vuln

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

bench: ## Time the chat hot path
	go test -bench=. -benchtime=100ms -run '^$$' ./chat/test
