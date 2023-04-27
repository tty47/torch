# Conntrack Exporter service
SERVICE_NAME=infra-orch
PROJECT_NAME=docker-infra-orch
REPOSITORY_NAME=docker-infra-orch
NAMESPACE_NAME=infra-orch

#### #### #### #### #### #### ####
# Load .env in order to use vars
ifneq (,$(wildcard ./.env))
    include .env
    export
endif
#### #### #### #### #### #### ####

#### #### #### #### #### #### ####

# Go
.PHYONY: run build test test_cover get docs
run:
	go run ./main.go

build:
	go build -o ./bin/${PROJECT_NAME} ./main.go

test:
	go test ./... -v -cover .

test_cover:
	go test ./... -v -coverprofile cover.out
	go tool cover -func ./cover.out | grep total | awk '{print $3}'

get:
	go get ./...

docs:
	godoc -http=:6060

golangci:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.42.1
	golangci-lint run ./... --verbose --no-config --out-format checkstyle > golangci-lint.out || true

# Docker
docker_build:
	docker build -f Dockerfile -t ${PROJECT_NAME} -t ${PROJECT_NAME}:latest .

docker_build_local_push:
	docker build  -f Dockerfile -t ${PROJECT_NAME} .
	docker tag ${PROJECT_NAME}:latest localhost:5000/${REPOSITORY_NAME}:latest
	docker push localhost:5000/${REPOSITORY_NAME}:latest

docker_run:
	docker run -p 8080:8080 ${PROJECT_NAME}:latest
