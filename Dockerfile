# Start from a base image
FROM golang:latest

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the application
RUN go build -o output main.go

# Command to run the executable
CMD ["./output"]

