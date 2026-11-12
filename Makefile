COMPOSE ?= podman compose -f deploy/compose.yml
CONTAINER_ENGINE ?= podman
PROMETHEUS_IMAGE ?= docker.io/prom/prometheus:v3.15.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: generate css build test test-integration test-monitoring lint fmt up down logs migrate run image clean

# shortcut: linux-x64 only; add the other platforms' asset names + sha256 when someone needs them.
TAILWIND_VERSION := 4.3.3
TAILWIND_SHA256 := dc61b3ac6b8c9ca874c0cc4c57b2409791a64c5540404ca5f5367360babc313a
TAILWIND := bin/tools/tailwindcss

$(TAILWIND):
	mkdir -p $(dir $@)
	curl -fsSL -o $@ https://github.com/tailwindlabs/tailwindcss/releases/download/v$(TAILWIND_VERSION)/tailwindcss-linux-x64
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
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/bot ./cmd/bot
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/datafetch ./cmd/datafetch
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/backtest ./cmd/backtest
	go build -trimpath -o bin/portfolio-backtest ./cmd/portfolio-backtest
	go build -trimpath -o bin/catalyst ./cmd/catalyst
	go build -trimpath -o bin/launch ./cmd/launch

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
	./bin/bot migrate up

run: build
	./bin/bot run

image:
	podman build --build-arg VERSION=$(VERSION) -t crypto-launch-bot:$(VERSION) .

clean:
	rm -rf bin
