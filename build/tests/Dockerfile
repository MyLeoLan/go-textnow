FROM golang:alpine as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -count=1 to force running tests again and ignore cache
CMD CGO_ENABLED=0 go test -count=1 -v  ./tests/.