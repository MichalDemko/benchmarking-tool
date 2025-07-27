# Makefile for the Go Benchmarking Tool

# Variables
BINARY_NAME=benchmarking-tool.out
CONFIG_FILE ?= config-examples/advanced-example.yml

# Default target
.DEFAULT_GOAL := help

# Build the application
build:
	@echo "Building the application..."
	go mod tidy
	go build -o $(BINARY_NAME) .
	@echo "Build successful!"

# Run the application
run: build
	@echo "Running the application with config: $(CONFIG_FILE)"
	./$(BINARY_NAME) run $(CONFIG_FILE)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean up build artifacts
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)

# Display help
help:
	@echo "Available commands:"
	@echo "  build    - Build the application"
	@echo "  run      - Run the application (default config: $(CONFIG_FILE))"
	@echo "           - To use a different config: make run CONFIG_FILE=path/to/your/config.yml"
	@echo "  test     - Run tests"
	@echo "  clean    - Clean up build artifacts"

.PHONY: build run test clean help
