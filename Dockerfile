# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install required packages
RUN apk add --no-cache gcc musl-dev sqlite

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o birthday-bot main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache sqlite

# Copy the binary from builder
COPY --from=builder /app/birthday-bot .
COPY --from=builder /app/init_birthdays.sql .

# Create data directory
RUN mkdir -p /app/data && chmod 777 /app/data

# Set environment variables
ENV DATABASE_PATH=/app/data/birthdays.db
ARG TELEGRAM_BOT_TOKEN
ENV TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}

# Expose port
EXPOSE 80

# Run the bot
CMD ["./birthday-bot"]