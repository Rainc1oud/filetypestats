GOOS ?= linux
GOARCH ?= amd64
BPFX := $(GOOS)-$(GOARCH)

# needed for sqlite dep, needs container builds for other archs
GOENV := CGO_ENABLED=1

.PHONY: clean
clean:
	rm -f *.sqlite
	rm -rf build/

.PHONY: testcli
testcli: build/$(BPFX)/testcli
build/$(BPFX)/testcli: internal/cmd/testcli/testcli.go
	$(GOENV) go build -v -o $@ $^