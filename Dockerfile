FROM golang:1.20.3-bullseye AS builder
WORKDIR /
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download
COPY . .
COPY cmd/main.go ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.gitCommit=$(git rev-list -1 HEAD)" -o /go/bin/torch ./cmd/main.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /go/bin/torch .
ENTRYPOINT ["./torch"]
