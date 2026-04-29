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

# Run Python frontend only (needs Go running)
run-py:
	@echo "🐍 Starting Python UI on :$(PY_PORT)..."
	cd $(PY_DIR) && python3 seo_analyser.py

# Run both (development)
dev:
	@echo "🚀 Starting both services..."
	@make run-go & make run-py

# Install dependencies
deps:
	@echo "📦 Installing Go dependencies..."
	cd $(GO_DIR) && go mod tidy
	@echo "📦 Python dependencies..."
	cd $(PY_DIR) && pip install -r requirements.txt

# Clean build artifacts
clean:
	rm -rf bin/
	cd $(GO_DIR) && go clean

# Run tests
test:
	cd $(GO_DIR) && go test ./...

# Benchmark Python vs Go
benchmark:
	@./scripts/benchmark.sh