.PHONY: help build test lint clean deploy-dev deploy-prod deploy-dev-dry deploy-prod-dry build-deploy

# Default target
help:
	@echo "Available commands:"
	@echo "  build         - Build the server binary"
	@echo "  build-deploy  - Build the deploy tool"
	@echo "  test          - Run tests"
	@echo "  lint          - Run linter"
	@echo "  clean         - Clean build artifacts"
	@echo ""
	@echo "Deployment commands:"
	@echo "  deploy-dev    - Deploy to development environment"
	@echo "  deploy-dev-dry- Preview development deployment"
	@echo "  deploy-prod   - Deploy to production environment"
	@echo "  deploy-prod-dry- Preview production deployment"
	@echo ""
	@echo "Local development:"
	@echo "  dev           - Run local development server"

# Build targets
build:
	go build -o bin/server ./cmd/server

build-deploy:
	go build -o bin/deploy cmd/deploy/main.go
	chmod +x bin/deploy

# Test and quality
test:
	go test -v ./...

test-coverage:
	go test -v -cover ./...

lint:
	golangci-lint run

# Clean
clean:
	rm -rf bin/

# Deployment commands
deploy-dev: ensure-deploy
	./bin/deploy dev

deploy-dev-dry: ensure-deploy
	./bin/deploy dev --dry-run

deploy-prod: ensure-deploy
	./bin/deploy prod

deploy-prod-dry: ensure-deploy
	./bin/deploy prod --dry-run

# Local development
dev:
	@echo "Starting local development server..."
	@echo "Make sure to set environment variables: GOOGLE_API_KEY, GOOGLE_CSE_ID, GCP_PROJECT_ID"
	DEBUG=true go run ./cmd/server

# Check if deploy binary exists, build if not
ensure-deploy:
	@if [ ! -f bin/deploy ]; then make build-deploy; fi