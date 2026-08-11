# syntax=docker/dockerfile:1
#
# Shared Dockerfile for every service in cmd/. Select which one to build with:
#   docker build --build-arg SERVICE=nats-server .
#
# Generates protobuf/gRPC stubs inside the image so `docker compose build` works
# from a clean checkout even though generated *.pb.go files are gitignored.

# ---- proto: run buf generate ----
FROM golang:1.26-alpine AS proto
RUN apk add --no-cache curl && \
    curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-Linux-x86_64" \
      -o /usr/local/bin/buf && chmod +x /usr/local/bin/buf
WORKDIR /src
COPY go.mod go.sum ./
# NOTE: adjust these to match the plugin versions pinned in your buf.gen.yaml
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
ENV PATH="/root/go/bin:${PATH}"
COPY buf.gen.yaml buf.yaml* ./
COPY proto ./proto
RUN buf generate

# ---- build: compile the selected service ----
FROM golang:1.26-alpine AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY --from=proto /src .
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 go build -o /out/service ./cmd/${SERVICE}

# ---- runtime: minimal final image ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/service /usr/local/bin/service
ENTRYPOINT ["/usr/local/bin/service"]