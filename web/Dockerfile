FROM golang:1.24-alpine3.23 AS build

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /viewer ./cmd/web

FROM cgr.dev/chainguard/wolfi-base

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /viewer /app/viewer

ENV WEB_BIND_ADDR=0.0.0.0
ENV WEB_PORT=8080
ENV SQLITE_DB_PATH=/data/ritardi.sqlite

EXPOSE 8080

RUN addgroup -g 1001 -S trenitalia && \
    adduser -u 1001 -S trenitalia -G trenitalia

USER trenitalia

ENTRYPOINT ["/app/viewer"]
