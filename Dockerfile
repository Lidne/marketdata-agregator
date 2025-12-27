FROM golang:1.25.1 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o marketdata-service ./cmd/server/main.go


FROM debian:bullseye-slim

COPY --from=builder /build/marketdata-service /marketdata-service

WORKDIR /

EXPOSE 50051

CMD ["./marketdata-service"]