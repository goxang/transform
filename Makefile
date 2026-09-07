# Everything CI runs has a target here, so a red build can be reproduced
# locally with one command instead of by reading a workflow file.

SHELL := /usr/bin/env bash
GO ?= go
GOLANGCI_LINT_VERSION ?= v2.13.2
COVERAGE_THRESHOLD ?= 85
BENCH_COUNT ?= 6

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN { FS = ":.*## " } /^[a-zA-Z0-9_.-]+:.*## / { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: all
all: tidy-check fmt-check vet test lint go.test.coverage ## Run everything CI runs, except fuzzing

.PHONY: test
test: ## Run the test suite with the race detector
	$(GO) test -race -shuffle=on -count=1 ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format the tree
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check: ## Fail if anything is unformatted
	@out=$$(gofmt -l .); \
	if [[ -n "$$out" ]]; then echo "not gofmt'd:"; echo "$$out"; exit 1; fi

.PHONY: tidy-check
tidy-check: ## Fail if go.mod or go.sum would change
	@cp go.mod go.mod.bak; [[ -f go.sum ]] && cp go.sum go.sum.bak || true; \
	$(GO) mod tidy; \
	status=0; \
	if ! diff -q go.mod go.mod.bak >/dev/null; then echo "go mod tidy changed go.mod; commit the result"; status=1; fi; \
	if [[ -f go.sum.bak ]] && ! diff -q go.sum go.sum.bak >/dev/null; then echo "go mod tidy changed go.sum; commit the result"; status=1; fi; \
	mv go.mod.bak go.mod; [[ -f go.sum.bak ]] && mv go.sum.bak go.sum || true; \
	exit $$status

.PHONY: lint-deps
lint-deps: ## Install the pinned golangci-lint
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: lint
lint: ## Run golangci-lint (install it with `make lint-deps`)
	golangci-lint run

.PHONY: go.test.coverage
go.test.coverage: ## Run tests with coverage and enforce the threshold
	$(GO) test -coverprofile=coverage.out -covermode=atomic .
	$(GO) tool cover -func=coverage.out
	@$(GO) tool cover -func=coverage.out | awk '/^total:/ { sub(/%/, "", $$3); \
		if ($$3 + 0 < $(COVERAGE_THRESHOLD)) { printf "coverage %.1f%% below $(COVERAGE_THRESHOLD)%% gate\n", $$3; exit 1 } }'

.PHONY: go-benchmark
go-benchmark: ## Run the benchmarks
	$(GO) test -run='^$$' -bench=. -benchmem -count=$(BENCH_COUNT) .

.PHONY: go-benchmark-compare
go-benchmark-compare: ## Compare benchmarks against BASE_REF (default origin/main)
	./tools/hack/go-benchmark-compare.sh

.PHONY: fuzz
fuzz: ## Run every fuzz target for FUZZTIME (default 30s)
	@for f in $$($(GO) test -list='^Fuzz' -run='^$$' . | grep '^Fuzz'); do \
		echo "fuzzing $$f"; \
		$(GO) test -fuzz="^$$f\$$" -fuzztime=$${FUZZTIME:-30s} -run='^$$' . || exit 1; \
	done

.PHONY: clean
clean: ## Remove build and test artifacts
	rm -f coverage.out
	$(GO) clean -testcache
