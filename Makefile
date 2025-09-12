.PHONY: build run test clean docker-build docker-run docker-stop fmt vet lint

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Binary name
BINARY_NAME=compas-auth-proxy
BINARY_PATH=./cmd

# Build the application
build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(BINARY_PATH)

# Run the application
run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(BINARY_PATH)
	./$(BINARY_NAME)

# Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Test all packages
test:
	$(GOTEST) -v ./...

# Test with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Format code
fmt:
	$(GOFMT) -s -w .

# Vet code
vet:
	$(GOVET) ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Download dependencies
deps:
	$(GOMOD) download

# Build Docker image
docker-build:
	docker build -t compas-auth-proxy:latest .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker Compose
docker-stop:
	docker-compose down

# View Docker Compose logs
docker-logs:
	docker-compose logs -f

# Run security scan (requires gosec)
security:
	gosec ./...

# Install development tools
install-tools:
	go install github.com/cosmtrek/air@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.54.2

# Run all checks
check: fmt vet lint test security

# Generate self-signed certificates for development
generate-certs:
	openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes \
		-subj "/C=DE/ST=BW/L=Stuttgart/O=TransnetBW/OU=CoMPAS/CN=localhost"

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  run           - Build and run the application"
	@echo "  dev           - Run with hot reload (requires air)"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format code"
	@echo "  vet           - Vet code"
	@echo "  lint          - Lint code"
	@echo "  tidy          - Tidy dependencies"
	@echo "  deps          - Download dependencies"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run with Docker Compose"
	@echo "  docker-stop   - Stop Docker Compose"
	@echo "  docker-logs   - View Docker Compose logs"
	@echo "  security      - Run security scan"
	@echo "  install-tools - Install development tools"
	@echo "  check         - Run all checks"
	@echo "  generate-certs- Generate self-signed certificates"
	@echo "  help          - Show this help"
