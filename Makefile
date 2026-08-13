# Makefile for the meter-readings PoC
# Assumes this file lives at the repo root (same level as proto/, cmd/, go.mod).

SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

export PATH := /tmp:$(PATH)

.PHONY: all setup generate tidy clean up down build logs ps client run-nats run-ingest run-consumer run-client info help

all: setup

## setup: generate stubs, tidy modules, then print how to run everything
setup: generate tidy info

## up: build (if needed) and start nats, nats-server, ingest-server, consumer
up:
	docker compose up -d --build

## down: stop and remove all containers and the nats-data volume
down:
	docker compose down -v

## build: (re)build all service images
build:
	docker compose build

## logs: tail logs from every running service
logs:
	docker compose logs -f

## ps: show status of the compose stack
ps:
	docker compose ps

## client: run the one-shot sample-reading client against the running stack
client:
	docker compose run --rm client

## generate: run buf generate to produce protobuf / gRPC stubs
generate:
	@echo "==> Generating protobuf / gRPC stubs with Buf"
	buf generate

## tidy: tidy go.mod / go.sum
tidy:
	@echo "==> Tidying Go modules"
	go mod tidy

## clean: remove generated protobuf / gRPC stubs
clean:
	@echo "==> Removing generated protobuf / gRPC stubs"
	@find . -type f \( -name '*.pb.go' -o -name '*_grpc.pb.go' -o -name '*.pb.gw.go' \) \
		-not -path './vendor/*' -print -delete
	@find . -type d -name 'gen' -not -path './vendor/*' -prune -exec rm -rf {} \; -print

## run-nats: start the NATS publisher gRPC service
run-nats:
	go run ./cmd/nats-server

## run-ingest: start the ingest gRPC service (calls the publisher)
run-ingest:
	go run ./cmd/ingest-server

## run-consumer: start the JetStream durable consumer
run-consumer:
	go run ./cmd/consumer

run-clickhouse-consumer:
	go run ./cmd/clickhouse-consumer

## run-client: send sample solar/wind/BESS readings
run-client:
	go run ./cmd/client

## info: print instructions for starting each component
info:
	@echo ""
	@echo "============================================================"
	@echo "  PoC is ready."
	@echo "============================================================"
	@echo ""
	@echo "  Whole stack, one command:"
	@echo "       make up            # nats, nats-server, ingest-server, consumer"
	@echo "       make client        # send sample solar/wind/BESS readings"
	@echo "       make logs          # tail all service logs"
	@echo "       make down          # stop and remove everything"
	@echo ""
	@echo "  Or run components locally against Go (useful while iterating):"
	@echo "       make run-nats      # go run ./cmd/nats-server"
	@echo "       make run-ingest    # go run ./cmd/ingest-server"
	@echo "       make run-consumer  # go run ./cmd/consumer/dummy"
	@echo "       make run-client    # go run ./cmd/client"
	@echo ""
	@echo "============================================================"

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed -e 's/## /  /'