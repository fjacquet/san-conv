BINARY    := san-conv
MODULE    := github.com/fjacquet/san-conv
BUILD_DIR := dist

VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS   := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

GO        := go
# Lint only project packages (./... hits indirect dep module cache in golangci-lint v2)
PKGS      := ./cmd/... ./internal/...
GOLINT    := golangci-lint
GORELEASER:= goreleaser

.DEFAULT_GOAL := help

# ─── build ────────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build binary for current platform
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY) .

.PHONY: install
install: ## Install binary to $GOPATH/bin
	CGO_ENABLED=0 $(GO) install -ldflags "$(LDFLAGS)" $(MODULE)

.PHONY: snapshot
snapshot: ## Build cross-platform snapshot binaries (no publish)
	$(GORELEASER) release --snapshot --clean

.PHONY: release
release: ## Cut a release via goreleaser (requires GITHUB_TOKEN)
	$(GORELEASER) release --clean

# ─── test & quality ───────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all tests
	$(GO) test ./...

.PHONY: test-v
test-v: ## Run all tests with verbose output
	$(GO) test -v ./...

.PHONY: test-race
test-race: ## Run tests with race detector
	$(GO) test -race ./...

.PHONY: cover
cover: ## Run tests and show coverage report
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

.PHONY: lint
lint: ## Run golangci-lint on project packages
	$(GOLINT) run $(PKGS)

.PHONY: fmt
fmt: ## Format source with gofmt
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: check
check: fmt vet lint test ## Run all quality checks (fmt + vet + lint + test)

# ─── dev helpers ──────────────────────────────────────────────────────────────

.PHONY: run-mds
run-mds: build ## Run mds2brocade conversion (INPUT= required)
	@test -n "$(INPUT)" || (echo "Usage: make run-mds INPUT=path/to/mds.cfg [OUTPUT=out.fos]" && exit 1)
	./$(BINARY) mds2brocade $(INPUT) $(if $(OUTPUT),--output $(OUTPUT)) $(if $(SCRIPT),--script $(SCRIPT))

.PHONY: run-brocade
run-brocade: build ## Run brocade2mds conversion (INPUT= required)
	@test -n "$(INPUT)" || (echo "Usage: make run-brocade INPUT=path/to/brocade.cfg [OUTPUT=out.mds]" && exit 1)
	./$(BINARY) brocade2mds $(INPUT) $(if $(OUTPUT),--output $(OUTPUT))

# ─── clean ────────────────────────────────────────────────────────────────────

.PHONY: clean
clean: ## Remove built binary and dist/
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR) coverage.out

# ─── help ─────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: make \033[36m<target>\033[0m\n\nTargets:\n"} \
	/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
