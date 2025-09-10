BINARY_NAME=data-collector
LAMBDA_ZIP=data-collector.zip
BUILD_DIR=build
DIST_DIR=dist

# Go build flags for Lambda
GO_BUILD_FLAGS=-ldflags="-s -w" -trimpath

.PHONY: build clean test package deploy local-run verify-package test-package

# Cross-compilation for AWS Lambda (Linux AMD64)
build:
	@echo "Building $(BINARY_NAME) for AWS Lambda..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GO_BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for local development (native architecture)
build-local:
	@echo "Building $(BINARY_NAME) for local development..."
	@mkdir -p $(BUILD_DIR)
	go build $(GO_BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-local main.go
	@echo "Local build completed: $(BUILD_DIR)/$(BINARY_NAME)-local"

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(LAMBDA_ZIP)

test:
	@echo "Running tests for $(BINARY_NAME)..."
	go test -v ./...
	@echo "All tests passed"

# Create Lambda deployment package
package: build
	@echo "Creating Lambda deployment package..."
	@mkdir -p $(DIST_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(DIST_DIR)/
	@cd $(DIST_DIR) && zip -r ../$(LAMBDA_ZIP) .
	@echo "Package created: $(LAMBDA_ZIP)"

# Verify package contents
verify-package: package
	@echo "Verifying package contents..."
	@unzip -l $(LAMBDA_ZIP)
	@echo "Package verification completed"

# Test package integrity
test-package: package
	@echo "Testing package integrity..."
	@test -f $(LAMBDA_ZIP) || (echo "Package file not found" && exit 1)
	@test $$(stat -f%z $(LAMBDA_ZIP) 2>/dev/null || stat -c%s $(LAMBDA_ZIP)) -gt 0 || (echo "Package file is empty" && exit 1)
	@unzip -t $(LAMBDA_ZIP) > /dev/null || (echo "Package file is corrupted" && exit 1)
	@echo "Package integrity test passed"

deploy: test-package
	@echo "Deploying $(BINARY_NAME) to AWS Lambda..."
	aws lambda update-function-code \
		--function-name $(BINARY_NAME) \
		--zip-file fileb://$(LAMBDA_ZIP)
	@echo "Deployment completed"

local-run: build-local
	@echo "Running $(BINARY_NAME) locally..."
	./$(BUILD_DIR)/$(BINARY_NAME)-local