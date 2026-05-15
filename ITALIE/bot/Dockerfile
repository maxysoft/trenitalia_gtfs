# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine3.23 AS build

# Installa dipendenze di sistema necessarie per la compilazione
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copia prima il file del modulo per sfruttare la cache di Docker
COPY go.mod ./
RUN go mod download

# Copia il codice sorgente e compila il binario
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s -X main.versione=1.0.0" \
    -o /bot ./cmd/bot

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.23

LABEL org.opencontainers.image.title="Bot Monitoraggio Trenitalia"
LABEL org.opencontainers.image.description="Bot Telegram per notifiche real-time sui ritardi Trenitalia"
LABEL org.opencontainers.image.source="https://github.com/maxysoft/trenitalia_gtfs"

WORKDIR /app

# Installa certificati CA e database dei fusi orari (necessario per Europe/Rome)
RUN apk add --no-cache ca-certificates tzdata

# Copia il binario dallo stage di build
COPY --from=build /bot /app/bot

# Directory persistenza SQLite
RUN mkdir -p /data

# Imposta il fuso orario italiano come predefinito
ENV TZ=Europe/Rome
ENV SQLITE_DB_PATH=/data/ritardi.sqlite

# Crea utente non privilegiato per sicurezza
RUN addgroup -g 1001 -S trenitalia && \
    adduser -u 1001 -S trenitalia -G trenitalia && \
    chown -R trenitalia:trenitalia /app /data

USER trenitalia

ENTRYPOINT ["/app/bot"]
