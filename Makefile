.PHONY: help build test test-go test-scala up down demo e2e results fmt

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

test-scala:
	cd analytics-scala && sbt test

up:
	docker compose up -d --build --wait

down:
	docker compose down

demo:
	docker compose --profile demo run --rm simulator --fixture F01-arena-normal

e2e:
	./tests/e2e/e2e.sh

results:
	curl -s localhost:8080/v1/games/$(GAME)/matches/$(MATCH)/results | jq

fmt:
	cd ingestion-go && gofmt -w .
	cd simulator && gofmt -w .
