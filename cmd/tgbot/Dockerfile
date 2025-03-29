# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /tgbot ./cmd/tgbot

# Final stage
FROM alpine:latest

# Add ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /tgbot .

# Set environment variables
ENV TELEGRAM_BOT_TOKEN=""
ENV OPENAI_API_KEY=""
ENV EMBEDDING_SERVICE_ADDRESS="http://embedding-service:8000/embeddings"
ENV QDRANT_SERVICE_ADDRESS="http://qdrant:6333"

# Command to run
CMD ["./tgbot"]