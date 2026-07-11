GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BPFX := $(GOOS)-$(GOARCH)
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

# keep builds pure Go; the sqlite dependency uses modernc.org/sqlite.
GOENV := CGO_ENABLED=0 GO111MODULE="on"

### build container settings
DOCKEREXE := $(shell command -v podman)
# translation list from target arch in GOARCH format to glibc-march tags of build containers
CMARCHLIST := arm-glibc2.17 arm64-glibc2.19 amd64-glibc2.31
CMARCH = $(filter $(GOARCH)-%,$(CMARCHLIST))
$(info CMARCH==$(CMARCH))
IMGNAME = rcbuild-go:$(CMARCH)-go1.20.1
DOCKERPULL = $(DOCKEREXE) pull --tls-verify=false docker://1nnoserv:15000/xbuildimg/$(IMGNAME)

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
test:
	$(GOENV) go test -v ./...

.PHONY: test-benchmark
test-benchmark: test-benchmark-csv

.PHONY: test-benchmark-raw
test-benchmark-raw:
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" $(GOENV) go test -run '^$$' -bench '$(BENCH_PATTERN)' -benchtime '$(BENCH_TIME)' -benchmem $(BENCH_PKG)

.PHONY: test-benchmark-csv
test-benchmark-csv: $(BENCH_OUT_DIR)/
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" $(GOENV) go test -run '^$$' -bench '$(BENCH_PATTERN)' -benchtime '$(BENCH_TIME)' -benchmem $(BENCH_PKG) | tee "$(BENCH_RAW)"
	awk -v driver="$(SQL_DRIVER)" -v datetime="$(BENCH_DATETIME)" -f scripts/bench-to-csv.awk "$(BENCH_RAW)" > "$(BENCH_CSV)"
	@echo "wrote $(BENCH_CSV)"

.PHONY: benchmark-chart
benchmark-chart: $(BENCH_OUT_DIR)/
	awk -v out="$(BENCH_CHART)" -f scripts/bench-csv-chart.awk $(BENCH_CHART_INPUTS)

.PHONY: test-stress
test-stress:
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" $(GOENV) go test -run '^$$' -bench 'Benchmark(WalkFileTypeStatsDBRealTree|TreeStatsWatcherScanDirRealTree)' -benchtime '$(BENCH_TIME)' -benchmem .

# catchall mkdir
%/:
	mkdir -p $@

.PHONY: testcli testcli-native
testcli: testcli-native
testcli-native: build/$(BPFX)/testcli
build/$(BPFX)/testcli: internal/cmd/testcli/testcli.go $(GOSRC) | build/$(BPFX)/
	$(GOENV) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -v -o $@ $<
build/linux-%/testcli: internal/cmd/testcli/testcli.go internal/cmd/testcli/testcli.go $(GOSRC) | build/ .tmp/%/
	$(DOCKERPULL)
	$(DOCKEREXE) run --rm \
		-v $(CURDIR):/buildroot \
		-v $(CURDIR)/build:/build/ \
		-v $(CURDIR)/.tmp/$*:/gotmp \
		-e GOPROXY \
		-e GONOSUMDB \
		-e GOMODCACHE="/gotmp/.gomodcache/pkg/mod" \
		-e GOCACHE="/gotmp/.gocache/go-build" \
		-e GOPATH="/gotmp/.go" \
		-w /buildroot \
		$(IMGNAME) bash -c '. /etc/environment; $(GOENV) go get -v -u ./...; $(GOENV) go build -v -o $@ $<'
