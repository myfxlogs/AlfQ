# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates curl bash
ARG BUF_VERSION=v1.50.0
RUN curl -sSL "https://github.com/bufbuild/buf/releases/download/${BUF_VERSION}/buf-$(uname -s)-$(uname -m)" -o /usr/local/bin/buf && chmod +x /usr/local/bin/buf
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
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
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/svc ./cmd/admin-api

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/svc /usr/local/bin/svc
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/svc"]
