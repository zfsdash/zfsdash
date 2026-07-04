BINARY := zfsdash
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DIR := build
LDFLAGS := -s -w \
           -X github.com/zfsdash/zfsdash/internal/version.Version=$(VERSION) \
           -X github.com/zfsdash/zfsdash/internal/version.Commit=$(COMMIT)

.PHONY: build build-all clean run

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) .

build-all: clean
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-arm64 .
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-freebsd-amd64 .

clean:
	rm -rf $(BUILD_DIR)

run:
	go run . -config config.example.yaml
