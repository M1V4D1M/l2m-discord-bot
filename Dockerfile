# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o bot .

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bot .
# Copy .env if needed (though Cloud Run usually uses env vars)
# COPY .env .

# Run the application
CMD ["./bot"]
