.PHONY: build test test-integration coverage swagger lint fmt docker-build run

APP=shikenphi
PORT=8088

build:
	go build -o bin/$(APP) ./

test:
	go test -race -cover ./...

test-integration:
	go test -race -cover -tags=integration ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

swagger:
	swag init -g main.go

lint:
	golangci-lint run ./...

fmt:
	gofmt -w . && goimports -w .

docker-build:
	docker build -t $(APP):latest -f Dockerfile ..

run:
	go run ./main.go
