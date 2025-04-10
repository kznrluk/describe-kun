# Stage 1: Build the Go binary
# Updated Go version to 1.23
FROM golang:1.23-bookworm AS builder

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Slack application
# CGO_ENABLED=0 produces a static binary (usually preferred for containers)
# -ldflags="-s -w" strips debug information to reduce binary size
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /describe-kun-slack ./cmd/describe-kun-slack

# Stage 2: Create the final runtime image
# Using debian:bookworm-slim as it provides a standard chromium package
FROM debian:bookworm-slim

# Install Chromium and necessary dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends chromium ca-certificates fonts-liberation && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create a non-root user and group
RUN groupadd -r appuser && useradd -r -g appuser -s /sbin/nologin -c "Docker image user" appuser

# Create app directory owned by the new user
WORKDIR /app
RUN chown appuser:appuser /app

# Set XDG directories for Chromium compatibility
ENV XDG_CONFIG_HOME=/tmp/.config
ENV XDG_CACHE_HOME=/tmp/.cache

# Switch to the non-root user
USER appuser

# Copy the built Slack binary from the builder stage
COPY --from=builder /describe-kun-slack /app/describe-kun-slack

# Set the entrypoint for the Slack app
ENTRYPOINT ["/app/describe-kun-slack"]
