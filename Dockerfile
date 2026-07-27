FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev linux-headers

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/legacycoin-miner .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/legacycoin-miner /usr/local/bin/legacycoin-miner
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
EXPOSE 3002
ENTRYPOINT ["docker-entrypoint.sh"]
