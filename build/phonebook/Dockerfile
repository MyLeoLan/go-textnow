# 1) Build
FROM golang:alpine as builder

# set the current working directory inside the container 
WORKDIR /app

# Copy the dependencies files
COPY go.mod go.sum ./

# Download the dependencies
RUN go mod download

# Then, finally, copy all the app files
COPY . .

# Instruct the go compiler to build a binary that depends on Go runtime libraries vs system libraries
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/phonebook/main.go 

# 2) Run the binary
# alpine is a smaller image, we just need to run the binary
FROM alpine:latest

WORKDIR /app

# Copy the output (only binary file) of the previous image
COPY --from=builder /app/main /app/

# Run it!.
CMD ["./main"]