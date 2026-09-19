GO  ?= go
BIN := bin/tarsier

DEADCODE  := golang.org/x/tools/cmd/deadcode@v0.50.0
MODERNIZE := golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@v0.23.0
GOVULNCHK := golang.org/x/vuln/cmd/govulncheck@v1.7.0

# `go run pkg@version` builds the tool under that module's go line (x/tools
# v0.50 → 1.26). Pin to this module's toolchain so analysis can load packages
# that require a newer Go (go.mod: 1.27.1).
run_tool = GOTOOLCHAIN=$$($(GO) env GOVERSION) $(GO) run

.PHONY: help build test rules cover lint fmt tidy vuln dead modern checkup ci clean

help:
	@grep -h '^\.PHONY:' $(MAKEFILE_LIST) | cut -d' ' -f2- | tr ' ' '\n'

build:
	$(GO) build -o $(BIN) ./cmd/tarsier

test:
	$(GO) test -race -shuffle=on ./...

rules:
	ast-grep test

checkup:
	./scripts/otel-checkup.sh fixtures/otel-checkup/collector-ok.yaml
	@./scripts/otel-checkup.sh fixtures/otel-checkup/collector-bad.yaml >/dev/null 2>&1; \
		test $$? -ne 0

cover:
	$(GO) test -race -covermode=atomic -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run

fmt:
	golangci-lint fmt

tidy:
	$(GO) mod tidy

vuln:
	$(run_tool) $(GOVULNCHK) ./...

dead:
	@out="$$($(run_tool) $(DEADCODE) -test ./...)"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

modern:
	$(run_tool) $(MODERNIZE) ./...

ci: lint dead modern rules test vuln checkup

clean:
	rm -rf bin coverage.out
