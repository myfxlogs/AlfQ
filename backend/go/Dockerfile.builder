# syntax=docker/dockerfile:1
# Shared builder for all 4 Go services (trading-core / md-gateway / quant-engine / assistant-svc).
# Usage: docker build --build-arg SVC=trading-core -f backend/go/Dockerfile.builder .
#
# Multi-stage layer caching:
#   tools  — install buf, protoc plugins         (invalidate: tool versions)
#   proto  — generate proto/mtapi stubs          (invalidate: proto files)
#   deps   — download Go modules                 (invalidate: go.mod/go.sum)
#   build  — compile binary                      (invalidate: any .go source)

# ── Stage 1: tools ──
FROM golang:1.26-alpine AS tools
RUN apk add --no-cache git ca-certificates curl bash gcc musl-dev
ENV GOPROXY=https://goproxy.cn,direct
RUN go install github.com/bufbuild/buf/cmd/buf@latest && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# ── Stage 2: proto generation ──
FROM tools AS proto
WORKDIR /app
COPY gprc/ gprc/
COPY backend/proto/ backend/proto/
COPY backend/proto-mtapi/ backend/proto-mtapi/
RUN cd backend/proto && buf generate
RUN bash backend/proto-mtapi/build.sh

# ── Stage 3: Go module download ──
FROM proto AS deps
WORKDIR /app/backend/go
COPY backend/go/go.mod backend/go/go.sum ./
RUN go mod download

# ── Stage 4: compile ──
FROM deps AS build
COPY backend/go/ ./
ARG SVC
ARG CGO_ENABLED=0
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=linux go build -ldflags="-s -w" -o /bin/svc ./cmd/${SVC}

# ── Final runtime image ──
FROM alpine:3.23
RUN apk add --no-cache ca-certificates docker-cli
WORKDIR /app
COPY --from=build /bin/svc /usr/local/bin/svc
ENTRYPOINT ["/usr/local/bin/svc"]
