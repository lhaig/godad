.PHONY: build run test lint docker-build docker-run test-coverage

build:
	go build -o bin/godad

run: build
	./bin/godad

test:
	go test -v ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

docker-build:
	docker build -t godad .

docker-run: docker-build
	docker run --rm godad

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html