# Stage 1: Build the application
ARG GO_VERSION=1.25.5
FROM golang:${GO_VERSION}-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy dependency files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# CGO_ENABLED=0 creates a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o syschecker .

# Stage 2: Create the runtime image
FROM alpine:latest

# Install runtime dependencies
# smartmontools is required for disk health checks
RUN apk add --no-cache smartmontools

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/syschecker .

# Set runtime environment variables
ENV TERM=xterm-256color

# Expose application ports
# (Note: This TUI application does not currently bind to a network port,
# but this instruction is included per requirements and for future extensibility)
EXPOSE 8080

# Specify the executable
ENTRYPOINT ["./syschecker"]
