FROM golang:1.26-alpine AS builder

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o shit-proxy .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/shit-proxy /shit-proxy

EXPOSE 8080
ENTRYPOINT ["/shit-proxy"]