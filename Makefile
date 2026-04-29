.PHONY: all build run dev clean test benchmark

# Variables
GO_DIR := backend
PY_DIR := frontend
GO_PORT := 8080
PY_PORT := 5000

all: build

# Build Go binary
build:
	@echo "🔨 Building Go crawler..."
	cd $(GO_DIR) && go build -o ../bin/seo-crawler ./cmd/server

# Run Go server only
run-go:
	@echo "🚀 Starting Go crawler on :$(GO_PORT)..."
	cd $(GO_DIR) && go run ./cmd/server

# Run both (development)
dev:
	@echo "🚀 Starting go service..."
	@make run-go

# Install dependencies
deps:
	@echo "📦 Installing Go dependencies..."
	cd $(GO_DIR) && go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/
	cd $(GO_DIR) && go clean

# Run tests
test:
	cd $(GO_DIR) && go test ./...
