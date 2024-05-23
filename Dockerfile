# Build stage
FROM golang:1.22 as builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Ensure go.mod and go.sum are up to date
RUN go mod tidy

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cluster-scan-monitor ./cmd/main.go

# Final stage
FROM scratch

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/cluster-scan-monitor /app/cluster-scan-monitor

# Command to run the binary
ENTRYPOINT ["/app/cluster-scan-monitor"]