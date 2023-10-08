# Start from a base image
FROM golang:latest

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the application
RUN go build -o output main.go retry.go redis_client.go process.go notify.go initialise_redis.go ingest.go destinations.go

# Command to run the executable
CMD ["./output"]

