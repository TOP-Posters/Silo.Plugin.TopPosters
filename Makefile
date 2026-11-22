.PHONY: build build-all test clean

BINARY=plugin
PLATFORMS=linux/amd64 linux/arm64 darwin/arm64 freebsd/amd64 freebsd/arm64
VERSION ?= $(shell sed -n 's/.*"version": "\([^"]*\)".*/\1/p' manifest.json | head -1)
LDFLAGS=-s -w -X main.version=$(VERSION)

# Without a local Go toolchain, run in the golang image with a cached module volume.
GO_IMAGE ?= golang:1.26
GO = $(if $(shell command -v go 2>/dev/null),go,docker run --rm -v "$(CURDIR)":/src -v silo-gomod:/go/pkg/mod -w /src $(DOCKER_ENV) $(GO_IMAGE) go)

build:
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

test:
	$(GO) vet ./...
	$(GO) test ./...

build-all:
	@for platform in $(PLATFORMS); do \
		os=$${platform%%/*}; arch=$${platform##*/}; \
		$(MAKE) --no-print-directory build-one OS=$$os ARCH=$$arch || exit 1; \
	done

build-one: DOCKER_ENV=-e CGO_ENABLED=0 -e GOOS=$(OS) -e GOARCH=$(ARCH)
build-one:
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-$(OS)-$(ARCH) .

clean:
	rm -rf $(BINARY) dist
