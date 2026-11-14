.PHONY: build test vet fmt install clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PKG := br4zz4/thepurge/internal/version
LDFLAGS := -X $(PKG).Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o thepurge ./cmd/thepurge

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

install: build
	install -m 0755 thepurge $(HOME)/.local/bin/thepurge

clean:
	rm -f thepurge
