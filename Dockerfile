FROM golang:alpine AS builder

# Install git for fetching dependencies
RUN apk update && apk add --no-cache git

WORKDIR /fogluted

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

# Build the binary.
RUN go build -o /go/bin/fogluted cmd/fogluted/main.go

## Build lighter image
FROM alpine:latest

# Copy our static executable.
COPY --from=builder /go/bin/fogluted /fogluted

EXPOSE 8080

# Run the binary.
ENTRYPOINT /fogluted