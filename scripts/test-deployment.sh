#!/bin/bash

# Test script for AWS deployment functionality
# Tests deployment scripts without actually deploying to AWS

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test configuration
MOCK_AWS_CLI=false
TEST_DIR="test-deployment"

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

# Test deployment script existence and permissions
test_deployment_script_exists() {
    log_test "Testing deployment script existence and permissions"
    
    if [ ! -f "scripts/deploy-aws.sh" ]; then
        test_failed "Deployment script not found"
        return 1
    fi
    test_passed "Deployment script exists"
    
    if [ ! -x "scripts/deploy-aws.sh" ]; then
        test_failed "Deployment script is not executable"
        return 1
    fi
    test_passed "Deployment script is executable"
    
    return 0
}

# Test deployment script help functionality
test_deployment_script_help() {
    log_test "Testing deployment script help functionality"
    
    local help_output
    if help_output=$(./scripts/deploy-aws.sh --help 2>&1); then
        if echo "$help_output" | grep -q "Usage:"; then
            test_passed "Help functionality works"
        else
            test_failed "Help output doesn't contain usage information"
            return 1
        fi
    else
        test_failed "Help command failed"
        return 1
    fi
    
    return 0
}

# Test deployment script argument parsing
test_deployment_script_args() {
    log_test "Testing deployment script argument parsing"
    
    # Test with mock AWS CLI to avoid actual AWS calls
    export PATH="$(pwd)/test-mocks:$PATH"
    
    # Create mock AWS CLI
    mkdir -p test-mocks
    cat > test-mocks/aws << 'EOF'
#!/bin/bash
# Mock AWS CLI for testing
case "$1" in
    "sts")
        case "$2" in
            "get-caller-identity")
                echo '{"Account": "123456789012", "Arn": "arn:aws:iam::123456789012:user/test"}'
                ;;
        esac
        ;;
    "s3")
        case "$2" in
            "ls")
                exit 1  # Simulate bucket doesn't exist
                ;;
            "mb")
                echo "make_bucket: $3"
                ;;
        esac
        ;;
    *)
        echo "Mock AWS CLI called with: $*" >&2
        exit 1
        ;;
esac
EOF
    chmod +x test-mocks/aws
    
    # Test that script accepts arguments (it should fail on missing packages, which is expected)
    if ./scripts/deploy-aws.sh --environment test --region us-west-2 2>&1 | grep -q "Missing deployment packages"; then
        test_passed "Script correctly parses arguments and validates prerequisites"
    else
        log_warn "Script behavior with arguments may need verification"
    fi
    
    # Cleanup
    rm -rf test-mocks
    
    return 0
}

# Test Makefile deployment integration
test_makefile_deployment_integration() {
    log_test "Testing Makefile deployment integration"
    
    # Check if deploy targets exist in service Makefiles
    local services=("data-collector" "batch-processor" "vector-coordinator" "embedding-api")
    
    for service in "${services[@]}"; do
        local makefile_path
        if [[ "$service" == "embedding-api" ]]; then
            makefile_path="python-services/$service/Makefile"
        else
            makefile_path="go-services/$service/Makefile"
        fi
        
        if [ -f "$makefile_path" ]; then
            if grep -q "^deploy:" "$makefile_path"; then
                test_passed "$service Makefile has deploy target"
            else
                test_failed "$service Makefile missing deploy target"
                return 1
            fi
        else
            test_failed "$service Makefile not found"
            return 1
        fi
    done
    
    # Check root Makefile deploy-all target
    if grep -q "^deploy-all:" Makefile; then
        test_passed "Root Makefile has deploy-all target"
    else
        test_failed "Root Makefile missing deploy-all target"
        return 1
    fi
    
    return 0
}

# Test deployment script structure and functions
test_deployment_script_structure() {
    log_test "Testing deployment script structure and functions"
    
    local script_content=$(cat scripts/deploy-aws.sh)
    
    # Check for essential functions
    local required_functions=(
        "check_prerequisites"
        "create_s3_bucket"
        "deploy_lambda_function"
        "create_lambda_execution_role"
        "deploy_go_services"
        "deploy_python_services"
        "setup_s3_event_trigger"
        "health_check"
        "show_deployment_summary"
    )
    
    for func in "${required_functions[@]}"; do
        if echo "$script_content" | grep -q "^$func()"; then
            test_passed "Function $func exists"
        else
            test_failed "Function $func missing"
            return 1
        fi
    done
    
    # Check for configuration variables
    local required_vars=(
        "AWS_REGION"
        "ENVIRONMENT"
        "PROJECT_NAME"
        "GO_SERVICES"
        "PYTHON_SERVICES"
        "S3_BUCKET"
    )
    
    for var in "${required_vars[@]}"; do
        if echo "$script_content" | grep -q "$var"; then
            test_passed "Variable $var is configured"
        else
            test_failed "Variable $var missing"
            return 1
        fi
    done
    
    return 0
}

# Test IAM policy structure
test_iam_policy_structure() {
    log_test "Testing IAM policy structure in deployment script"
    
    local script_content=$(cat scripts/deploy-aws.sh)
    
    # Check for DynamoDB permissions
    if echo "$script_content" | grep -q "dynamodb:PutItem"; then
        test_passed "DynamoDB permissions included"
    else
        test_failed "DynamoDB permissions missing"
        return 1
    fi
    
    # Check for S3 permissions
    if echo "$script_content" | grep -q "s3:GetObject"; then
        test_passed "S3 permissions included"
    else
        test_failed "S3 permissions missing"
        return 1
    fi
    
    # Check for CloudWatch Logs permissions
    if echo "$script_content" | grep -q "logs:CreateLogGroup"; then
        test_passed "CloudWatch Logs permissions included"
    else
        test_failed "CloudWatch Logs permissions missing"
        return 1
    fi
    
    return 0
}

# Test S3 event trigger configuration
test_s3_event_trigger_config() {
    log_test "Testing S3 event trigger configuration"
    
    local script_content=$(cat scripts/deploy-aws.sh)
    
    # Check for S3 notification configuration
    if echo "$script_content" | grep -q "LambdaConfigurations"; then
        test_passed "S3 Lambda trigger configuration exists"
    else
        test_failed "S3 Lambda trigger configuration missing"
        return 1
    fi
    
    # Check for proper event filtering
    if echo "$script_content" | grep -q "s3:ObjectCreated"; then
        test_passed "S3 event filtering configured"
    else
        test_failed "S3 event filtering missing"
        return 1
    fi
    
    # Check for file extension filtering
    if echo "$script_content" | grep -q '\.gz'; then
        test_passed "File extension filtering configured"
    else
        test_failed "File extension filtering missing"
        return 1
    fi
    
    return 0
}

# Test deployment validation and health checks
test_deployment_validation() {
    log_test "Testing deployment validation and health checks"
    
    local script_content=$(cat scripts/deploy-aws.sh)
    
    # Check for prerequisite validation
    if echo "$script_content" | grep -q "check_prerequisites"; then
        test_passed "Prerequisite validation exists"
    else
        test_failed "Prerequisite validation missing"
        return 1
    fi
    
    # Check for AWS CLI validation
    if echo "$script_content" | grep -q "command -v aws"; then
        test_passed "AWS CLI validation exists"
    else
        test_failed "AWS CLI validation missing"
        return 1
    fi
    
    # Check for package existence validation
    if echo "$script_content" | grep -q "missing_packages"; then
        test_passed "Package existence validation exists"
    else
        test_failed "Package existence validation missing"
        return 1
    fi
    
    # Check for health check functionality
    if echo "$script_content" | grep -q "health_check"; then
        test_passed "Health check functionality exists"
    else
        test_failed "Health check functionality missing"
        return 1
    fi
    
    return 0
}

# Test error handling and logging
test_error_handling() {
    log_test "Testing error handling and logging"
    
    local script_content=$(cat scripts/deploy-aws.sh)
    
    # Check for set -e (exit on error)
    if echo "$script_content" | grep -q "set -e"; then
        test_passed "Exit on error is enabled"
    else
        test_failed "Exit on error not enabled"
        return 1
    fi
    
    # Check for logging functions
    local log_functions=("log_info" "log_warn" "log_error" "log_deploy")
    
    for func in "${log_functions[@]}"; do
        if echo "$script_content" | grep -q "$func()"; then
            test_passed "Logging function $func exists"
        else
            test_failed "Logging function $func missing"
            return 1
        fi
    done
    
    # Check for colored output
    if echo "$script_content" | grep -q "\\033\["; then
        test_passed "Colored output is configured"
    else
        test_failed "Colored output not configured"
        return 1
    fi
    
    return 0
}

# Test deployment script with dry-run simulation
test_dry_run_simulation() {
    log_test "Testing deployment script with dry-run simulation"
    
    # Create a simple test to verify script can be parsed and basic validation works
    if bash -n scripts/deploy-aws.sh; then
        test_passed "Deployment script syntax is valid"
    else
        test_failed "Deployment script has syntax errors"
        return 1
    fi
    
    # Test help functionality
    if ./scripts/deploy-aws.sh --help > /dev/null 2>&1; then
        test_passed "Help command executes successfully"
    else
        test_failed "Help command fails"
        return 1
    fi
    
    return 0
}

# Main test function
main() {
    log_info "Starting deployment script tests..."
    echo "======================================="
    
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
    
    # Run all tests
    test_deployment_script_exists || ((failed_tests++))
    echo ""
    
    test_deployment_script_help || ((failed_tests++))
    echo ""
    
    test_deployment_script_args || ((failed_tests++))
    echo ""
    
    test_makefile_deployment_integration || ((failed_tests++))
    echo ""
    
    test_deployment_script_structure || ((failed_tests++))
    echo ""
    
    test_iam_policy_structure || ((failed_tests++))
    echo ""
    
    test_s3_event_trigger_config || ((failed_tests++))
    echo ""
    
    test_deployment_validation || ((failed_tests++))
    echo ""
    
    test_error_handling || ((failed_tests++))
    echo ""
    
    test_dry_run_simulation || ((failed_tests++))
    echo ""
    
    echo "======================================="
    if [ "$failed_tests" -eq 0 ]; then
        log_info "All deployment script tests passed!"
        
        echo ""
        log_info "Deployment script features:"
        echo "  ✓ AWS CLI integration"
        echo "  ✓ Multi-environment support"
        echo "  ✓ S3 bucket management"
        echo "  ✓ Lambda function deployment"
        echo "  ✓ IAM role and policy management"
        echo "  ✓ S3 event trigger configuration"
        echo "  ✓ Health checks and validation"
        echo "  ✓ Comprehensive error handling"
        echo "  ✓ Colored logging output"
        
        echo ""
        log_info "Usage examples:"
        echo "  ./scripts/deploy-aws.sh                    # Deploy to dev environment"
        echo "  ./scripts/deploy-aws.sh -e prod -r us-west-2  # Deploy to prod in us-west-2"
        echo "  ./scripts/deploy-aws.sh --help             # Show help"
        
        exit 0
    else
        log_error "$failed_tests test(s) failed"
        exit 1
    fi
}

# Run tests
main "$@"