GOOS ?= linux
GOARCH ?= amd64
BPFX := $(GOOS)-$(GOARCH)

# needed for sqlite dep, needs container builds for other archs
GOENV := CGO_ENABLED=1 GO111MODULE="on"

### build container settings
DOCKEREXE := $(shell command -v podman)
# translation list from target arch in GOARCH format to glibc-march tags of build containers
CMARCHLIST := arm_glibc-2.19-armhf arm64_glibc-2.19-aarch64 amd64_unknown_x86_64
CMARCH = $(word 2, $(subst _, ,$(filter $(GOARCH)_%,$(CMARCHLIST))))
$(info CMARCH==$(CMARCH))
IMGNAME = rcbuild-go:1.16.8-$(CMARCH)
DOCKERPULL = $(DOCKEREXE) pull --tls-verify=false docker://1nnoserv:15000/xbuildenv/$(IMGNAME)

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

.PHONY: testcli
testcli: build/$(BPFX)/testcli
build/linux-amd64/testcli: internal/cmd/testcli/testcli.go $(GOSRC)
	$(GOENV) go build -v -o $@ $<
build/linux-arm/testcli: internal/cmd/testcli/testcli.go internal/cmd/testcli/testcli.go $(GOSRC)
	$(DOCKERPULL)
	[[ -d "$(CURDIR)/build" ]] || mkdir -p "$(CURDIR)/build"
	$(DOCKEREXE) run --rm -v $(CURDIR):/buildroot -v $(CURDIR)/build:/build/ -w /buildroot $(IMGNAME) bash -c '. /etc/environment; $(GOENV) go get -v -u ./...; $(GOENV) go build -v -o $@ $<'
build/linux-arm64/testcli: internal/cmd/testcli/testcli.go internal/cmd/testcli/testcli.go $(GOSRC)
	$(DOCKERPULL)
	[[ -d "$(CURDIR)/build" ]] || mkdir -p "$(CURDIR)/build"
	$(DOCKEREXE) run --rm -v $(CURDIR):/buildroot -v $(CURDIR)/build:/build/ -w /buildroot $(IMGNAME) bash -c '. /etc/environment; $(GOENV) go get -v -u ./...; $(GOENV) go build -v -o $@ $<'