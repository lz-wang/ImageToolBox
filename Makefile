.PHONY: build clean

VERSION := $(shell date +%Y-%m-%d)-$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o itb .

clean:
	rm -f itb