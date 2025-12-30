.PHONY: build build-cli build-server run-server run-cli test clean

# Build both server and CLI
build: build-server build-cli

# Build the web server
build-server:
	@echo "Building web server..."
	@go build -o bin/server .

# Build the CLI
build-cli:
	@echo "Building CLI..."
	@go build -o bin/vidcli ./cmd/cli

# Run the web server
run-server:
	@go run .

# Run the CLI
run-cli:
	@go run ./cmd/cli/main.go

# Run tests
test:
	@go test -v ./...

# Clean build artifacts
clean:
	@rm -rf bin/
	@rm -rf test_output/
	@echo "Cleaned build artifacts"

# Run both (server in background, CLI in foreground)
run-both: build
	@./bin/server &
	@sleep 2
	@./bin/vidcli