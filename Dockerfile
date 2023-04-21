FROM golang:1.20-alpine AS builder

RUN addgroup -S ghcuser && adduser -S -u 10000 -g ghcuser ghcuser

WORKDIR /app

COPY ["go.mod", "go.sum", "."]

RUN ["go", "mod", "download"]

COPY [".", "."]

RUN go build -ldflags='-w -s -extldflags "-static"' -a ./cmd/ghcomment

FROM scratch

COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /app/ghcomment /ghcomment

USER ghcuser

ENTRYPOINT ["/ghcomment"]
