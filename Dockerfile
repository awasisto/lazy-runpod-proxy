FROM golang:1.19-bullseye AS build

WORKDIR /app

COPY . .

RUN go mod tidy && go build -o /usr/local/bin/lazy-runpod-proxy main.go

FROM debian:bullseye-slim

COPY --from=build /usr/local/bin/lazy-runpod-proxy /usr/local/bin/lazy-runpod-proxy

RUN apt update \
    && apt -y install ca-certificates \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

CMD ["lazy-runpod-proxy"]