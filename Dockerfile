# Stage 1: Build the Go application
FROM golang:1.21-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# We need to initialize a Go module and install dependencies.
# This assumes your project is at the root of the Docker context.
# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy the source code into the container
COPY . .

# Build the Go application
# CGO_ENABLED=0 for a static binary, useful for Alpine images
# -ldflags="-w -s" to strip debug information and reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server/main.go

# Stage 2: Create the runtime image from a minimal base
FROM alpine:latest

# Add ca-certificates for HTTPS calls if needed by the application
RUN apk --no-cache add ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/server /app/server

# Expose the port the application runs on (should match config.ServerAddress)
EXPOSE 8080

# Define environment variables that the application might need
# These can be overridden at runtime (e.g., with docker run -e)
# Default values are provided here for illustration, but it's better to
# set them via docker-compose.yml or Kubernetes manifests for production.
ENV SERVER_PORT="8080"
ENV DB_HOST="db" # Example, assuming a 'db' service in docker-compose
ENV DB_PORT="5432"
ENV DB_USER="testuser"
ENV DB_PASSWORD="testpassword"
ENV DB_NAME="entertainmentecomm"
ENV JWT_SECRET="a_very_secret_key_for_docker_that_should_be_changed"
ENV JWT_EXPIRY_HOURS="24"
ENV VIDEO_PROCESSING_QUEUE="video_processing_jobs"
ENV LOG_LEVEL="info"
# Add other ENV vars as needed for S3, Redis, etc. when they are integrated

# Command to run the executable
ENTRYPOINT ["/app/server"]
