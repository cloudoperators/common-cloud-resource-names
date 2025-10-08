FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the webhook service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o webhook-server ./cmd/webhook

# Use a distroless base image for a smaller footprint
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy the binary from the builder stage
COPY --from=builder /app/webhook-server /webhook-server

# Use an unprivileged user
USER 65532:65532

# Expose the webhook port
EXPOSE 8443

# Set the entrypoint
ENTRYPOINT ["/webhook-server"]