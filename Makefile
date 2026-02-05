.PHONY: build test clean

build:
	go build -o bin/shelld ./cmd/shelld

test:
	./test.sh

clean:
	rm -rf bin/

deps:
	go mod download
	go mod tidy
