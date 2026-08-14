# Iaas Platform — developer and ops entry points.
# Requires Go 1.26+. Optional: Docker for the container targets.

GO       ?= go
BINARY   := server
MIGRATE  := migrate
PORT     ?= 8080
IMAGE    ?= iaas-platform:dev

.PHONY: all fmt vet audit test race build migrate tidy spec docker-build docker-run clean

all: fmt vet audit test build

fmt:
	@echo "==> gofmt"
	@gofmt -l . ; test -z "$$(gofmt -l .)"

vet:
	@echo "==> go vet"
	$(GO) vet ./...

audit:
	@echo "==> govulncheck"
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

test:
	@echo "==> go test"
	$(GO) test ./... -count=1

race:
	@echo "==> go test -race"
	$(GO) test ./... -race -count=1

build:
	@echo "==> building binaries"
	$(GO) build -o bin/$(BINARY) ./cmd/server
	$(GO) build -o bin/$(MIGRATE) ./cmd/migrate

migrate:
	@echo "==> applying migrations"
	$(GO) run ./cmd/migrate

tidy:
	$(GO) mod tidy

spec:
	@echo "==> verifying embedded OpenAPI copy is in sync"
	@cmp openapi.yaml internal/dashboard/static/openapi.yaml

docker-build:
	@echo "==> building image $(IMAGE)"
	docker build -t $(IMAGE) .

docker-run:
	@echo "==> starting stack"
	docker compose up --build

clean:
	rm -rf bin
