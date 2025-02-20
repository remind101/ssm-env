.DEFAULT_GOAL := all

VERSION := $(shell git describe --tags --match 'v*')
APP_NAME := ssm-env
OUTPUT_DIR := bin
CGO_ENABLED ?= 0
ARGS :=
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

all: clean build

build:
	@mkdir -p $(OUTPUT_DIR)
	@echo "Building $(APP_NAME) for multiple platforms..."
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d'/' -f1); \
		GOARCH=$$(echo $$platform | cut -d'/' -f2); \
		echo "Building for $$GOOS/$$GOARCH..."; \
		BIN_NAME=$(OUTPUT_DIR)/$(APP_NAME)-$$GOOS-$$GOARCH$$( [ "$$GOOS" = "windows" ] && echo .exe ); \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$$GOOS GOARCH=$$GOARCH go build -o $$BIN_NAME -ldflags "-X main.version=$(VERSION)" . || exit 1; \
	done
	@echo "Build completed."

.PHONY: run
run:
	CGO_ENABLED=$(CGO_ENABLED) go run -ldflags "-X main.version=$(VERSION)" . $(ARGS)

.PHONY: clean
clean:
	rm -rf $(OUTPUT_DIR)
	@echo "Cleaned build artifacts."

.PHONY: test
test:
	go test -race $(shell go list ./... | grep -v /vendor/)

.PHONY: all build clean test