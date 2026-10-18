.PHONY: help build test test-go test-scala up down demo gui gui-run gui-check e2e results fmt

GAME ?= arena-combat
MATCH ?= match-a1

help:
	@grep -E '^[a-z0-9-]+:' Makefile | cut -d: -f1

build:
	cd ingestion-go && go build ./...
	cd simulator && go build ./...
	cd analytics-scala && sbt assembly

test: test-go test-scala

test-go:
	cd ingestion-go && go test -race -count=1 ./...
	cd simulator && go test -race -count=1 ./...
	cd simulator && go test -tags gui -count=1 -run TestWindowBuildsWithoutIngestion ./cmd/gui

test-scala:
	cd analytics-scala && sbt test

up:
	docker compose up -d --build --wait

down:
	docker compose down

demo:
	docker compose --profile demo run --rm simulator --scenario demo --seed 42

# Aplicación de escritorio (ventana nativa, Fyne). Necesita Go y un compilador de C.
gui:
	cd simulator && go build -tags gui -trimpath -o ../bin/gamestats-gui ./cmd/gui
	@echo "Ejecutable: bin/gamestats-gui"

gui-run: gui
	./bin/gamestats-gui

# Revisa que la interfaz compile sin compilador de C (la compila para WebAssembly).
gui-check:
	cd simulator && GOOS=js GOARCH=wasm go vet -tags gui ./cmd/gui

e2e:
	./tests/e2e/e2e.sh

results:
	curl -s localhost:8080/v1/games/$(GAME)/matches/$(MATCH)/results | jq

fmt:
	cd ingestion-go && gofmt -w .
	cd simulator && gofmt -w .
