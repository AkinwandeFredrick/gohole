FROM golang:1.22-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-w -s" \
    -o gohole ./cmd/gohole

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata sqlite-libs

WORKDIR /app

COPY --from=builder /build/gohole .
COPY config.yaml .

# DNS
EXPOSE 53/udp
EXPOSE 53/tcp
# Dashboard
EXPOSE 8080

VOLUME ["/app/data"]

ENV GOHOLE_DB_PATH=/app/data/gohole.db

ENTRYPOINT ["./gohole", "-config", "config.yaml"]
