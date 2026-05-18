FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY backend/go/go.mod backend/go/go.sum ./
RUN go mod download
COPY backend/go/ ./
RUN CGO_ENABLED=0 go build -o /admin-api ./cmd/admin-api

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /admin-api /usr/local/bin/admin-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/admin-api"]
