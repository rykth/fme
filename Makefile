VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY  := fme

.PHONY: build test docker-shell lint vet bench clean tidy

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/fme

test:
	go test ./...

docker-shell:
	./test/docker/shell.sh

test-verbose:
	go test -v ./...

bench:
	go test -bench=. -benchmem ./internal/...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
