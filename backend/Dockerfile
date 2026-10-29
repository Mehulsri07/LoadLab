# Build stage
FROM golang:1.24-alpine AS builder
ARG VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.buildVersion=${VERSION}" -o /server .

# Runtime stage
FROM gcr.io/distroless/static:nonroot
# Defaults are a healthy image; the :bad image sets FAULT_LATENCY_MS=300 FAULT_ERROR_RATE=0.1
ARG FAULT_LATENCY_MS=0
ARG FAULT_ERROR_RATE=0
ENV FAULT_LATENCY_MS=${FAULT_LATENCY_MS} FAULT_ERROR_RATE=${FAULT_ERROR_RATE}

COPY --from=builder /server /server
USER nonroot:nonroot
# 3000 public; 9091 admin, internal only (never publish it)
EXPOSE 3000 9091

ENTRYPOINT ["/server"]
