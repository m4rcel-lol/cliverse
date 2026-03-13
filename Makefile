BINARY    := cliverse
WORKER    := worker
HASHPW    := hash-password
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X main.version=$(VERSION)

.PHONY: build build-worker build-hash-password test vet fmt lint clean docker run help

## build: compile the main server binary
build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/cliverse

## build-worker: compile the federation worker binary
build-worker:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(WORKER) ./cmd/worker

## build-hash-password: compile the admin password hashing helper
build-hash-password:
	CGO_ENABLED=0 go build -o $(HASHPW) ./cmd/hash-password

## all: build both binaries
all: build build-worker

## test: run all unit tests
test:
	go test -count=1 -race ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go source files
fmt:
	gofmt -s -w .

## lint: run vet + test
lint: vet test

## clean: remove build artifacts
clean:
	rm -f $(BINARY) $(WORKER) $(HASHPW)

## docker: build Docker images with Compose
docker:
	docker compose build

## docker-up: start all services
docker-up:
	docker compose up -d

## docker-down: stop all services
docker-down:
	docker compose down

## run: build and run the server locally (needs DB + Redis)
run: build
	./$(BINARY)

## help: show available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
