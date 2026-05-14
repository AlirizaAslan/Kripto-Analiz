FROM golang:1.23-alpine AS builder
WORKDIR /app

# Enable CGO for SQLite if needed (though modernc.org/sqlite is pure Go, it's safer to have standard alpine deps if building)
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
