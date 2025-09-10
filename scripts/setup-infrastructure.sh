#!/bin/bash

# Infrastructure Setup Script
# Creates DynamoDB tables, S3 buckets, and other AWS resources

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

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_setup() {
    echo -e "${BLUE}[SETUP]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v aws &> /dev/null; then
        log_error "AWS CLI not found. Please install AWS CLI."
        exit 1
    fi
    
    if ! aws sts get-caller-identity &> /dev/null; then
        log_error "AWS credentials not configured. Please run 'aws configure'."
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

# Create DynamoDB Papers table
create_papers_table() {
    local table_name="${PROJECT_NAME}-papers-${ENVIRONMENT}"
    
    log_setup "Creating DynamoDB Papers table: $table_name"
    
    if aws dynamodb describe-table --table-name "$table_name" &> /dev/null; then
        log_info "Table $table_name already exists"
        return
    fi
    
    aws dynamodb create-table \
        --table-name "$table_name" \
        --attribute-definitions \
            AttributeName=paper_id,AttributeType=S \
            AttributeName=source,AttributeType=S \
            AttributeName=published_date,AttributeType=S \
            AttributeName=trace_id,AttributeType=S \
            AttributeName=batch_timestamp,AttributeType=S \
        --key-schema \
            AttributeName=paper_id,KeyType=HASH \
        --global-secondary-indexes \
            'IndexName=source-published-date-index,KeySchema=[{AttributeName=source,KeyType=HASH},{AttributeName=published_date,KeyType=RANGE}],Projection={ProjectionType=ALL},ProvisionedThroughput={ReadCapacityUnits=5,WriteCapacityUnits=5}' \
            'IndexName=trace-id-batch-timestamp-index,KeySchema=[{AttributeName=trace_id,KeyType=HASH},{AttributeName=batch_timestamp,KeyType=RANGE}],Projection={ProjectionType=ALL},ProvisionedThroughput={ReadCapacityUnits=5,WriteCapacityUnits=5}' \
        --provisioned-throughput \
            ReadCapacityUnits=10,WriteCapacityUnits=10 \
        --region "$AWS_REGION"
    
    log_info "Waiting for table to be active..."
    aws dynamodb wait table-exists --table-name "$table_name" --region "$AWS_REGION"
    
    log_info "Papers table created successfully"
}

# Create DynamoDB Vectors table
create_vectors_table() {
    local table_name="${PROJECT_NAME}-vectors-${ENVIRONMENT}"
    
    log_setup "Creating DynamoDB Vectors table: $table_name"
    
    if aws dynamodb describe-table --table-name "$table_name" &> /dev/null; then
        log_info "Table $table_name already exists"
        return
    fi
    
    aws dynamodb create-table \
        --table-name "$table_name" \
        --attribute-definitions \
            AttributeName=paper_id,AttributeType=S \
            AttributeName=vector_type,AttributeType=S \
            AttributeName=created_at,AttributeType=S \
            AttributeName=model_version,AttributeType=S \
        --key-schema \
            AttributeName=paper_id,KeyType=HASH \
            AttributeName=vector_type,KeyType=RANGE \
        --global-secondary-indexes \
            'IndexName=vector-type-created-at-index,KeySchema=[{AttributeName=vector_type,KeyType=HASH},{AttributeName=created_at,KeyType=RANGE}],Projection={ProjectionType=ALL},ProvisionedThroughput={ReadCapacityUnits=5,WriteCapacityUnits=5}' \
            'IndexName=model-version-paper-id-index,KeySchema=[{AttributeName=model_version,KeyType=HASH},{AttributeName=paper_id,KeyType=RANGE}],Projection={ProjectionType=ALL},ProvisionedThroughput={ReadCapacityUnits=5,WriteCapacityUnits=5}' \
        --provisioned-throughput \
            ReadCapacityUnits=10,WriteCapacityUnits=10 \
        --region "$AWS_REGION"
    
    log_info "Waiting for table to be active..."
    aws dynamodb wait table-exists --table-name "$table_name" --region "$AWS_REGION"
    
    log_info "Vectors table created successfully"
}

# Create S3 buckets
create_s3_buckets() {
    log_setup "Creating S3 buckets..."
    
    local buckets=(
        "${PROJECT_NAME}-raw-data-${ENVIRONMENT}"
        "${PROJECT_NAME}-config-${ENVIRONMENT}"
        "${PROJECT_NAME}-deployment-${ENVIRONMENT}"
    )
    
    for bucket in "${buckets[@]}"; do
        log_info "Creating S3 bucket: $bucket"
        
        if aws s3 ls "s3://$bucket" &> /dev/null; then
            log_info "Bucket $bucket already exists"
            continue
        fi
        
        if [ "$AWS_REGION" = "us-east-1" ]; then
            aws s3 mb "s3://$bucket"
        else
            aws s3 mb "s3://$bucket" --region "$AWS_REGION"
        fi
        
        # Enable versioning for deployment bucket
        if [[ "$bucket" == *"deployment"* ]]; then
            aws s3api put-bucket-versioning \
                --bucket "$bucket" \
                --versioning-configuration Status=Enabled
        fi
        
        # Set lifecycle policy for raw data bucket
        if [[ "$bucket" == *"raw-data"* ]]; then
            local lifecycle_policy='{
                "Rules": [
                    {
                        "ID": "ArchiveOldData",
                        "Status": "Enabled",
                        "Filter": {
                            "Prefix": "raw-data/"
                        },
                        "Transitions": [
                            {
                                "Days": 30,
                                "StorageClass": "STANDARD_IA"
                            },
                            {
                                "Days": 90,
                                "StorageClass": "GLACIER"
                            }
                        ]
                    }
                ]
            }'
            
            echo "$lifecycle_policy" > /tmp/lifecycle-policy.json
            aws s3api put-bucket-lifecycle-configuration \
                --bucket "$bucket" \
                --lifecycle-configuration file:///tmp/lifecycle-policy.json
            rm -f /tmp/lifecycle-policy.json
        fi
        
        log_info "Bucket $bucket created successfully"
    done
}

# Upload configuration files
upload_config_files() {
    log_setup "Uploading configuration files..."
    
    local config_bucket="${PROJECT_NAME}-config-${ENVIRONMENT}"
    
    if [ -f "config/pipeline-config.yaml" ]; then
        log_info "Uploading pipeline configuration..."
        aws s3 cp config/pipeline-config.yaml "s3://$config_bucket/pipeline-config.yaml"
        log_info "Configuration uploaded successfully"
    else
        log_warn "Pipeline configuration file not found"
    fi
}

# Create CloudWatch Log Groups
create_log_groups() {
    log_setup "Creating CloudWatch Log Groups..."
    
    local services=("data-collector" "batch-processor" "vector-coordinator" "embedding-api")
    
    for service in "${services[@]}"; do
        local log_group="/aws/lambda/${PROJECT_NAME}-${service}-${ENVIRONMENT}"
        
        log_info "Creating log group: $log_group"
        
        if aws logs describe-log-groups --log-group-name-prefix "$log_group" --region "$AWS_REGION" | grep -q "$log_group"; then
            log_info "Log group $log_group already exists"
        else
            aws logs create-log-group --log-group-name "$log_group" --region "$AWS_REGION"
            
            # Set retention policy (30 days)
            aws logs put-retention-policy \
                --log-group-name "$log_group" \
                --retention-in-days 30 \
                --region "$AWS_REGION"
            
            log_info "Log group $log_group created successfully"
        fi
    done
}

# Create Step Function state machine
create_step_function() {
    log_setup "Creating Step Function state machine..."
    
    local state_machine_name="${PROJECT_NAME}-orchestrator-${ENVIRONMENT}"
    local role_name="${PROJECT_NAME}-stepfunction-role-${ENVIRONMENT}"
    
    # Create execution role for Step Function
    log_info "Creating Step Function execution role..."
    
    local trust_policy='{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Service": "states.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
            }
        ]
    }'
    
    if ! aws iam get-role --role-name "$role_name" &> /dev/null; then
        aws iam create-role \
            --role-name "$role_name" \
            --assume-role-policy-document "$trust_policy" \
            --description "Execution role for Pipeline API DynamoDB Step Function"
        
        # Attach Lambda invoke policy
        aws iam attach-role-policy \
            --role-name "$role_name" \
            --policy-arn "arn:aws:iam::aws:policy/service-role/AWSLambdaRole"
        
        log_info "Step Function role created"
    else
        log_info "Step Function role already exists"
    fi
    
    # Get account ID and role ARN
    local account_id=$(aws sts get-caller-identity --query Account --output text)
    local role_arn="arn:aws:iam::${account_id}:role/${role_name}"
    
    # Define state machine
    local definition='{
        "Comment": "Pipeline API DynamoDB Orchestrator",
        "StartAt": "BatchProcessor",
        "States": {
            "BatchProcessor": {
                "Type": "Task",
                "Resource": "arn:aws:lambda:'$AWS_REGION':'$account_id':function:'$PROJECT_NAME'-batch-processor-'$ENVIRONMENT'",
                "Next": "VectorCoordinator",
                "Retry": [
                    {
                        "ErrorEquals": ["States.TaskFailed"],
                        "IntervalSeconds": 30,
                        "MaxAttempts": 3,
                        "BackoffRate": 2.0
                    }
                ],
                "Catch": [
                    {
                        "ErrorEquals": ["States.ALL"],
                        "Next": "ProcessingFailed"
                    }
                ]
            },
            "VectorCoordinator": {
                "Type": "Task",
                "Resource": "arn:aws:lambda:'$AWS_REGION':'$account_id':function:'$PROJECT_NAME'-vector-coordinator-'$ENVIRONMENT'",
                "End": true,
                "Retry": [
                    {
                        "ErrorEquals": ["States.TaskFailed"],
                        "IntervalSeconds": 30,
                        "MaxAttempts": 3,
                        "BackoffRate": 2.0
                    }
                ],
                "Catch": [
                    {
                        "ErrorEquals": ["States.ALL"],
                        "Next": "ProcessingFailed"
                    }
                ]
            },
            "ProcessingFailed": {
                "Type": "Fail",
                "Cause": "Processing pipeline failed"
            }
        }
    }'
    
    # Check if state machine exists
    if aws stepfunctions describe-state-machine --state-machine-arn "arn:aws:states:${AWS_REGION}:${account_id}:stateMachine:${state_machine_name}" &> /dev/null; then
        log_info "Step Function state machine already exists"
    else
        log_info "Creating Step Function state machine..."
        
        # Wait for role to be available
        sleep 10
        
        aws stepfunctions create-state-machine \
            --name "$state_machine_name" \
            --definition "$definition" \
            --role-arn "$role_arn" \
            --region "$AWS_REGION"
        
        log_info "Step Function state machine created successfully"
    fi
}

# Show infrastructure summary
show_infrastructure_summary() {
    log_info "Infrastructure Setup Summary"
    echo "============================"
    echo "Environment: $ENVIRONMENT"
    echo "Region: $AWS_REGION"
    echo ""
    echo "Created Resources:"
    echo "  DynamoDB Tables:"
    echo "    - ${PROJECT_NAME}-papers-${ENVIRONMENT}"
    echo "    - ${PROJECT_NAME}-vectors-${ENVIRONMENT}"
    echo ""
    echo "  S3 Buckets:"
    echo "    - ${PROJECT_NAME}-raw-data-${ENVIRONMENT}"
    echo "    - ${PROJECT_NAME}-config-${ENVIRONMENT}"
    echo "    - ${PROJECT_NAME}-deployment-${ENVIRONMENT}"
    echo ""
    echo "  CloudWatch Log Groups:"
    echo "    - /aws/lambda/${PROJECT_NAME}-data-collector-${ENVIRONMENT}"
    echo "    - /aws/lambda/${PROJECT_NAME}-batch-processor-${ENVIRONMENT}"
    echo "    - /aws/lambda/${PROJECT_NAME}-vector-coordinator-${ENVIRONMENT}"
    echo "    - /aws/lambda/${PROJECT_NAME}-embedding-api-${ENVIRONMENT}"
    echo ""
    echo "  Step Function:"
    echo "    - ${PROJECT_NAME}-orchestrator-${ENVIRONMENT}"
    echo ""
    echo "Next Steps:"
    echo "1. Deploy Lambda functions using ./scripts/deploy-aws.sh"
    echo "2. Test the pipeline with sample data"
    echo "3. Configure monitoring and alerts"
}

# Main function
main() {
    log_info "Starting infrastructure setup..."
    echo "================================="
    
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
                echo "  -e, --environment ENV    Environment (default: dev)"
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
    
    log_info "Infrastructure configuration:"
    echo "  Environment: $ENVIRONMENT"
    echo "  Region: $AWS_REGION"
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
    
    # Run setup steps
    check_prerequisites
    create_papers_table
    create_vectors_table
    create_s3_buckets
    upload_config_files
    create_log_groups
    create_step_function
    show_infrastructure_summary
    
    echo "================================="
    log_info "Infrastructure setup completed successfully!"
}

# Run main function
main "$@"