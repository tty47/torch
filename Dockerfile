FROM golang:1.20.3-bullseye AS builder
WORKDIR /
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/torch ./cmd/main.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /go/bin/torch .
ENTRYPOINT ["./torch"]
