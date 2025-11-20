# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod* go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build argument for version injection
# Default "development" is used for local/manual builds without --build-arg VERSION=...
# Automated deployments (cmd/deploy/main.go) inject the actual version from git tags
# Example: docker build --build-arg VERSION=v1.2.3 -t arguseek .
ARG VERSION=development

# Build the binary with version injection
# IMPORTANT: ldflags path must match Makefile release target and internal/version package
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X arguseek/internal/version.injectedVersion=${VERSION}" \
    -a -installsuffix cgo -o server cmd/server/main.go

# Final stage - using alpine for operational tooling
FROM alpine:3.19

# Install ca-certificates for HTTPS and wget for health checks
RUN apk --no-cache add ca-certificates wget

# Copy the binary from builder
COPY --from=builder /app/server /server

# Expose port
EXPOSE 8080

# Run the binary in HTTP mode (required for Docker networking)
ENTRYPOINT ["/server"]
CMD ["-http"]