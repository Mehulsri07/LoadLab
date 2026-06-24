# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o server .

# Runtime stage
FROM alpine:3.21

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 3000

CMD ["./server"]
