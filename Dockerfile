FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rate-limiter ./cmd/ratelimiter

FROM alpine:3.21

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/rate-limiter .
COPY --from=builder /app/.env.example .env

EXPOSE 8080

CMD ["./rate-limiter"]
