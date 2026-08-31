.PHONY: web build bins verify-bins build-full check test serve clean

VERSION := $(shell date +%Y-%m-%d)-$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

WEB_DIR := web
WEB_DIST := $(WEB_DIR)/dist
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

web:
	cd $(WEB_DIR) && npm run build
	@printf 'placeholder for go:embed; run `make web` to build the real WebUI\n' > $(WEB_DIST)/.placeholder

build: web
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

build-full: bins verify-bins web
	go build $(LDFLAGS) -o itb .

check:
	go vet ./...
	cd $(WEB_DIR) && npm run type-check
	cd $(WEB_DIR) && npm run lint

test:
	go test ./...
	cd $(WEB_DIR) && npm run test

serve: build
	./itb serve

clean:
	rm -f itb
	@if [ -d $(WEB_DIST) ]; then find $(WEB_DIST) -mindepth 1 -not -name '.placeholder' -delete; fi
