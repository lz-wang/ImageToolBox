.PHONY: web build check test serve clean

VERSION := $(shell date +%Y-%m-%d)-$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

WEB_DIR := web
WEB_DIST := $(WEB_DIR)/dist

web:
	cd $(WEB_DIR) && npm run build
	@printf 'placeholder for go:embed; run `make web` to build the real WebUI\n' > $(WEB_DIST)/.placeholder

build: web
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
