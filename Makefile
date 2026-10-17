COMPOSE ?= podman compose -f deploy/compose.yml
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: generate build test test-integration lint fmt up down logs migrate run image clean

# templ templates -> *_templ.go (committed)
generate:
	go tool templ generate -path internal/web/views

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/bot ./cmd/bot
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/datafetch ./cmd/datafetch

test:
	go test ./...

# Needs the compose database running (make up).
test-integration:
	TB_TEST_DATABASE_URL=$${TB_TEST_DATABASE_URL:-postgres://bot:bot@localhost:5432/tradingbot?sslmode=disable} \
		go test -tags integration ./...

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
