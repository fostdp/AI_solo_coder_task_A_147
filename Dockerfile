FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

RUN go build -ldflags="-s -w -X main.Version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o /app/noria-bearing-server .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY --from=builder /app/noria-bearing-server /app/noria-bearing-server
COPY backend/config/ /app/config/
COPY backend/config.yaml /app/config.yaml

RUN addgroup -g 10001 appgroup && \
    adduser -u 10000 -G appgroup -s /sbin/nologin -D appuser && \
    chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080 5020 6060 9090

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

ENTRYPOINT ["/app/noria-bearing-server"]
CMD ["config.yaml"]
