FROM golang:1.19-bullseye AS build

ENV INACTIVITY_LIMIT_SECONDS=1200
ENV START_TIME_LIMIT_SECONDS=300
ENV RETRY_INTERVAL_SECONDS=5
ENV PREVENT_STALE_POD="yes"
ENV LISTEN_ADDRESS="0.0.0.0:8080"

WORKDIR /app

COPY . .

RUN go mod tidy && go build -o /usr/local/bin/lazy-runpod-proxy main.go

FROM debian:bullseye-slim

COPY --from=build /usr/local/bin/lazy-runpod-proxy /usr/local/bin/lazy-runpod-proxy

RUN apt update \
    && apt -y install ca-certificates \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

CMD ["lazy-runpod-proxy"]