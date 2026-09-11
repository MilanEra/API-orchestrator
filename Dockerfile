# ==========================================
# Stage 1: Build the Go binary
# ==========================================
FROM golang:1.27-alpine AS builder

WORKDIR /build

# Copy dependency files first (optimizes Docker layer caching)
COPY go.mod ./

# Copy the rest of the source code
COPY . .

# Build a statically linked binary for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api-orchestrator ./cmd/server

# ==========================================
# Stage 2: Create minimal runtime image
# ==========================================
FROM alpine:3.20

# Install CA certificates: required for HTTPS calls to external APIs
RUN apk --no-cache add ca-certificates

# Create a non-root user for security (industry best practice)
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /build/api-orchestrator .

# Copy configuration files (the app reads them at runtime)
COPY --from=builder /build/configs ./configs

# Grant ownership to the non-root user
RUN chown -R appuser:appuser /app

# Switch to the non-root user
USER appuser

# Expose the application port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./api-orchestrator"]