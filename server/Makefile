.PHONY: build run clean vet lint test

build:
	CGO_ENABLED=0 go build -o bin/llm-apm-server ./cmd/llm-apm-server

run:
	go run ./cmd/llm-apm-server

clean:
	rm -rf bin/

vet:
	go vet ./...

lint:
	golangci-lint run

test:
	go test -race -count=1 ./...