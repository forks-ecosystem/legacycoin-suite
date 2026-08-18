FROM golang:1.22-bookworm AS builder
ENV GOTOOLCHAIN=go1.26.5
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /legacycoin-suite .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /legacycoin-suite /usr/local/bin/legacycoin-suite
RUN mkdir -p /app/tmp
WORKDIR /app
EXPOSE 3002
ENTRYPOINT ["legacycoin-suite", "-web=:3002"]
