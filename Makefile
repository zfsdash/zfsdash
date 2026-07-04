.PHONY: build build-all release clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  = -ldflags "-s -w -X main.Version=$(VERSION)"
OUT      = dist

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(OUT)/zfsdash .

build-all: $(OUT)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(OUT)/zfsdash-linux-amd64   .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(OUT)/zfsdash-linux-arm64   .
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build $(LDFLAGS) -o $(OUT)/zfsdash-freebsd-amd64 .
	@echo "Built: $(shell ls -lh $(OUT)/zfsdash-* | awk '{print $$5, $$9}')"

release: build-all
	gh release create $(VERSION) $(OUT)/zfsdash-* \
		--repo zfsdash/zfsdash \
		--title "$(VERSION)" \
		--generate-notes

$(OUT):
	mkdir -p $(OUT)

clean:
	rm -rf $(OUT)
