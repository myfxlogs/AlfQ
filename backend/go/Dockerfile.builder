# syntax=docker/dockerfile:1
# Shared builder for all 4 Go services (trading-core / md-gateway / quant-engine / assistant-svc).
# Usage: docker build --build-arg SVC=trading-core -f backend/go/Dockerfile.builder .

FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates curl bash
ENV GOPROXY=https://goproxy.cn,direct
RUN go install github.com/bufbuild/buf/cmd/buf@latest && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

WORKDIR /app
COPY gprc/ gprc/
COPY backend/proto/ backend/proto/
COPY backend/proto-mtapi/ backend/proto-mtapi/
RUN cd backend/proto && buf generate
RUN bash backend/proto-mtapi/build.sh

WORKDIR /app/backend/go
COPY backend/go/go.mod backend/go/go.sum ./
RUN go mod download
COPY backend/go/ ./
ARG SVC
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/svc ./cmd/${SVC}

FROM alpine:3.23
RUN apk add --no-cache ca-certificates docker-cli
WORKDIR /app
COPY --from=builder /bin/svc /usr/local/bin/svc
ENTRYPOINT ["/usr/local/bin/svc"]
