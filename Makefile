GOOS ?= linux
GOARCH ?= amd64
BPFX := $(GOOS)-$(GOARCH)

.PHONY: clean
clean:
	rm -f *.sqlite
	rm -rf build/

.PHONY: testcli
testcli: build/$(BPFX)/testcli
build/$(BPFX)/testcli: internal/cmd/testcli/testcli.go
	go build -v -o $@ $^