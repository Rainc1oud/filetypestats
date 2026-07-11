GOOS ?= linux
GOARCH ?= amd64
BPFX := $(GOOS)-$(GOARCH)
BENCH_ROOT ?= ..
BENCH_TIME ?= 1x
BENCH_PATTERN ?= .
BENCH_PKG ?= ./...
SQL_DRIVER ?= sqlite3
BENCH_DATETIME := $(shell date +%Y%m%d-%H%M%S)
BENCH_OUT_DIR ?= _tests/benchmark
BENCH_CSV ?= $(BENCH_OUT_DIR)/benchmark-$(SQL_DRIVER)-$(BENCH_DATETIME).csv
BENCH_RAW ?= $(BENCH_OUT_DIR)/benchmark-$(SQL_DRIVER)-$(BENCH_DATETIME).txt

# needed for sqlite dep, needs container builds for other archs
GOENV := CGO_ENABLED=1 GO111MODULE="on"

### build container settings
DOCKEREXE := $(shell command -v podman)
# translation list from target arch in GOARCH format to glibc-march tags of build containers
CMARCHLIST := arm-glibc2.17 arm64-glibc2.19 amd64-glibc2.31
CMARCH = $(filter $(GOARCH)-%,$(CMARCHLIST))
$(info CMARCH==$(CMARCH))
IMGNAME = rcbuild-go:$(CMARCH)-go1.25.6
DOCKERREPO ?= docker://1nnoserv:15000/xbuildimg
DOCKERPULL = $(DOCKEREXE) pull --tls-verify=false $(DOCKERREPO)/$(IMGNAME)

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
	go test -v ./...

.PHONY: test-benchmark
test-benchmark: test-benchmark-csv

.PHONY: test-benchmark-raw
test-benchmark-raw:
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" go test -run '^$$' -bench '$(BENCH_PATTERN)' -benchtime '$(BENCH_TIME)' -benchmem $(BENCH_PKG)

.PHONY: test-benchmark-csv
test-benchmark-csv: $(BENCH_OUT_DIR)/
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" go test -run '^$$' -bench '$(BENCH_PATTERN)' -benchtime '$(BENCH_TIME)' -benchmem $(BENCH_PKG) | tee "$(BENCH_RAW)"
	awk -v driver="$(SQL_DRIVER)" -v datetime="$(BENCH_DATETIME)" -f scripts/bench-to-csv.awk "$(BENCH_RAW)" > "$(BENCH_CSV)"
	@echo "wrote $(BENCH_CSV)"

.PHONY: test-stress
test-stress:
	FILETYPESTATS_BENCH_ROOT="$(BENCH_ROOT)" go test -run '^$$' -bench 'Benchmark(WalkFileTypeStatsDBRealTree|TreeStatsWatcherScanDirRealTree)' -benchtime '$(BENCH_TIME)' -benchmem .

# catchall mkdir
%/:
	mkdir -p $@

.PHONY: testcli
testcli: build/$(BPFX)/testcli
build/linux-amd64/testcli: internal/cmd/testcli/testcli.go $(GOSRC)
	$(GOENV) go build -v -o $@ $<
build/linux-%/testcli: internal/cmd/testcli/testcli.go internal/cmd/testcli/testcli.go $(GOSRC) | build/ .tmp/%/
	$(DOCKERPULL) --platform linux/$(GOARCH) 
	$(DOCKEREXE) run --platform linux/$(GOARCH) --rm \
		-v $(CURDIR):/buildroot \
		-v $(CURDIR)/build:/build/ \
		-v $(CURDIR)/.tmp/$*:/gotmp \
		-e GOPROXY \
		-e GONOSUMDB \
		-e GOMODCACHE="/gotmp/.gomodcache/pkg/mod" \
		-e GOCACHE="/gotmp/.gocache/go-build" \
		-e GOPATH="/gotmp/.go" \
		-w /buildroot \
		$(DOCKERREPO)/$(IMGNAME) bash -c '. /etc/environment; $(GOENV) go get -v -u ./...; $(GOENV) go build -v -o $@ $<'
