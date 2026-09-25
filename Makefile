# with ONESHELL all lines of a recipe are executed in one shell; -e makes rules actually fail on error
.ONESHELL:
.SHELLFLAGS := -ec

BENCH_ROOT ?= ..
BENCH_TIME ?= 1x
BENCH_PATTERN ?= .
BENCH_PKG ?= ./...
SQL_DRIVER ?= modernc-sqlite
BENCH_DATETIME := $(shell date +%Y%m%d-%H%M%S)
BENCH_OUT_DIR ?= _tests/benchmark
BENCH_CSV ?= $(BENCH_OUT_DIR)/benchmark-$(SQL_DRIVER)-$(BENCH_DATETIME).csv
BENCH_RAW ?= $(BENCH_OUT_DIR)/benchmark-$(SQL_DRIVER)-$(BENCH_DATETIME).txt
BENCH_CHART ?= $(BENCH_OUT_DIR)/benchmark-comparison.svg
BENCH_CHART_INPUTS ?= $(wildcard $(BENCH_OUT_DIR)/benchmark-*.csv)
BENCH_LABEL ?= baseline
BENCH_OPT_OUT_DIR ?= _tests/benchmark-optimize
BENCH_OPT_CSV ?= $(BENCH_OPT_OUT_DIR)/benchmark-$(BENCH_LABEL)-$(BENCH_DATETIME).csv
BENCH_OPT_RAW ?= $(BENCH_OPT_OUT_DIR)/benchmark-$(BENCH_LABEL)-$(BENCH_DATETIME).txt
BENCH_OPT_CHART ?= $(BENCH_OPT_OUT_DIR)/benchmark-comparison.svg
BENCH_OPT_CHART_INPUTS ?= $(wildcard $(BENCH_OPT_OUT_DIR)/benchmark-*.csv)

GOCMD := go
export GOWORK=off# build against this go.mod, not an enclosing go.work (rc-integration)

# golangci-lint: prefer whatever's already on PATH (e.g. the nix-pinned devenv one); if
# missing, fall back to `go run`.
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
ifeq ($(GOLANGCI_LINT),)
  GOLANGCI_LINT := $(GOCMD) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
endif

# std Makefile stuff
GOSRC := $(wildcard *.go types/*.go ftsdb/*.go treestatsquery/*.go internal/cmd/testcli/*.go)
$(info GOSRC: $(GOSRC))

.PHONY: all
all: testcli

.PHONY: clean
clean:
	# rm -f *.sqlite
	rm -rf build/

.PHONY: test
test: ## run all go tests
	$(GOCMD) run gotest.tools/gotestsum@latest --format testdox -- ./...

.PHONY: check
check: ## static analysis (golangci-lint; its default set includes govet)
	$(GOLANGCI_LINT) run ./...

.PHONY: test-coverage
test-coverage: ## test coverage report (cover.report.html)
	$(GOCMD) test -covermode count -coverprofile cover.report.out ./...
	$(GOCMD) tool cover -html=cover.report.out -o cover.report.html

# Deliberately separate from test and CI: the race detector needs a C toolchain.
.PHONY: test-race
test-race:
	$(GOCMD) test -race ./...

.PHONY: test-benchmark
test-benchmark: test-benchmark-csv

.PHONY: test-benchmark-raw
test-benchmark-raw:
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" $(GOCMD) test -run '^$$' -bench '$(BENCH_PATTERN)' -benchtime '$(BENCH_TIME)' -benchmem $(BENCH_PKG)

.PHONY: test-benchmark-csv
test-benchmark-csv: $(BENCH_OUT_DIR)/
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" $(GOCMD) test -run '^$$' -bench '$(BENCH_PATTERN)' -benchtime '$(BENCH_TIME)' -benchmem $(BENCH_PKG) | tee "$(BENCH_RAW)"
	awk -v driver="$(SQL_DRIVER)" -v datetime="$(BENCH_DATETIME)" -f scripts/bench-to-csv.awk "$(BENCH_RAW)" > "$(BENCH_CSV)"
	@echo "wrote $(BENCH_CSV)"

.PHONY: benchmark-chart
benchmark-chart: $(BENCH_OUT_DIR)/
	awk -v out="$(BENCH_CHART)" -f scripts/bench-csv-chart.awk $(BENCH_CHART_INPUTS)

# targets below exercise the BenchmarkOptimize* suite (ftsdb_optimize_benchmark_test.go,
# filetypestats_optimize_benchmark_test.go), which specifically isolates the write-path
# performance issues found during review (no prepared statements, no PRAGMA tuning, the
# per-row category subquery, unbounded conn pool, unlocked live-watch writes racing the
# scan, missing index on fileinfo.updated). Use BENCH_LABEL=baseline before optimizing and
# BENCH_LABEL=optimized after, then run benchmark-optimize-chart to compare the two.
.PHONY: test-benchmark-optimize-csv
test-benchmark-optimize-csv: $(BENCH_OPT_OUT_DIR)/
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" $(GOCMD) test -run '^$$' -bench 'Optimize' -benchtime '$(BENCH_TIME)' -benchmem $(BENCH_PKG) | tee "$(BENCH_OPT_RAW)"
	awk -v driver="$(BENCH_LABEL)" -v datetime="$(BENCH_DATETIME)" -f scripts/bench-to-csv.awk "$(BENCH_OPT_RAW)" > "$(BENCH_OPT_CSV)"
	@echo "wrote $(BENCH_OPT_CSV)"

.PHONY: benchmark-optimize-chart
benchmark-optimize-chart: $(BENCH_OPT_OUT_DIR)/
	awk -v out="$(BENCH_OPT_CHART)" -v title="Optimization Benchmark Comparison (baseline vs optimized)" -f scripts/bench-csv-chart.awk $(BENCH_OPT_CHART_INPUTS)

.PHONY: test-stress
test-stress:
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" $(GOCMD) test -run '^$$' -bench 'Benchmark(WalkFileTypeStatsDBRealTree|TreeStatsWatcherScanDirRealTree)' -benchtime '$(BENCH_TIME)' -benchmem .

# catchall mkdir
%/:
	mkdir -p $@

.PHONY: testcli
testcli: build/testcli
build/testcli: internal/cmd/testcli/testcli.go $(GOSRC) | build/
	$(GOCMD) build -v -o $@ $<
