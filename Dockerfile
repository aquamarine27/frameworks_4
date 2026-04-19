FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o frameworks_4 .

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/frameworks_4 .

EXPOSE 3000

CMD ["./frameworks_4"]