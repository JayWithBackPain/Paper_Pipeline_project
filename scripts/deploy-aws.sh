#!/bin/bash

# AWS Lambda Deployment Script
# Deploys all services to AWS Lambda using AWS CLI

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
AWS_REGION=${AWS_REGION:-us-east-1}
ENVIRONMENT=${ENVIRONMENT:-dev}
PROJECT_NAME="pipeline-api-dynamodb"

# Service configurations
GO_SERVICES="data-collector batch-processor vector-coordinator"
PYTHON_SERVICES="embedding-api"

# Memory configurations (in MB)
get_memory_size() {
    case $1 in
        "data-collector") echo "128" ;;
        "batch-processor") echo "256" ;;
        "vector-coordinator") echo "256" ;;
        "embedding-api") echo "512" ;;
        *) echo "256" ;;  # default
    esac
}

# S3 bucket for deployment artifacts
S3_BUCKET="${PROJECT_NAME}-deployment-${ENVIRONMENT}"
S3_PREFIX="lambda-packages"

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

log_deploy() {
    echo -e "${BLUE}[DEPLOY]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check AWS CLI
    if ! command -v aws &> /dev/null; then
        log_error "AWS CLI not found. Please install AWS CLI."
        exit 1
    fi
    
    # Check AWS credentials
    if ! aws sts get-caller-identity &> /dev/null; then
        log_error "AWS credentials not configured. Please run 'aws configure'."
        exit 1
    fi
    
    # Check if packages exist
    local missing_packages=()
    for service in $GO_SERVICES; do
        if [ ! -f "go-services/$service/$service.zip" ]; then
            missing_packages+=("$service")
        fi
    done
    
    for service in $PYTHON_SERVICES; do
        if [ ! -f "python-services/$service/$service.zip" ]; then
            missing_packages+=("$service")
        fi
    done
    
    if [ ${#missing_packages[@]} -gt 0 ]; then
        log_error "Missing deployment packages: ${missing_packages[*]}"
        log_info "Run 'make package-all' to create packages"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

# Create S3 bucket for deployment artifacts
create_s3_bucket() {
    log_info "Setting up S3 bucket for deployment artifacts..."
    
    if aws s3 ls "s3://$S3_BUCKET" &> /dev/null; then
        log_info "S3 bucket $S3_BUCKET already exists"
    else
        log_info "Creating S3 bucket: $S3_BUCKET"
        
        if [ "$AWS_REGION" = "us-east-1" ]; then
            aws s3 mb "s3://$S3_BUCKET"
        else
            aws s3 mb "s3://$S3_BUCKET" --region "$AWS_REGION"
        fi
        
        # Enable versioning
        aws s3api put-bucket-versioning \
            --bucket "$S3_BUCKET" \
            --versioning-configuration Status=Enabled
        
        log_info "S3 bucket created successfully"
    fi
}

# Upload package to S3
upload_package() {
    local service=$1
    local package_path=$2
    local s3_key="$S3_PREFIX/$service/$service-$(date +%Y%m%d-%H%M%S).zip"
    
    log_info "Uploading $service package to S3..."
    aws s3 cp "$package_path" "s3://$S3_BUCKET/$s3_key"
    
    echo "$s3_key"
}

# Create or update Lambda function
deploy_lambda_function() {
    local service=$1
    local memory_size=$2
    local package_path=$3
    local function_name="${PROJECT_NAME}-${service}-${ENVIRONMENT}"
    
    log_deploy "Deploying Lambda function: $function_name"
    
    # Upload package to S3
    local s3_key=$(upload_package "$service" "$package_path")
    
    # Check if function exists
    if aws lambda get-function --function-name "$function_name" &> /dev/null; then
        log_info "Updating existing function: $function_name"
        
        # Update function code
        aws lambda update-function-code \
            --function-name "$function_name" \
            --s3-bucket "$S3_BUCKET" \
            --s3-key "$s3_key"
        
        # Update function configuration
        aws lambda update-function-configuration \
            --function-name "$function_name" \
            --memory-size "$memory_size" \
            --timeout 300 \
            --environment Variables="{ENVIRONMENT=$ENVIRONMENT,LOG_LEVEL=INFO}"
        
    else
        log_info "Creating new function: $function_name"
        
        # Create execution role if it doesn't exist
        local role_name="${PROJECT_NAME}-lambda-role-${ENVIRONMENT}"
        local role_arn=$(create_lambda_execution_role "$role_name")
        
        # Wait for role to be available
        sleep 10
        
        # Determine runtime based on service type
        local runtime="go1.x"
        local handler="main"
        
        if [[ " $PYTHON_SERVICES " =~ " $service " ]]; then
            runtime="python3.9"
            handler="main.lambda_handler"
        fi
        
        # Create function
        aws lambda create-function \
            --function-name "$function_name" \
            --runtime "$runtime" \
            --role "$role_arn" \
            --handler "$handler" \
            --code S3Bucket="$S3_BUCKET",S3Key="$s3_key" \
            --memory-size "$memory_size" \
            --timeout 300 \
            --environment Variables="{ENVIRONMENT=$ENVIRONMENT,LOG_LEVEL=INFO}" \
            --description "Pipeline API DynamoDB - $service service"
    fi
    
    # Wait for function to be active
    log_info "Waiting for function to be active..."
    aws lambda wait function-active --function-name "$function_name"
    
    log_info "Function $function_name deployed successfully"
}

# Create Lambda execution role
create_lambda_execution_role() {
    local role_name=$1
    
    # Check if role exists
    if aws iam get-role --role-name "$role_name" &> /dev/null; then
        log_info "IAM role $role_name already exists"
        aws iam get-role --role-name "$role_name" --query 'Role.Arn' --output text
        return
    fi
    
    log_info "Creating IAM role: $role_name"
    
    # Trust policy for Lambda
    local trust_policy='{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Service": "lambda.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
            }
        ]
    }'
    
    # Create role
    aws iam create-role \
        --role-name "$role_name" \
        --assume-role-policy-document "$trust_policy" \
        --description "Execution role for Pipeline API DynamoDB Lambda functions"
    
    # Attach basic execution policy
    aws iam attach-role-policy \
        --role-name "$role_name" \
        --policy-arn "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
    
    # Create and attach custom policy for DynamoDB and S3 access
    local policy_document='{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "dynamodb:PutItem",
                    "dynamodb:GetItem",
                    "dynamodb:UpdateItem",
                    "dynamodb:DeleteItem",
                    "dynamodb:Query",
                    "dynamodb:Scan",
                    "dynamodb:BatchWriteItem",
                    "dynamodb:BatchGetItem"
                ],
                "Resource": [
                    "arn:aws:dynamodb:*:*:table/pipeline-*",
                    "arn:aws:dynamodb:*:*:table/pipeline-*/index/*"
                ]
            },
            {
                "Effect": "Allow",
                "Action": [
                    "s3:GetObject",
                    "s3:PutObject",
                    "s3:DeleteObject"
                ],
                "Resource": [
                    "arn:aws:s3:::pipeline-*/*"
                ]
            },
            {
                "Effect": "Allow",
                "Action": [
                    "s3:ListBucket"
                ],
                "Resource": [
                    "arn:aws:s3:::pipeline-*"
                ]
            },
            {
                "Effect": "Allow",
                "Action": [
                    "logs:CreateLogGroup",
                    "logs:CreateLogStream",
                    "logs:PutLogEvents"
                ],
                "Resource": "arn:aws:logs:*:*:*"
            }
        ]
    }'
    
    local policy_name="${PROJECT_NAME}-lambda-policy-${ENVIRONMENT}"
    
    # Create policy
    aws iam create-policy \
        --policy-name "$policy_name" \
        --policy-document "$policy_document" \
        --description "Custom policy for Pipeline API DynamoDB Lambda functions" \
        &> /dev/null || true
    
    # Get account ID
    local account_id=$(aws sts get-caller-identity --query Account --output text)
    
    # Attach custom policy
    aws iam attach-role-policy \
        --role-name "$role_name" \
        --policy-arn "arn:aws:iam::${account_id}:policy/${policy_name}"
    
    # Return role ARN
    aws iam get-role --role-name "$role_name" --query 'Role.Arn' --output text
}

# Deploy Go services
deploy_go_services() {
    log_info "Deploying Go services..."
    
    for service in $GO_SERVICES; do
        local memory_size=$(get_memory_size "$service")
        local package_path="go-services/$service/$service.zip"
        
        deploy_lambda_function "$service" "$memory_size" "$package_path"
        echo ""
    done
}

# Deploy Python services
deploy_python_services() {
    log_info "Deploying Python services..."
    
    for service in $PYTHON_SERVICES; do
        local memory_size=$(get_memory_size "$service")
        local package_path="python-services/$service/$service.zip"
        
        deploy_lambda_function "$service" "$memory_size" "$package_path"
        echo ""
    done
}

# Create S3 event trigger for batch-processor
setup_s3_event_trigger() {
    log_info "Setting up S3 event trigger for batch-processor..."
    
    local function_name="${PROJECT_NAME}-batch-processor-${ENVIRONMENT}"
    local bucket_name="${PROJECT_NAME}-raw-data-${ENVIRONMENT}"
    
    # Create raw data bucket if it doesn't exist
    if ! aws s3 ls "s3://$bucket_name" &> /dev/null; then
        log_info "Creating raw data S3 bucket: $bucket_name"
        
        if [ "$AWS_REGION" = "us-east-1" ]; then
            aws s3 mb "s3://$bucket_name"
        else
            aws s3 mb "s3://$bucket_name" --region "$AWS_REGION"
        fi
    fi
    
    # Add Lambda permission for S3 to invoke the function
    aws lambda add-permission \
        --function-name "$function_name" \
        --principal s3.amazonaws.com \
        --action lambda:InvokeFunction \
        --source-arn "arn:aws:s3:::$bucket_name" \
        --statement-id "s3-trigger-$(date +%s)" \
        &> /dev/null || true
    
    # Create notification configuration
    local notification_config='{
        "LambdaConfigurations": [
            {
                "Id": "batch-processor-trigger",
                "LambdaFunctionArn": "arn:aws:lambda:'$AWS_REGION':'$(aws sts get-caller-identity --query Account --output text)':function:'$function_name'",
                "Events": ["s3:ObjectCreated:*"],
                "Filter": {
                    "Key": {
                        "FilterRules": [
                            {
                                "Name": "prefix",
                                "Value": "raw-data/"
                            },
                            {
                                "Name": "suffix",
                                "Value": ".gz"
                            }
                        ]
                    }
                }
            }
        ]
    }'
    
    # Apply notification configuration
    echo "$notification_config" > /tmp/s3-notification.json
    aws s3api put-bucket-notification-configuration \
        --bucket "$bucket_name" \
        --notification-configuration file:///tmp/s3-notification.json
    
    rm -f /tmp/s3-notification.json
    
    log_info "S3 event trigger configured successfully"
}

# Health check for deployed functions
health_check() {
    log_info "Performing health checks..."
    
    local all_services="$GO_SERVICES $PYTHON_SERVICES"
    
    for service in $all_services; do
        local function_name="${PROJECT_NAME}-${service}-${ENVIRONMENT}"
        
        log_info "Checking function: $function_name"
        
        # Get function configuration
        local function_info=$(aws lambda get-function --function-name "$function_name" 2>/dev/null)
        
        if [ $? -eq 0 ]; then
            local state=$(echo "$function_info" | jq -r '.Configuration.State // "Unknown"')
            local last_modified=$(echo "$function_info" | jq -r '.Configuration.LastModified // "Unknown"')
            
            if [ "$state" = "Active" ]; then
                log_info "✓ $function_name is active (last modified: $last_modified)"
            else
                log_warn "⚠ $function_name state: $state"
            fi
        else
            log_error "✗ $function_name not found or not accessible"
        fi
    done
}

# Show deployment summary
show_deployment_summary() {
    log_info "Deployment Summary"
    echo "=================="
    echo "Environment: $ENVIRONMENT"
    echo "Region: $AWS_REGION"
    echo "S3 Bucket: $S3_BUCKET"
    echo ""
    echo "Deployed Functions:"
    
    local all_services="$GO_SERVICES $PYTHON_SERVICES"
    
    for service in $all_services; do
        local function_name="${PROJECT_NAME}-${service}-${ENVIRONMENT}"
        echo "  - $function_name"
    done
    
    echo ""
    echo "Next Steps:"
    echo "1. Configure DynamoDB tables if not already done"
    echo "2. Set up Step Functions for orchestration"
    echo "3. Configure CloudWatch alarms and monitoring"
    echo "4. Test the deployment with sample data"
}

# Main deployment function
main() {
    log_info "Starting AWS Lambda deployment..."
    echo "=================================="
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --environment|-e)
                ENVIRONMENT="$2"
                shift 2
                ;;
            --region|-r)
                AWS_REGION="$2"
                shift 2
                ;;
            --help|-h)
                echo "Usage: $0 [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  -e, --environment ENV    Deployment environment (default: dev)"
                echo "  -r, --region REGION      AWS region (default: us-east-1)"
                echo "  -h, --help              Show this help message"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    # Update derived variables
    S3_BUCKET="${PROJECT_NAME}-deployment-${ENVIRONMENT}"
    
    log_info "Deployment configuration:"
    echo "  Environment: $ENVIRONMENT"
    echo "  Region: $AWS_REGION"
    echo "  S3 Bucket: $S3_BUCKET"
    echo ""
    
    # Ensure we're in the project root
    if [ ! -f "Makefile" ]; then
        if [ -f "../Makefile" ]; then
            cd ..
        else
            log_error "Cannot find project root"
            exit 1
        fi
    fi
    
    # Run deployment steps
    check_prerequisites
    create_s3_bucket
    deploy_go_services
    deploy_python_services
    setup_s3_event_trigger
    health_check
    show_deployment_summary
    
    echo "=================================="
    log_info "Deployment completed successfully!"
}

# Run main function
main "$@"