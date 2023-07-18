GOOS ?= linux
GOARCH ?= amd64
BPFX := $(GOOS)-$(GOARCH)

# needed for sqlite dep, needs container builds for other archs
GOENV := CGO_ENABLED=1 GO111MODULE="on"

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
	go test -v ./...

# catchall mkdir
%/:
	mkdir -p $@

.PHONY: testcli
testcli: build/$(BPFX)/testcli
build/linux-amd64/testcli: internal/cmd/testcli/testcli.go $(GOSRC)
	$(GOENV) go build -v -o $@ $<
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
