# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download || true

COPY . .

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o tsusage-hub main.go

# Runtime stage (Alpine with tzdata, ~15MB total)
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/tsusage-hub /app/tsusage-hub

VOLUME ["/data"]

EXPOSE 3888

ENV PORT=3888 \
    DB_PATH=/data/usage.db \
    TZ=Asia/Shanghai

CMD ["/app/tsusage-hub"]
