FROM golang:1.25-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -o /out/bot ./cmd/bot

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates wget \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -r -u 10001 app && mkdir -p /app/config && chown -R app:app /app

WORKDIR /app
COPY --from=build /out/bot /app/bot
COPY config/local.yaml /app/config/local.yaml

ENV CONFIG_PATH=/app/config/local.yaml

USER app
EXPOSE 8080
ENTRYPOINT ["/app/bot"]
