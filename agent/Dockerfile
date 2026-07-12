# Tessera console-api — the verification HTTP API (real consensus + signing + bond).
# Multi-stage: build the static Go binary, run it on a minimal base.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/console-api ./cmd/console-api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 tessera
USER tessera
WORKDIR /app
COPY --from=build /out/console-api /app/console-api
# Cloud host injects $PORT; config binds :$PORT when set (defaults :8787 locally).
EXPOSE 8787
ENTRYPOINT ["/app/console-api"]
