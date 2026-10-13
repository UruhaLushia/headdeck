SHELL := /bin/bash

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/UruhaLushia/headdeck/internal/version.Version=$(VERSION) \
	-X github.com/UruhaLushia/headdeck/internal/version.Commit=$(COMMIT) \
	-X github.com/UruhaLushia/headdeck/internal/version.BuildTime=$(DATE)

BIN     := hdkt
DIST    := dist
WEB     := web
PLATFORMS := linux/amd64 linux/arm64

.PHONY: all
all: build

# The browser UI is compiled into internal/web/dist and embedded, so `web` must
# run before `build` whenever the frontend changed.
.PHONY: web
web:
	pnpm --dir $(WEB) install
	pnpm --dir $(WEB) build

.PHONY: build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/hdkt

.PHONY: build-web
build-web: web build

.PHONY: test
test:
	go test ./...

.PHONY: race
race:
	go test -race ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -l -w ./cmd ./internal
	pnpm --dir $(WEB) format

.PHONY: web-check
web-check:
	pnpm --dir $(WEB) format:check
	pnpm --dir $(WEB) lint:check
	pnpm --dir $(WEB) typecheck

.PHONY: check
check: vet lint test web-check

.PHONY: install
install: build
	install -m 0755 $(BIN) /usr/local/bin/$(BIN)

.PHONY: release
release: web
	rm -rf $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		out=$(DIST)/$(BIN)_$${os}_$${arch}; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
			-o $$out ./cmd/hdkt || exit 1; \
	done
	@cd $(DIST) && sha256sum $(BIN)_* > SHA256SUMS && cat SHA256SUMS

.PHONY: clean
clean:
	rm -rf $(DIST) $(BIN)
	rm -rf $(WEB)/node_modules
	find internal/web/dist -mindepth 1 ! -name .gitkeep -delete
