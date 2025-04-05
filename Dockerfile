# Build stage
FROM golang:1.23.4-alpine AS builder

WORKDIR /app
COPY . .

# Install dependencies
RUN apk add --no-cache git
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o music-collection .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates
RUN apk add --no-cache ca-certificates

# Copy the binary and assets from builder
COPY --from=builder /app/music-collection .
COPY --from=builder /app/web ./web

# Set environment variables
ENV DOCKER_ENV=true

# Run the application
CMD ["./music-collection"]

