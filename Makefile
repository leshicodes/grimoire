.PHONY: build run test test-verbose tidy docker-build docker-run clean

build:
	go build -o grimoire .

run:
	go run .

test:
	go test ./...

test-verbose:
	go test -v ./...

tidy:
	go mod tidy

docker-build:
	docker build -t grimoire .

docker-run:
	docker compose up

clean:
	rm -f grimoire
