VERSION := 1.0.0
BUILDDIR := build
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

all: clean windows linux darwin

clean:
	rm -rf $(BUILDDIR)
	mkdir -p $(BUILDDIR)

windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILDDIR)/server-windows-amd64.exe .
	GOOS=windows GOARCH=386   go build $(LDFLAGS) -o $(BUILDDIR)/server-windows-386.exe .

linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-arm64 .

darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILDDIR)/server-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILDDIR)/server-darwin-arm64 .

SWAG := $(shell go env GOPATH)/bin/swag
SWAG_SRC := main.go
SWAG_OUT := docs

.PHONY: all clean windows linux darwin swag swag-check swag-install

swag-install:
	go install github.com/swaggo/swag/cmd/swag@latest

swag:
	$(SWAG) init -g $(SWAG_SRC) -o $(SWAG_OUT) --parseInternal
	@cp -f $(SWAG_OUT)/swagger.json ../../openapi.json
	@echo "✅ swagger docs regenerated → docs/ + openapi.json"

swag-check: swag
	@if ! git diff --quiet $(SWAG_OUT)/ ../../openapi.json; then \
		echo "❌ swagger docs are stale. Run 'make swag' and commit the changes."; \
		git diff --stat $(SWAG_OUT)/ ../../openapi.json; \
		exit 1; \
	else \
		echo "✅ swagger docs up to date"; \
	fi

.PHONY: all clean windows linux darwin
