FROM golang:1.21-alpine

WORKDIR /app

# Copy go mod and sum files
COPY go.mod ./

# Copy source code
COPY . .

# Build the application
RUN go build -o test-server

# Expose port 8080
EXPOSE 8080

# Run the application
CMD ["./test-server"] 
