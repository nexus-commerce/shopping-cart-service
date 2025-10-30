FROM golang:1.24.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod tidy && \
    go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/main ./cmd/app
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8083
CMD ["./main"]