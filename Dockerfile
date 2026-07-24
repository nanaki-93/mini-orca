# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o mini-orca ./cmd/daemon

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary
COPY --from=builder /app/mini-orca .

# Create directories
RUN mkdir -p config .mini-orca

# Copy default config
COPY config/config.yaml config/

# Expose port
EXPOSE 8080

# Run
CMD ["./mini-orca"]
