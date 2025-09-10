# Pipeline API DynamoDB - Root Makefile
.PHONY: build-all clean-all test-all package-all deploy-all help verify-all test-packages

# Service definitions
GO_SERVICES = data-collector batch-processor vector-coordinator
PYTHON_SERVICES = embedding-api
ALL_SERVICES = $(GO_SERVICES) $(PYTHON_SERVICES)

# Build configuration
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

help:
	@echo "Pipeline API DynamoDB - Build System"
	@echo "===================================="
	@echo ""
	@echo "Main Commands:"
	@echo "  build-all      - Build all services for Lambda deployment"
	@echo "  package-all    - Create deployment packages for all services"
	@echo "  test-all       - Run tests for all services"
	@echo "  verify-all     - Verify all deployment packages"
	@echo "  test-packages  - Test integrity of all deployment packages"
	@echo "  deploy-all     - Deploy all services to AWS using deployment script"
	@echo "  clean-all      - Clean all build artifacts"
	@echo ""
	@echo "Infrastructure Commands:"
	@echo "  setup-infrastructure     - Setup AWS infrastructure (DynamoDB, S3, etc.)"
	@echo "  setup-infrastructure-env - Setup infrastructure for specific environment"
	@echo "                             Usage: make setup-infrastructure-env ENV=prod"
	@echo "  deploy-env              - Deploy to specific environment"
	@echo "                             Usage: make deploy-env ENV=prod"
	@echo ""
	@echo "Service-specific Commands:"
	@echo "  build-go       - Build all Go services"
	@echo "  build-python   - Build all Python services"
	@echo "  test-go        - Test all Go services"
	@echo "  test-python    - Test all Python services"
	@echo ""
	@echo "Development Commands:"
	@echo "  local-build    - Build all services for local development"
	@echo "  local-test     - Run all services locally for testing"
	@echo ""
	@echo "Individual Services:"
	@echo "  Use 'make <service-name> TARGET=<target>' to run specific commands"
	@echo "  Available services: $(ALL_SERVICES)"
	@echo "  Example: make data-collector TARGET=build"
	@echo ""
	@echo "Deployment Scripts:"
	@echo "  ./scripts/deploy-aws.sh           - Deploy services to AWS"
	@echo "  ./scripts/setup-infrastructure.sh - Setup AWS infrastructure"
	@echo "  ./scripts/test-deployment.sh      - Test deployment scripts"

# Build all services
build-all: build-go build-python

build-go:
	@echo "Building Go services for Lambda deployment..."
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@for service in $(GO_SERVICES); do \
		echo ""; \
		echo "Building $$service..."; \
		cd go-services/$$service && $(MAKE) build && cd ../..; \
	done
	@echo ""
	@echo "All Go services built successfully"

build-python:
	@echo "Building Python services..."
	@for service in $(PYTHON_SERVICES); do \
		echo ""; \
		echo "Building $$service..."; \
		cd python-services/$$service && $(MAKE) install && cd ../..; \
	done
	@echo ""
	@echo "All Python services built successfully"

# Local development builds
local-build:
	@echo "Building all services for local development..."
	@for service in $(GO_SERVICES); do \
		echo "Building $$service for local development..."; \
		cd go-services/$$service && $(MAKE) build-local && cd ../..; \
	done
	@echo "Local builds completed"

# Package all services for deployment
package-all: build-all
	@echo "Creating deployment packages for all services..."
	@for service in $(GO_SERVICES); do \
		echo ""; \
		echo "Packaging $$service..."; \
		cd go-services/$$service && $(MAKE) package && cd ../..; \
	done
	@for service in $(PYTHON_SERVICES); do \
		echo ""; \
		echo "Packaging $$service..."; \
		cd python-services/$$service && $(MAKE) package && cd ../..; \
	done
	@echo ""
	@echo "All services packaged successfully"

# Verify all packages
verify-all: package-all
	@echo "Verifying all deployment packages..."
	@for service in $(GO_SERVICES); do \
		echo ""; \
		echo "Verifying $$service package..."; \
		cd go-services/$$service && $(MAKE) verify-package && cd ../..; \
	done
	@for service in $(PYTHON_SERVICES); do \
		echo ""; \
		echo "Verifying $$service package..."; \
		cd python-services/$$service && $(MAKE) verify-package && cd ../..; \
	done
	@echo ""
	@echo "All packages verified successfully"

# Test package integrity
test-packages: package-all
	@echo "Testing integrity of all deployment packages..."
	@for service in $(GO_SERVICES); do \
		echo ""; \
		echo "Testing $$service package integrity..."; \
		cd go-services/$$service && $(MAKE) test-package && cd ../..; \
	done
	@for service in $(PYTHON_SERVICES); do \
		echo ""; \
		echo "Testing $$service package integrity..."; \
		cd python-services/$$service && $(MAKE) test-package && cd ../..; \
	done
	@echo ""
	@echo "All package integrity tests passed"

# Clean all build artifacts
clean-all:
	@echo "Cleaning all build artifacts..."
	@for service in $(GO_SERVICES); do \
		echo "Cleaning $$service..."; \
		cd go-services/$$service && $(MAKE) clean && cd ../..; \
	done
	@for service in $(PYTHON_SERVICES); do \
		echo "Cleaning $$service..."; \
		cd python-services/$$service && $(MAKE) clean && cd ../..; \
	done
	@echo "All artifacts cleaned"

# Test all services
test-all: test-go test-python

test-go:
	@echo "Running tests for all Go services..."
	@for service in $(GO_SERVICES); do \
		echo ""; \
		echo "Testing $$service..."; \
		cd go-services/$$service && $(MAKE) test && cd ../..; \
	done
	@echo ""
	@echo "All Go service tests passed"

test-python:
	@echo "Running tests for all Python services..."
	@for service in $(PYTHON_SERVICES); do \
		echo ""; \
		echo "Testing $$service..."; \
		cd python-services/$$service && $(MAKE) test && cd ../..; \
	done
	@echo ""
	@echo "All Python service tests passed"

# Deploy all services using AWS CLI script
deploy-all: test-packages
	@echo "Deploying all services to AWS using deployment script..."
	./scripts/deploy-aws.sh

# Deploy to specific environment
deploy-env:
	@echo "Deploying to environment: $(ENV)"
	./scripts/deploy-aws.sh --environment $(ENV)

# Setup AWS infrastructure
setup-infrastructure:
	@echo "Setting up AWS infrastructure..."
	./scripts/setup-infrastructure.sh

# Setup infrastructure for specific environment
setup-infrastructure-env:
	@echo "Setting up infrastructure for environment: $(ENV)"
	./scripts/setup-infrastructure.sh --environment $(ENV)

# Individual service deployment (legacy support)
deploy-services: test-packages
	@echo "Deploying services individually..."
	@for service in $(GO_SERVICES); do \
		echo ""; \
		echo "Deploying $$service..."; \
		cd go-services/$$service && $(MAKE) deploy && cd ../..; \
	done
	@for service in $(PYTHON_SERVICES); do \
		echo ""; \
		echo "Deploying $$service..."; \
		cd python-services/$$service && $(MAKE) deploy && cd ../..; \
	done
	@echo ""
	@echo "All services deployed successfully"

# Local testing
local-test:
	@echo "Running all services locally for testing..."
	@echo "Starting data-collector..."
	@cd go-services/data-collector && $(MAKE) local-run &
	@echo "Starting batch-processor..."
	@cd go-services/batch-processor && $(MAKE) local-run &
	@echo "Starting vector-coordinator..."
	@cd go-services/vector-coordinator && $(MAKE) local-run &
	@echo "Starting embedding-api..."
	@cd python-services/embedding-api && $(MAKE) local-run &
	@echo "All services started in background"

# Individual service commands
data-collector:
	cd go-services/data-collector && $(MAKE) $(TARGET)

batch-processor:
	cd go-services/batch-processor && $(MAKE) $(TARGET)

vector-coordinator:
	cd go-services/vector-coordinator && $(MAKE) $(TARGET)

embedding-api:
	cd python-services/embedding-api && $(MAKE) $(TARGET)

# Build status and information
status:
	@echo "Pipeline API DynamoDB - Build Status"
	@echo "===================================="
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo ""
	@echo "Services:"
	@echo "  Go Services: $(GO_SERVICES)"
	@echo "  Python Services: $(PYTHON_SERVICES)"
	@echo ""
	@echo "Package Status:"
	@for service in $(GO_SERVICES); do \
		if [ -f "go-services/$$service/$$service.zip" ]; then \
			echo "  ✓ $$service.zip"; \
		else \
			echo "  ✗ $$service.zip (not built)"; \
		fi; \
	done
	@for service in $(PYTHON_SERVICES); do \
		if [ -f "python-services/$$service/$$service.zip" ]; then \
			echo "  ✓ $$service.zip"; \
		else \
			echo "  ✗ $$service.zip (not built)"; \
		fi; \
	done