FROM golang:alpine AS builder

# Install git for fetching dependencies
RUN apk update && apk add --no-cache git

WORKDIR /fogluted

COPY go.mod .
COPY go.sum .

RUN go mod download

# Add EdgeUsher
RUN mkdir /edgeusher \
    && wget https://raw.githubusercontent.com/di-unipi-socc/EdgeUsher/master/edgeusher.pl -P /edgeusher \
    && wget https://raw.githubusercontent.com/di-unipi-socc/EdgeUsher/master/hedgeusher.pl -P /edgeusher

COPY . .

# Build the binary.
RUN go build -o /go/bin/fogluted cmd/fogluted/main.go

## Build lighter image
FROM python:3.7-alpine

RUN apk update && apk upgrade && apk add bash

# Add Problog
RUN pip install problog

# Copy EdgeUsher folder
COPY --from=builder /edgeusher /edgeusher

# Copy our static executable.
COPY --from=builder /go/bin/fogluted /fogluted

EXPOSE 8080

# Run the binary.
ENTRYPOINT /fogluted