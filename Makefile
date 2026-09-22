GO  ?= go
BIN := bin/tarsier

DEADCODE  := golang.org/x/tools/cmd/deadcode@v0.50.0
MODERNIZE := golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@v0.23.0
GOVULNCHK := golang.org/x/vuln/cmd/govulncheck@v1.7.0

# `go run pkg@version` builds the tool under that module's go line (x/tools
# v0.50 -> 1.26). Pin to go.mod's go line so analysis can load packages that
# need a newer Go, and so GOTOOLCHAIN=local on an older host go still selects
# the module toolchain (GOVERSION collapses to the bootstrap under local).
GO_LINE := $(shell sed -n 's/^go //p' go.mod | head -1)
run_tool = GOTOOLCHAIN=go$(GO_LINE) $(GO) run

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

# Do not capture stderr: cold-cache `go: downloading ...` noise must not
# trip the non-empty-stdout deadcode check. Tool errors still fail via status.
dead:
	@out="$$($(run_tool) $(DEADCODE) -test ./...)"; \
	status=$$?; \
	if [ $$status -ne 0 ]; then exit $$status; fi; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

modern:
	$(run_tool) $(MODERNIZE) ./...

ci: lint dead modern rules test vuln checkup

clean:
	rm -rf bin coverage.out
