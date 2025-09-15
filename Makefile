# Name des finalen Binaries
BINARY=metrics-agent
# Pfad zum main-Package
PKG=github.com/janhuddel/metrics-agent/cmd/metrics-agent
# Build-Verzeichnis
BUILDDIR=.build

# Git-Infos für Version einbetten
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build test clean release

all: build

## Lokales Binary bauen
build:
	@mkdir -p $(BUILDDIR)
	go build $(LDFLAGS) -o $(BUILDDIR)/$(BINARY) $(PKG)
	@echo "Built $(BUILDDIR)/$(BINARY)"

## Tests laufen lassen
test:
	go test ./... -v

## Build-Verzeichnis leeren
clean:
	rm -rf $(BUILDDIR)

## Cross-Compile Release-Binaries
release: clean
	@mkdir -p $(BUILDDIR)
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(BUILDDIR)/$(BINARY)-linux-amd64 $(PKG)
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(BUILDDIR)/$(BINARY)-darwin-arm64 $(PKG)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILDDIR)/$(BINARY)-windows-amd64.exe $(PKG)
	@echo "Release artifacts in $(BUILDDIR)"
