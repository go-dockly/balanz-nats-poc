# Makefile for the meter-readings PoC
# Assumes this file lives at the repo root (same level as proto/, cmd/, go.mod).

SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

export PATH := /tmp:$(PATH)

.PHONY: all setup generate tidy clean run-nats run-ingest run-consumer run-client info help

all: setup

## setup: generate stubs, tidy modules, then print how to run everything
setup: generate tidy info

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

## run-client: send sample solar/wind/BESS readings
run-client:
	go run ./cmd/client

## info: print instructions for starting each component
info:
	@echo ""
	@echo "============================================================"
	@echo "  PoC is ready. Start the components in separate terminals:"
	@echo "============================================================"
	@echo ""
	@echo "  1. NATS Server (with JetStream):"
	@echo "       nats-server -js -m 8222"
	@echo ""
	@echo "  2. NATS Publisher gRPC service:"
	@echo "       make run-nats      # or: go run ./cmd/nats-server"
	@echo ""
	@echo "  3. Ingest gRPC service (calls the publisher):"
	@echo "       make run-ingest    # or: go run ./cmd/ingest-server"
	@echo ""
	@echo "  4. Consumer (prints published readings):"
	@echo "       make run-consumer  # or: go run ./cmd/consumer"
	@echo ""
	@echo "  5. Client (sends sample meter readings):"
	@echo "       make run-client    # or: go run ./cmd/client"
	@echo ""
	@echo "============================================================"

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed -e 's/## /  /'