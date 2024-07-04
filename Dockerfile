# Use the official Golang image as the base image
FROM golang:1.20-alpine

# Set the working directory to /app
WORKDIR /app

# Copy the Go source code into the container
COPY . .

# Download the required dependencies
RUN go mod download

# Build the Go application
RUN go build -o main .

# Expose the port that the application will run on
EXPOSE 7000

# Run the application
CMD ["./main"]
