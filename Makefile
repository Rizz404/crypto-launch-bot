COMPOSE ?= podman compose -f deploy/compose.yml
CONTAINER_ENGINE ?= podman
PROMETHEUS_IMAGE ?= docker.io/prom/prometheus:v3.15.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: generate css build test test-integration test-monitoring lint fmt up down logs migrate run image clean

# Windows needs GNU make plus a POSIX shell (Git Bash/MSYS2) for these recipes.
EXE := $(shell go env GOEXE)
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Checksums from the release's sha256sums.txt (glibc builds on Linux).
TAILWIND_VERSION := 4.3.3
TAILWIND_ASSET_linux_amd64 := tailwindcss-linux-x64
TAILWIND_SHA256_linux_amd64 := dc61b3ac6b8c9ca874c0cc4c57b2409791a64c5540404ca5f5367360babc313a
TAILWIND_ASSET_linux_arm64 := tailwindcss-linux-arm64
TAILWIND_SHA256_linux_arm64 := 55fd0b241214eff3de1e8ee4f22796662f2d2e7a49bcfca7477cfd0bac398195
TAILWIND_ASSET_darwin_amd64 := tailwindcss-macos-x64
TAILWIND_SHA256_darwin_amd64 := 7922e0953f2110c05976e3bf58f14e643d90427575e766b7d433f5f80cbee7e1
TAILWIND_ASSET_darwin_arm64 := tailwindcss-macos-arm64
TAILWIND_SHA256_darwin_arm64 := cdf646702987a743464dff4d9c60fd4480d1c1e73dd819a9a67f1078815dce9d
TAILWIND_ASSET_windows_amd64 := tailwindcss-windows-x64.exe
TAILWIND_SHA256_windows_amd64 := e0e260ce048014e9268f6237ff18f8ccf02cef521cbd0ae04e82c2cdf7aa3955
TAILWIND_ASSET := $(TAILWIND_ASSET_$(GOOS)_$(GOARCH))
TAILWIND_SHA256 := $(TAILWIND_SHA256_$(GOOS)_$(GOARCH))
TAILWIND := bin/tools/tailwindcss$(EXE)

$(TAILWIND):
	@test -n "$(TAILWIND_ASSET)" || { echo "no pinned Tailwind binary for $(GOOS)/$(GOARCH)"; exit 1; }
	mkdir -p $(dir $@)
	curl -fsSL -o $@ https://github.com/tailwindlabs/tailwindcss/releases/download/v$(TAILWIND_VERSION)/$(TAILWIND_ASSET)
	echo "$(TAILWIND_SHA256)  $@" | sha256sum -c -
	chmod +x $@

# Tailwind -> internal/ui/static/app.css (committed). Needs the templ output first.
css: $(TAILWIND)
	$(TAILWIND) -i internal/ui/input.css -o internal/ui/static/app.css --minify

# templ templates -> *_templ.go and the Tailwind bundle (both committed)
generate:
	go tool templ generate -path internal
	$(MAKE) css

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/bot$(EXE) ./cmd/bot
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/datafetch$(EXE) ./cmd/datafetch
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/backtest$(EXE) ./cmd/backtest
	go build -trimpath -o bin/portfolio-backtest$(EXE) ./cmd/portfolio-backtest
	go build -trimpath -o bin/catalyst$(EXE) ./cmd/catalyst
	go build -trimpath -o bin/launch$(EXE) ./cmd/launch

test:
	go test ./...

test-monitoring:
	$(CONTAINER_ENGINE) run --rm --network none -v "$(CURDIR)/deploy/prometheus:/checks:ro" -w /checks \
		--entrypoint /bin/promtool $(PROMETHEUS_IMAGE) test rules alerts.test.yml

# Needs the compose database running (make up).
test-integration:
	TB_TEST_DATABASE_URL=$${TB_TEST_DATABASE_URL:-postgres://bot:bot@localhost:5432/cryptolaunchbot?sslmode=disable} \
		go test -count=1 -tags integration ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

migrate: build
	./bin/bot$(EXE) migrate up

run: build
	./bin/bot$(EXE) run

image:
	podman build --build-arg VERSION=$(VERSION) -t crypto-launch-bot:$(VERSION) .

clean:
	rm -rf bin
