FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o content-service ./cmd/main.go

FROM alpine:3.18

RUN apk add --no-cache ca-certificates


COPY --from=builder /src/content-service /usr/local/bin/content-service
RUN chmod +x /usr/local/bin/content-service

COPY config/config.yaml /etc/content-service/config.yaml


WORKDIR /


EXPOSE 3000 50051

ENTRYPOINT ["/usr/local/bin/content-service", "--config", "/etc/content-service/config.yaml"]