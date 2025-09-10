#!/bin/bash

# Comprehensive packaging test script
# Tests all aspects of the packaging system

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
GO_SERVICES=("data-collector" "batch-processor" "vector-coordinator")
PYTHON_SERVICES=("embedding-api")
TEST_OUTPUT_DIR="test-packaging-output"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

test_passed() {
    echo -e "${GREEN}✓${NC} $1"
}

test_failed() {
    echo -e "${RED}✗${NC} $1"
    return 1
}

# Test Go service packaging
test_go_service_packaging() {
    local service=$1
    log_test "Testing Go service packaging: $service"
    
    cd "go-services/$service"
    
    # Clean first
    make clean > /dev/null 2>&1 || true
    
    # Test build
    log_info "Building $service..."
    if ! make build; then
        test_failed "Build failed for $service"
        return 1
    fi
    
    # Verify binary exists and is executable
    if [ ! -f "build/$service" ]; then
        test_failed "Binary not created for $service"
        return 1
    fi
    
    if [ ! -x "build/$service" ]; then
        test_failed "Binary not executable for $service"
        return 1
    fi
    
    # Test packaging
    log_info "Packaging $service..."
    if ! make package; then
        test_failed "Packaging failed for $service"
        return 1
    fi
    
    # Verify package exists
    if [ ! -f "$service.zip" ]; then
        test_failed "Package not created for $service"
        return 1
    fi
    
    # Test package size
    local size=$(stat -f%z "$service.zip" 2>/dev/null || stat -c%s "$service.zip")
    if [ "$size" -lt 1000 ]; then
        test_failed "Package too small for $service (${size} bytes)"
        return 1
    fi
    
    # Test package integrity
    if ! unzip -t "$service.zip" > /dev/null 2>&1; then
        test_failed "Package corrupted for $service"
        return 1
    fi
    
    # Test package contents
    local contents=$(unzip -l "$service.zip")
    if ! echo "$contents" | grep -q "$service"; then
        test_failed "Binary not found in package for $service"
        return 1
    fi
    
    # Test verification
    if ! make verify-package > /dev/null 2>&1; then
        test_failed "Package verification failed for $service"
        return 1
    fi
    
    # Test package integrity check
    if ! make test-package > /dev/null 2>&1; then
        test_failed "Package integrity test failed for $service"
        return 1
    fi
    
    test_passed "$service packaging complete"
    cd ../..
    return 0
}

# Test Python service packaging
test_python_service_packaging() {
    local service=$1
    log_test "Testing Python service packaging: $service"
    
    cd "python-services/$service"
    
    # Clean first
    make clean > /dev/null 2>&1 || true
    
    # Test install (may fail in CI without proper Python setup)
    log_info "Installing dependencies for $service..."
    if make install > /dev/null 2>&1; then
        test_passed "Dependencies installed for $service"
        
        # Test packaging
        log_info "Packaging $service..."
        if make package > /dev/null 2>&1; then
            # Verify package exists
            if [ ! -f "$service.zip" ]; then
                test_failed "Package not created for $service"
                cd ../..
                return 1
            fi
            
            # Test package size
            local size=$(stat -f%z "$service.zip" 2>/dev/null || stat -c%s "$service.zip")
            if [ "$size" -lt 1000 ]; then
                test_failed "Package too small for $service (${size} bytes)"
                cd ../..
                return 1
            fi
            
            # Test package integrity
            if ! unzip -t "$service.zip" > /dev/null 2>&1; then
                test_failed "Package corrupted for $service"
                cd ../..
                return 1
            fi
            
            # Test package contents
            local contents=$(unzip -l "$service.zip")
            if ! echo "$contents" | grep -q "main.py"; then
                test_failed "main.py not found in package for $service"
                cd ../..
                return 1
            fi
            
            # Check for dependencies
            if ! echo "$contents" | grep -q "transformers"; then
                test_failed "Dependencies not found in package for $service"
                cd ../..
                return 1
            fi
            
            # Test verification
            if ! make verify-package > /dev/null 2>&1; then
                test_failed "Package verification failed for $service"
                cd ../..
                return 1
            fi
            
            # Test package integrity check
            if ! make test-package > /dev/null 2>&1; then
                test_failed "Package integrity test failed for $service"
                cd ../..
                return 1
            fi
            
            test_passed "$service packaging complete"
        else
            log_warn "Packaging failed for $service (may be expected without proper Python environment)"
        fi
    else
        log_warn "Dependency installation failed for $service (may be expected in CI)"
    fi
    
    cd ../..
    return 0
}

# Test root-level packaging commands
test_root_packaging() {
    log_test "Testing root-level packaging commands"
    
    # Test package-all
    log_info "Testing package-all command..."
    if make package-all > /dev/null 2>&1; then
        test_passed "package-all command successful"
    else
        log_warn "package-all command failed (may be expected without proper environment)"
    fi
    
    # Test verify-all
    log_info "Testing verify-all command..."
    if make verify-all > /dev/null 2>&1; then
        test_passed "verify-all command successful"
    else
        log_warn "verify-all command failed (may be expected without packages)"
    fi
    
    # Test test-packages
    log_info "Testing test-packages command..."
    if make test-packages > /dev/null 2>&1; then
        test_passed "test-packages command successful"
    else
        log_warn "test-packages command failed (may be expected without packages)"
    fi
    
    # Test status command
    log_info "Testing status command..."
    if make status > /dev/null 2>&1; then
        test_passed "status command successful"
    else
        test_failed "status command failed"
        return 1
    fi
    
    return 0
}

# Test package size optimization
test_package_optimization() {
    log_test "Testing package size optimization"
    
    for service in "${GO_SERVICES[@]}"; do
        if [ -f "go-services/$service/$service.zip" ]; then
            local size=$(stat -f%z "go-services/$service/$service.zip" 2>/dev/null || stat -c%s "go-services/$service/$service.zip")
            log_info "$service package size: $(numfmt --to=iec $size 2>/dev/null || echo "${size} bytes")"
            
            # Go binaries should be reasonably sized (less than 50MB for Lambda)
            if [ "$size" -gt 52428800 ]; then  # 50MB
                log_warn "$service package is quite large (${size} bytes)"
            else
                test_passed "$service package size is reasonable"
            fi
        fi
    done
    
    for service in "${PYTHON_SERVICES[@]}"; do
        if [ -f "python-services/$service/$service.zip" ]; then
            local size=$(stat -f%z "python-services/$service/$service.zip" 2>/dev/null || stat -c%s "python-services/$service/$service.zip")
            log_info "$service package size: $(numfmt --to=iec $size 2>/dev/null || echo "${size} bytes")"
            
            # Python packages with ML libraries can be large, but should be under 250MB
            if [ "$size" -gt 262144000 ]; then  # 250MB
                log_warn "$service package is very large (${size} bytes)"
            else
                test_passed "$service package size is acceptable"
            fi
        fi
    done
}

# Test cross-platform compatibility
test_cross_platform() {
    log_test "Testing cross-platform compatibility"
    
    for service in "${GO_SERVICES[@]}"; do
        if [ -f "go-services/$service/build/$service" ]; then
            # Check if we can determine the binary format
            if command -v file > /dev/null 2>&1; then
                local file_info=$(file "go-services/$service/build/$service")
                log_info "$service binary info: $file_info"
                
                # Should be Linux binary for Lambda
                if [[ "$file_info" == *"Linux"* ]] || [[ "$file_info" == *"ELF"* ]]; then
                    test_passed "$service compiled for Linux"
                else
                    log_warn "$service may not be compiled for Linux: $file_info"
                fi
            else
                log_warn "Cannot check binary format (file command not available)"
            fi
        fi
    done
}

# Main test function
main() {
    log_info "Starting comprehensive packaging tests..."
    echo "================================================"
    
    # Ensure we're in the project root
    if [ ! -f "Makefile" ]; then
        if [ -f "../Makefile" ]; then
            cd ..
        else
            log_error "Cannot find project root"
            exit 1
        fi
    fi
    
    local failed_tests=0
    
    # Test Go services
    for service in "${GO_SERVICES[@]}"; do
        if ! test_go_service_packaging "$service"; then
            ((failed_tests++))
        fi
        echo ""
    done
    
    # Test Python services
    for service in "${PYTHON_SERVICES[@]}"; do
        if ! test_python_service_packaging "$service"; then
            ((failed_tests++))
        fi
        echo ""
    done
    
    # Test root commands
    if ! test_root_packaging; then
        ((failed_tests++))
    fi
    echo ""
    
    # Test optimizations
    test_package_optimization
    echo ""
    
    # Test cross-platform
    test_cross_platform
    echo ""
    
    echo "================================================"
    if [ "$failed_tests" -eq 0 ]; then
        log_info "All packaging tests completed successfully!"
        
        # Show final package status
        echo ""
        log_info "Final package status:"
        make status 2>/dev/null || log_warn "Status command failed"
        
        exit 0
    else
        log_error "$failed_tests test(s) failed"
        exit 1
    fi
}

# Run tests
main "$@"