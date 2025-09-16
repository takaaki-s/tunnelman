# Tunnelman - Simple Makefile

BINARY_NAME=tunnelman
MAIN_FILE=cmd/tunnelman/main.go
BUILD_DIR=build

# Default target
.DEFAULT_GOAL := help

# help: Show this help message
help:
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Targets:'
	@echo '  build      Build the binary'
	@echo '  run        Run the application'
	@echo '  test       Run tests'
	@echo '  clean      Remove binary and build artifacts'
	@echo '  install    Install the binary to $$GOPATH/bin'

# build: Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

# run: Run the application
run:
	go run $(MAIN_FILE)

# test: Run tests
test:
	go test -v ./...

# clean: Remove binary and build artifacts
clean:
	go clean
	rm -rf $(BUILD_DIR)

# install: Install the binary to $GOPATH/bin
install:
	go install ./cmd/tunnelman

.PHONY: help build run test clean install