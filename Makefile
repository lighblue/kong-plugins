.PHONY: build ut integration

GO ?= go
GOOS ?= linux
GOARCH ?= $(shell $(GO) env GOARCH)
BINARY := build/$(GOOS)-$(GOARCH)/kong-auth

# Build a Linux plugin for the host architecture; override GOOS/GOARCH as needed.
build:
	mkdir -p $(dir $(BINARY))
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -o $(BINARY) ./cmd/server

ut:
	$(GO) test ./...

integration:
	./tests/run.sh
