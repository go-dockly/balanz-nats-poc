# syntax=docker/dockerfile:1
#
# Shared Dockerfile for every service in cmd/. 
#
# Generates protobuf/gRPC stubs inside the image so `docker compose build` works
# from a clean checkout even though generated *.pb.go files are gitignored.

# buf gen
FROM golang:1.26-alpine AS proto
RUN apk add --no-cache curl && \
    curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-Linux-x86_64" \
      -o /usr/local/bin/buf && chmod +x /usr/local/bin/buf
WORKDIR /src
COPY go.mod go.sum ./
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
ENV PATH="/root/go/bin:${PATH}"
COPY buf.gen.yaml buf.yaml* ./
COPY proto ./proto
RUN buf generate

# build
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG SERVICE
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY --from=proto /src .
COPY . .

# use target platform variables that BuildKit injects
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /out/service ./cmd/${SERVICE}

# runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/service /usr/local/bin/service
RUN chmod +x /usr/local/bin/service
ENTRYPOINT ["/usr/local/bin/service"]