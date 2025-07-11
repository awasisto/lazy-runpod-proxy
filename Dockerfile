FROM golang:1.19-bullseye

WORKDIR /app

COPY . .

RUN go mod tidy && go build -o lazy-runpod-proxy main.go

EXPOSE 8080

CMD ["./lazy-runpod-proxy"]