package config

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	yamlData := `
data_sources:
  arxiv:
    api_endpoint: "http://export.arxiv.org/api/query"
    fields_mapping:
      id: "id"
      title: "title"
      abstract: "summary"
    rate_limit: 3
    max_results: 1000
    search_query: "cat:cs.AI"
    enabled: true
  pubmed:
    api_endpoint: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/"
    rate_limit: 10
    enabled: false

aws:
  s3:
    raw_data_bucket: "test-bucket"
    config_bucket: "test-config"
    raw_data_prefix: "raw-data"
  dynamodb:
    papers_table: "Papers"
    vectors_table: "Vectors"
    region: "us-east-1"
  lambda:
    timeout: 900
    memory: 1024

processing:
  batch_size: 25
  compression: "gzip"
  retry_attempts: 3
  retry_delay: 1

vectorization:
  model_name: "sentence-transformers/all-MiniLM-L6-v2"
  vector_dimension: 384
  batch_size: 10
  text_fields: ["title", "abstract"]
  max_text_length: 1024

logging:
  level: "INFO"
  structured: true
  include_trace_id: true
`

	manager := &Manager{}
	config, err := manager.parseConfig([]byte(yamlData))
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Test data sources
	if len(config.DataSources) != 2 {
		t.Errorf("Expected 2 data sources, got %d", len(config.DataSources))
	}

	arxivConfig, exists := config.DataSources["arxiv"]
	if !exists {
		t.Error("arXiv data source not found")
	}

	if arxivConfig.APIEndpoint != "http://export.arxiv.org/api/query" {
		t.Errorf("Expected arXiv API endpoint 'http://export.arxiv.org/api/query', got '%s'", arxivConfig.APIEndpoint)
	}

	if arxivConfig.RateLimit != 3 {
		t.Errorf("Expected arXiv rate limit 3, got %d", arxivConfig.RateLimit)
	}

	if !arxivConfig.Enabled {
		t.Error("Expected arXiv to be enabled")
	}

	// Test AWS config
	if config.AWS.S3.RawDataBucket != "test-bucket" {
		t.Errorf("Expected S3 bucket 'test-bucket', got '%s'", config.AWS.S3.RawDataBucket)
	}

	if config.AWS.DynamoDB.PapersTable != "Papers" {
		t.Errorf("Expected Papers table 'Papers', got '%s'", config.AWS.DynamoDB.PapersTable)
	}

	// Test processing config
	if config.Processing.BatchSize != 25 {
		t.Errorf("Expected batch size 25, got %d", config.Processing.BatchSize)
	}

	// Test vectorization config
	if config.Vectorization.VectorDim != 384 {
		t.Errorf("Expected vector dimension 384, got %d", config.Vectorization.VectorDim)
	}

	if len(config.Vectorization.TextFields) != 2 {
		t.Errorf("Expected 2 text fields, got %d", len(config.Vectorization.TextFields))
	}

	// Test logging config
	if config.Logging.Level != "INFO" {
		t.Errorf("Expected log level 'INFO', got '%s'", config.Logging.Level)
	}

	if !config.Logging.Structured {
		t.Error("Expected structured logging to be enabled")
	}
}

func TestGetDataSourceConfig(t *testing.T) {
	config := &Config{
		DataSources: map[string]DataSourceConfig{
			"arxiv": {
				APIEndpoint: "http://test.com",
				RateLimit:   3,
				Enabled:     true,
			},
			"disabled_source": {
				APIEndpoint: "http://disabled.com",
				RateLimit:   5,
				Enabled:     false,
			},
		},
	}

	// Test getting enabled source
	arxivConfig, err := config.GetDataSourceConfig("arxiv")
	if err != nil {
		t.Fatalf("Failed to get arXiv config: %v", err)
	}

	if arxivConfig.APIEndpoint != "http://test.com" {
		t.Errorf("Expected API endpoint 'http://test.com', got '%s'", arxivConfig.APIEndpoint)
	}

	// Test getting disabled source
	_, err = config.GetDataSourceConfig("disabled_source")
	if err == nil {
		t.Error("Expected error for disabled source, got nil")
	}

	// Test getting non-existent source
	_, err = config.GetDataSourceConfig("non_existent")
	if err == nil {
		t.Error("Expected error for non-existent source, got nil")
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	if config == nil {
		t.Fatal("Default config is nil")
	}

	// Test that arXiv is configured by default
	arxivConfig, exists := config.DataSources["arxiv"]
	if !exists {
		t.Error("arXiv not found in default config")
	}

	if !arxivConfig.Enabled {
		t.Error("arXiv should be enabled by default")
	}

	if arxivConfig.RateLimit != 3 {
		t.Errorf("Expected default rate limit 3, got %d", arxivConfig.RateLimit)
	}

	// Test AWS defaults
	if config.AWS.S3.RawDataBucket != "pipeline-raw-data" {
		t.Errorf("Expected default bucket 'pipeline-raw-data', got '%s'", config.AWS.S3.RawDataBucket)
	}

	// Test processing defaults
	if config.Processing.BatchSize != 25 {
		t.Errorf("Expected default batch size 25, got %d", config.Processing.BatchSize)
	}

	if config.Processing.Compression != "gzip" {
		t.Errorf("Expected default compression 'gzip', got '%s'", config.Processing.Compression)
	}
}

func TestLoadFromBytes(t *testing.T) {
	yamlData := `
data_sources:
  test_source:
    api_endpoint: "http://test.com"
    rate_limit: 5
    enabled: true

aws:
  s3:
    raw_data_bucket: "test-bucket"
`

	manager := &Manager{}
	config, err := manager.LoadFromBytes([]byte(yamlData))
	if err != nil {
		t.Fatalf("Failed to load config from bytes: %v", err)
	}

	if len(config.DataSources) != 1 {
		t.Errorf("Expected 1 data source, got %d", len(config.DataSources))
	}

	testSource, exists := config.DataSources["test_source"]
	if !exists {
		t.Error("test_source not found")
	}

	if testSource.APIEndpoint != "http://test.com" {
		t.Errorf("Expected API endpoint 'http://test.com', got '%s'", testSource.APIEndpoint)
	}
}

func TestInvalidYAML(t *testing.T) {
	invalidYAML := `
invalid yaml content
  - missing proper structure
    bad indentation
`

	manager := &Manager{}
	_, err := manager.LoadFromBytes([]byte(invalidYAML))
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}