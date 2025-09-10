FUNCTION_NAME=embedding-api
LAMBDA_ZIP=embedding-api.zip
PACKAGE_DIR=package
BUILD_DIR=build
VENV_DIR=venv

# Python version for Lambda (3.9)
PYTHON_VERSION=python3.9

.PHONY: install clean test package deploy local-run verify-package test-package setup-venv

# Setup virtual environment for consistent builds
setup-venv:
	@echo "Setting up virtual environment..."
	@test -d $(VENV_DIR) || python3 -m venv $(VENV_DIR)
	@echo "Virtual environment ready"

# Install dependencies in virtual environment
install: setup-venv
	@echo "Installing dependencies..."
	@. $(VENV_DIR)/bin/activate && pip install --upgrade pip
	@. $(VENV_DIR)/bin/activate && pip install -r requirements.txt
	@echo "Dependencies installed"

clean:
	@echo "Cleaning build artifacts..."
	rm -rf __pycache__ *.pyc $(LAMBDA_ZIP) $(PACKAGE_DIR)/ $(BUILD_DIR)/ .pytest_cache/
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	find . -name "*.pyc" -delete 2>/dev/null || true

# Run tests with coverage
test: install
	@echo "Running tests for $(FUNCTION_NAME)..."
	@. $(VENV_DIR)/bin/activate && python -m pytest test_*.py -v --tb=short
	@echo "All tests passed"

# Create optimized Lambda deployment package
package: clean install
	@echo "Creating Lambda deployment package..."
	@mkdir -p $(PACKAGE_DIR)
	@echo "Installing production dependencies..."
	@. $(VENV_DIR)/bin/activate && pip install -r requirements.txt -t $(PACKAGE_DIR)/ --no-deps --upgrade
	@echo "Copying source code..."
	@cp -r *.py $(PACKAGE_DIR)/
	@cp -r api/ $(PACKAGE_DIR)/ 2>/dev/null || true
	@cp -r models/ $(PACKAGE_DIR)/ 2>/dev/null || true
	@echo "Removing unnecessary files..."
	@find $(PACKAGE_DIR) -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@find $(PACKAGE_DIR) -name "*.pyc" -delete 2>/dev/null || true
	@find $(PACKAGE_DIR) -name "*.pyo" -delete 2>/dev/null || true
	@find $(PACKAGE_DIR) -name "*.dist-info" -exec rm -rf {} + 2>/dev/null || true
	@find $(PACKAGE_DIR) -name "*.egg-info" -exec rm -rf {} + 2>/dev/null || true
	@echo "Creating ZIP archive..."
	@cd $(PACKAGE_DIR) && zip -r ../$(LAMBDA_ZIP) . -x "*.pyc" "*/__pycache__/*"
	@echo "Package created: $(LAMBDA_ZIP)"

# Verify package contents and structure
verify-package: package
	@echo "Verifying package contents..."
	@unzip -l $(LAMBDA_ZIP) | head -20
	@echo "..."
	@echo "Total files: $$(unzip -l $(LAMBDA_ZIP) | tail -1 | awk '{print $$2}')"
	@echo "Package size: $$(ls -lh $(LAMBDA_ZIP) | awk '{print $$5}')"
	@echo "Package verification completed"

# Test package integrity and structure
test-package: package
	@echo "Testing package integrity..."
	@test -f $(LAMBDA_ZIP) || (echo "Package file not found" && exit 1)
	@test $$(stat -f%z $(LAMBDA_ZIP) 2>/dev/null || stat -c%s $(LAMBDA_ZIP)) -gt 0 || (echo "Package file is empty" && exit 1)
	@unzip -t $(LAMBDA_ZIP) > /dev/null || (echo "Package file is corrupted" && exit 1)
	@echo "Checking for main.py in package..."
	@unzip -l $(LAMBDA_ZIP) | grep -q "main.py" || (echo "main.py not found in package" && exit 1)
	@echo "Checking for required dependencies..."
	@unzip -l $(LAMBDA_ZIP) | grep -q "transformers" || (echo "transformers dependency not found" && exit 1)
	@echo "Package integrity test passed"

deploy: test-package
	@echo "Deploying $(FUNCTION_NAME) to AWS Lambda..."
	aws lambda update-function-code \
		--function-name $(FUNCTION_NAME) \
		--zip-file fileb://$(LAMBDA_ZIP)
	@echo "Deployment completed"

local-run: install
	@echo "Running $(FUNCTION_NAME) locally..."
	@. $(VENV_DIR)/bin/activate && python main.py

# Development helpers
dev-install: setup-venv
	@echo "Installing development dependencies..."
	@. $(VENV_DIR)/bin/activate && pip install --upgrade pip
	@. $(VENV_DIR)/bin/activate && pip install -r requirements.txt
	@. $(VENV_DIR)/bin/activate && pip install pytest-cov black flake8
	@echo "Development environment ready"

lint: dev-install
	@echo "Running code linting..."
	@. $(VENV_DIR)/bin/activate && flake8 *.py api/ models/ --max-line-length=88 --ignore=E203,W503
	@echo "Linting completed"

format: dev-install
	@echo "Formatting code..."
	@. $(VENV_DIR)/bin/activate && black *.py api/ models/
	@echo "Code formatting completed"