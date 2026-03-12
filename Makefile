# Tunnelman - Makefile

BINARY_NAME=tunnelman
MAIN_PKG=./cmd/tunnelman
BUILD_DIR=build

.DEFAULT_GOAL := help

help:
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@echo '  build      Build the binary'
	@echo '  run        Run the application'
	@echo '  test       Run tests'
	@echo '  lint       Run linter'
	@echo '  fmt        Check formatting'
	@echo '  clean      Remove binary and build artifacts'
	@echo '  install    Install the binary to $$GOPATH/bin'

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

run:
	go run $(MAIN_PKG)

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

fmt:
	@test -z "$$(gofmt -l .)" || (gofmt -d . && exit 1)

clean:
	go clean
	rm -rf $(BUILD_DIR)

install:
	go install $(MAIN_PKG)

.PHONY: help build run test lint fmt clean install
