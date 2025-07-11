# spa-server Makefile
# Cross-platform build targets

BINARY_NAME=spa-server
VERSION?=latest
BUILD_DIR=build

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-s -w"
CGO_ENABLED=0

# Default target
.PHONY: all
all: clean build-all

# Clean build directory
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build for current platform
.PHONY: build
build: $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build for all platforms
.PHONY: build-all
build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64

# macOS Intel
.PHONY: build-darwin-amd64
build-darwin-amd64: $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .

# macOS Apple Silicon
.PHONY: build-darwin-arm64
build-darwin-arm64: $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .

# Linux AMD64
.PHONY: build-linux-amd64
build-linux-amd64: $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .

# Linux ARM64
.PHONY: build-linux-arm64
build-linux-arm64: $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .

# Windows AMD64
.PHONY: build-windows-amd64
build-windows-amd64: $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# Windows ARM64
.PHONY: build-windows-arm64
build-windows-arm64: $(BUILD_DIR)
	GOOS=windows GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe .

# Run tests
.PHONY: test
test:
	$(GOTEST) -v ./...

# Create compressed archives for distribution
.PHONY: package
package: build-all
	cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	cd $(BUILD_DIR) && tar -czf $(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	cd $(BUILD_DIR) && zip $(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	cd $(BUILD_DIR) && zip $(BINARY_NAME)-windows-arm64.zip $(BINARY_NAME)-windows-arm64.exe

# Docker build
.PHONY: docker
docker:
	docker build -t $(BINARY_NAME):$(VERSION) .

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all              - Clean and build for all platforms"
	@echo "  clean            - Remove build directory"
	@echo "  deps             - Download dependencies"
	@echo "  build            - Build for current platform"
	@echo "  build-all        - Build for all supported platforms"
	@echo "  build-darwin-amd64  - Build for macOS Intel"
	@echo "  build-darwin-arm64  - Build for macOS Apple Silicon"
	@echo "  build-linux-amd64   - Build for Linux AMD64"
	@echo "  build-linux-arm64   - Build for Linux ARM64"
	@echo "  build-windows-amd64 - Build for Windows AMD64"
	@echo "  build-windows-arm64 - Build for Windows ARM64"
	@echo "  test             - Run tests"
	@echo "  package          - Create compressed archives for distribution"
	@echo "  docker           - Build Docker image"
	@echo "  help             - Show this help message"