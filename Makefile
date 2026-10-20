COMPOSE ?= podman compose -f deploy/compose.yml
CONTAINER_ENGINE ?= podman
PROMETHEUS_IMAGE ?= docker.io/prom/prometheus:v3.15.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: generate build test test-integration test-monitoring lint fmt up down logs migrate run image clean

# templ templates -> *_templ.go (committed)
generate:
	go tool templ generate -path internal/web/views

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/bot ./cmd/bot
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/datafetch ./cmd/datafetch
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/backtest ./cmd/backtest
	go build -trimpath -o bin/portfolio-backtest ./cmd/portfolio-backtest

test:
	go test ./...

test-monitoring:
	$(CONTAINER_ENGINE) run --rm --network none -v "$(CURDIR)/deploy/prometheus:/checks:ro" -w /checks \
		--entrypoint /bin/promtool $(PROMETHEUS_IMAGE) test rules alerts.test.yml

# Needs the compose database running (make up).
test-integration:
	TB_TEST_DATABASE_URL=$${TB_TEST_DATABASE_URL:-postgres://bot:bot@localhost:5432/tradingbot?sslmode=disable} \
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
	podman build --build-arg VERSION=$(VERSION) -t trading-bot:$(VERSION) .

clean:
	rm -rf bin
