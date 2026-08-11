#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "==> Generating protobuf / gRPC stubs with Buf"
export PATH="/tmp:$PATH"
buf generate

echo "==> Tidying Go modules"
go mod tidy

echo ""
echo "============================================================"
echo "  PoC is ready. Start the components in separate terminals:"
echo "============================================================"
echo ""
echo "  1. NATS Server (with JetStream):"
echo "       nats-server -js -m 8222"
echo ""
echo "  2. NATS Publisher gRPC service:"
echo "       go run ./cmd/nats-server"
echo ""
echo "  3. Ingest gRPC service (calls the publisher):"
echo "       go run ./cmd/ingest-server"
echo ""
echo "  4. Consumer (prints published readings):"
echo "       go run ./cmd/consumer"
echo ""
echo "  5. Client (sends sample meter readings):"
echo "       go run ./cmd/client"
echo ""
echo "============================================================"
