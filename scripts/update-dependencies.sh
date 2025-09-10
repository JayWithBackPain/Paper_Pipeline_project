#!/bin/bash

# Dependency Update Script
# Updates Go and Python dependencies to latest versions

set -e

echo "🔄 Updating project dependencies..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed. Please install Go 1.23+ first."
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
REQUIRED_GO_VERSION="1.23"

if [ "$(printf '%s\n' "$REQUIRED_GO_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_GO_VERSION" ]; then
    print_warning "Go version $GO_VERSION detected. Recommended: $REQUIRED_GO_VERSION+"
fi

# Check if Python is installed
if ! command -v python3 &> /dev/null; then
    print_error "Python 3 is not installed. Please install Python 3.11+ first."
    exit 1
fi

# Update Go dependencies
echo "📦 Updating Go dependencies..."

GO_SERVICES=("data-collector" "batch-processor" "vector-coordinator")

for service in "${GO_SERVICES[@]}"; do
    echo "  Updating $service..."
    cd "go-services/$service"
    
    # Update go.mod to latest Go version
    go mod edit -go=1.23
    
    # Update dependencies
    go get -u github.com/aws/aws-lambda-go@latest
    go get -u github.com/aws/aws-sdk-go@latest
    go get -u github.com/google/uuid@latest
    go get -u github.com/stretchr/testify@latest
    go get -u gopkg.in/yaml.v3@latest
    
    # Tidy up
    go mod tidy
    
    cd ../..
    print_status "Updated $service"
done

# Update shared module
echo "  Updating shared module..."
cd "go-services/shared"
go mod edit -go=1.23
go mod tidy
cd ../..
print_status "Updated shared module"

# Update Python dependencies
echo "🐍 Updating Python dependencies..."

cd "python-services/embedding-api"

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    print_warning "Virtual environment not found. Creating one..."
    python3 -m venv venv
fi

# Activate virtual environment
source venv/bin/activate

# Update pip first
pip install --upgrade pip

# Install latest versions of dependencies
pip install --upgrade \
    transformers \
    sentence-transformers \
    torch \
    numpy \
    boto3 \
    requests \
    pytest \
    psutil \
    flask \
    werkzeug

# Generate updated requirements.txt
pip freeze > requirements_new.txt

# Update requirements.txt with specific versions we want to maintain
cat > requirements.txt << EOF
transformers>=4.45.0
sentence-transformers>=3.1.1
torch>=2.5.0
numpy>=2.1.0
boto3>=1.35.0
requests>=2.32.0
pytest>=8.3.0
psutil>=6.1.0
flask>=3.0.0
werkzeug>=3.0.0
EOF

deactivate
cd ../..

print_status "Updated Python dependencies"

# Run tests to ensure everything still works
echo "🧪 Running tests to verify updates..."

# Test Go services
for service in "${GO_SERVICES[@]}"; do
    echo "  Testing $service..."
    cd "go-services/$service"
    if go test ./... > /dev/null 2>&1; then
        print_status "$service tests passed"
    else
        print_warning "$service tests failed - please check manually"
    fi
    cd ../..
done

# Test Python service
echo "  Testing embedding-api..."
cd "python-services/embedding-api"
source venv/bin/activate
if python -m pytest > /dev/null 2>&1; then
    print_status "embedding-api tests passed"
else
    print_warning "embedding-api tests failed - please check manually"
fi
deactivate
cd ../..

echo ""
print_status "Dependency update completed!"
echo ""
echo "📋 Summary of updates:"
echo "  • Go version: 1.23"
echo "  • AWS Lambda Go: latest"
echo "  • AWS SDK Go: latest"
echo "  • Python dependencies: latest compatible versions"
echo ""
echo "🔍 Next steps:"
echo "  1. Review the changes with 'git diff'"
echo "  2. Test your application thoroughly"
echo "  3. Update any version-specific code if needed"
echo "  4. Commit the changes"