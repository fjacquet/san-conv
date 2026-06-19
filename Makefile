# Canonical Go Makefile — fjacquet/ci standard interface (do not rename targets)
.DEFAULT_GOAL := all
DIST  ?= dist
COVER ?= coverage.out
GOLANGCI_VERSION ?= v2.12.2
GORELEASER_VERSION ?= v2.16.0

BINARY := san-conv
MODULE := github.com/fjacquet/san-conv
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

# Lint only project packages (./... hits indirect dep module cache in golangci-lint v2)
PKGS ?= ./cmd/... ./internal/...

.PHONY: all clean install tools lint format test build vuln sbom security docs coverage-upload release ci \
        snapshot run-mds run-brocade help

all: clean lint test build

clean:
	rm -rf $(DIST) site $(COVER) *.sarif
	rm -f $(BINARY)

install:
	go mod download

tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

lint:
	golangci-lint run --timeout=5m $(PKGS)

format:
	golangci-lint fmt

test:
	go test -race -coverprofile=$(COVER) -covermode=atomic ./...

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

sbom:
	mkdir -p $(DIST)
	go run github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest mod -json -output $(DIST)/sbom.cdx.json

security:  # advisory: reports findings but never blocks the build (CodeQL/osv are the blocking gates)
	uvx semgrep scan --config auto --skip-unknown-extensions || true

docs:
	uvx --with mkdocs-material --with pymdown-extensions mkdocs build --strict --site-dir site

coverage-upload:
	uvx --from codecov-cli codecov upload-process --file $(COVER) || true

release:
	goreleaser release --clean

ci: lint test build vuln

# ---------------------------------------------------------------------------
# Convenience targets (preserved from original Makefile)
# ---------------------------------------------------------------------------

snapshot: ## Build cross-platform snapshot binaries (no publish)
	goreleaser release --snapshot --clean

run-mds: build ## Run mds2brocade conversion (INPUT= required)
	@test -n "$(INPUT)" || (echo "Usage: make run-mds INPUT=path/to/mds.cfg [OUTPUT=out.fos]" && exit 1)
	./$(BINARY) mds2brocade $(INPUT) $(if $(OUTPUT),--output $(OUTPUT)) $(if $(SCRIPT),--script $(SCRIPT))

run-brocade: build ## Run brocade2mds conversion (INPUT= required)
	@test -n "$(INPUT)" || (echo "Usage: make run-brocade INPUT=path/to/brocade.cfg [OUTPUT=out.mds]" && exit 1)
	./$(BINARY) brocade2mds $(INPUT) $(if $(OUTPUT),--output $(OUTPUT))

help:
	@echo "Usage: make [target]"
