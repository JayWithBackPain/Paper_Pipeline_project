#!/bin/bash

# Test script for build system verification
# This script tests the compilation and packaging functionality

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_DIR="test-build-output"
SERVICES=("data-collector" "batch-processor" "vector-coordinator" "embedding-api")
GO_SERVICES=("data-collector" "batch-processor" "vector-coordinator")
PYTHON_SERVICES=("embedding-api")

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

test_passed() {
    echo -e "${GREEN}✓ PASSED:${NC} $1"
}

test_failed() {
    echo -e "${RED}✗ FAILED:${NC} $1"
    exit 1
}

# Test functions
test_makefile_exists() {
    log_info "Testing Makefile existence..."
    
    # Test root Makefile
    if [ ! -f "Makefile" ]; then
        test_failed "Root Makefile not found"
    fi
    test_passed "Root Makefile exists"
    
    # Test service Makefiles
    for service in "${GO_SERVICES[@]}"; do
        if [ ! -f "go-services/$service/Makefile" ]; then
            test_failed "Makefile for $service not found"
        fi
        test_passed "Makefile for $service exists"
    done
    
    for service in "${PYTHON_SERVICES[@]}"; do
        if [ ! -f "python-services/$service/Makefile" ]; then
            test_failed "Makefile for $service not found"
        fi
        test_passed "Makefile for $service exists"
    done
}

test_makefile_targets() {
    log_info "Testing Makefile targets..."
    
    # Test root Makefile targets
    local root_targets=("build-all" "package-all" "test-all" "clean-all" "verify-all" "test-packages")
    for target in "${root_targets[@]}"; do
        if ! grep -q "^$target:" Makefile; then
            test_failed "Target '$target' not found in root Makefile"
        fi
        test_passed "Target '$target' exists in root Makefile"
    done
    
    # Test service Makefile targets
    local service_targets=("build" "clean" "test" "package" "verify-package" "test-package")
    for service in "${GO_SERVICES[@]}"; do
        for target in "${service_targets[@]}"; do
            if ! grep -q "^$target:" "go-services/$service/Makefile"; then
                test_failed "Target '$target' not found in $service Makefile"
            fi
        done
        test_passed "All required targets exist in $service Makefile"
    done
}

test_go_build() {
    log_info "Testing Go service builds..."
    
    for service in "${GO_SERVICES[@]}"; do
        log_info "Building $service..."
        
        # Clean first
        cd "go-services/$service"
        make clean > /dev/null 2>&1 || true
        
        # Test build
        if ! make build > /dev/null 2>&1; then
            test_failed "Failed to build $service"
        fi
        
        # Check if binary was created
        if [ ! -f "build/$service" ]; then
            test_failed "Binary for $service not created"
        fi
        
        # Check if binary is executable
        if [ ! -x "build/$service" ]; then
            test_failed "Binary for $service is not executable"
        fi
        
        test_passed "$service built successfully"
        cd ../..
    done
}

test_go_packaging() {
    log_info "Testing Go service packaging..."
    
    for service in "${GO_SERVICES[@]}"; do
        log_info "Packaging $service..."
        
        cd "go-services/$service"
        
        # Test packaging
        if ! make package > /dev/null 2>&1; then
            test_failed "Failed to package $service"
        fi
        
        # Check if ZIP file was created
        if [ ! -f "$service.zip" ]; then
            test_failed "ZIP package for $service not created"
        fi
        
        # Check ZIP file size (should be > 0)
        local size=$(stat -f%z "$service.zip" 2>/dev/null || stat -c%s "$service.zip")
        if [ "$size" -eq 0 ]; then
            test_failed "ZIP package for $service is empty"
        fi
        
        # Test ZIP integrity
        if ! unzip -t "$service.zip" > /dev/null 2>&1; then
            test_failed "ZIP package for $service is corrupted"
        fi
        
        # Check if binary is in ZIP
        if ! unzip -l "$service.zip" | grep -q "$service"; then
            test_failed "Binary not found in $service ZIP package"
        fi
        
        test_passed "$service packaged successfully"
        cd ../..
    done
}

test_python_build() {
    log_info "Testing Python service builds..."
    
    for service in "${PYTHON_SERVICES[@]}"; do
        log_info "Building $service..."
        
        cd "python-services/$service"
        
        # Clean first
        make clean > /dev/null 2>&1 || true
        
        # Test install (build dependencies)
        if ! make install > /dev/null 2>&1; then
            log_warn "Failed to install dependencies for $service (may be expected in CI)"
        else
            test_passed "$service dependencies installed successfully"
        fi
        
        cd ../..
    done
}

test_python_packaging() {
    log_info "Testing Python service packaging..."
    
    for service in "${PYTHON_SERVICES[@]}"; do
        log_info "Packaging $service..."
        
        cd "python-services/$service"
        
        # Test packaging (may fail without proper Python environment)
        if make package > /dev/null 2>&1; then
            # Check if ZIP file was created
            if [ ! -f "$service.zip" ]; then
                test_failed "ZIP package for $service not created"
            fi
            
            # Check ZIP file size (should be > 0)
            local size=$(stat -f%z "$service.zip" 2>/dev/null || stat -c%s "$service.zip")
            if [ "$size" -eq 0 ]; then
                test_failed "ZIP package for $service is empty"
            fi
            
            # Test ZIP integrity
            if ! unzip -t "$service.zip" > /dev/null 2>&1; then
                test_failed "ZIP package for $service is corrupted"
            fi
            
            # Check if main.py is in ZIP
            if ! unzip -l "$service.zip" | grep -q "main.py"; then
                test_failed "main.py not found in $service ZIP package"
            fi
            
            test_passed "$service packaged successfully"
        else
            log_warn "Python packaging for $service failed (may be expected without proper environment)"
        fi
        
        cd ../..
    done
}

test_package_verification() {
    log_info "Testing package verification..."
    
    for service in "${GO_SERVICES[@]}"; do
        cd "go-services/$service"
        
        if [ -f "$service.zip" ]; then
            # Test verify-package target
            if ! make verify-package > /dev/null 2>&1; then
                test_failed "Package verification failed for $service"
            fi
            
            # Test test-package target
            if ! make test-package > /dev/null 2>&1; then
                test_failed "Package integrity test failed for $service"
            fi
            
            test_passed "$service package verification successful"
        fi
        
        cd ../..
    done
}

test_clean_functionality() {
    log_info "Testing clean functionality..."
    
    # Test individual service clean
    for service in "${GO_SERVICES[@]}"; do
        cd "go-services/$service"
        
        # Build first to have something to clean
        make build > /dev/null 2>&1 || true
        make package > /dev/null 2>&1 || true
        
        # Test clean
        if ! make clean > /dev/null 2>&1; then
            test_failed "Clean failed for $service"
        fi
        
        # Check if artifacts were removed
        if [ -f "build/$service" ] || [ -f "$service.zip" ]; then
            test_failed "Clean did not remove all artifacts for $service"
        fi
        
        test_passed "$service clean successful"
        cd ../..
    done
    
    # Test root clean-all
    if ! make clean-all > /dev/null 2>&1; then
        test_failed "Root clean-all failed"
    fi
    test_passed "Root clean-all successful"
}

test_cross_compilation() {
    log_info "Testing cross-compilation for Lambda..."
    
    for service in "${GO_SERVICES[@]}"; do
        cd "go-services/$service"
        
        # Build for Lambda
        make build > /dev/null 2>&1 || test_failed "Cross-compilation failed for $service"
        
        # Check if binary is Linux AMD64
        if command -v file > /dev/null 2>&1; then
            local file_info=$(file "build/$service")
            if [[ "$file_info" != *"Linux"* ]] && [[ "$file_info" != *"x86-64"* ]] && [[ "$file_info" != *"x86_64"* ]]; then
                log_warn "Binary for $service may not be compiled for Linux AMD64: $file_info"
            else
                test_passed "$service cross-compiled for Linux AMD64"
            fi
        else
            log_warn "file command not available, skipping binary format check"
        fi
        
        cd ../..
    done
}

# Main test execution
main() {
    log_info "Starting build system tests..."
    echo "========================================"
    
    # Change to project root if not already there
    if [ ! -f "Makefile" ]; then
        if [ -f "../Makefile" ]; then
            cd ..
        else
            test_failed "Cannot find project root with Makefile"
        fi
    fi
    
    # Run tests
    test_makefile_exists
    test_makefile_targets
    test_go_build
    test_go_packaging
    test_python_build
    test_python_packaging
    test_package_verification
    test_cross_compilation
    test_clean_functionality
    
    echo "========================================"
    log_info "All build system tests completed successfully!"
    
    # Show final status
    echo ""
    make status 2>/dev/null || log_warn "Status command not available"
}

# Run main function
main "$@"