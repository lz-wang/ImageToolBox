.PHONY: build bins verify-bins build-full check test test-unit serve clean

VERSION := $(shell date +%Y-%m-%d)-$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

build:
	go build $(LDFLAGS) -o itb .

bins:
ifeq ($(GOOS),linux)
	./scripts/build-linux-bins-container.sh $(GOARCH) bins/linux-$(GOARCH)
else
	@echo "bins target currently supports Linux only"
	@exit 1
endif

verify-bins:
ifeq ($(GOOS),linux)
	./scripts/verify-linux-abi.sh $(GOARCH) bins/linux-$(GOARCH)
else
	@echo "verify-bins target currently supports Linux only"
	@exit 1
endif

build-full: bins verify-bins
	go build $(LDFLAGS) -o itb .

check:
	go vet ./...

test:
	go test ./...

test-unit:
	go test ./... -skip '^TestMinIO(Integration|CLIE2E)$$'

serve: build
	./itb serve

clean:
	rm -f itb
