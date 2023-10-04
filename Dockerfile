# Start from a Debian based image with the latest version of Go installed
# and a workspace (GOPATH) configured at /go.
# A sample dockerfile

FROM golang:latest

# Copy the local package files to the container's workspace.
ADD . /go/src/myapp

# Set the current working directory inside the container.
WORKDIR /go/src/myapp

# Build the Go app
RUN go build .

# This container exposes port 8080 to the outside world
EXPOSE 8080

# Run the binary program produced by `go install`
CMD ["./myapp"]
