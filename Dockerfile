# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod* go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go

# Final stage - using alpine for operational tooling
FROM alpine:3.19

# Install ca-certificates for HTTPS and wget for health checks
RUN apk --no-cache add ca-certificates wget

# Copy the binary from builder
COPY --from=builder /app/server /server

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/server"]