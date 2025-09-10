package config

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"gopkg.in/yaml.v3"
)

// Config represents the complete pipeline configuration
type Config struct {
	DataSources   map[string]DataSourceConfig `yaml:"data_sources"`
	AWS           AWSConfig                   `yaml:"aws"`
	Processing    ProcessingConfig            `yaml:"processing"`
	Vectorization VectorizationConfig         `yaml:"vectorization"`
	Logging       LoggingConfig               `yaml:"logging"`
}

// DataSourceConfig represents configuration for a data source
type DataSourceConfig struct {
	APIEndpoint   string            `yaml:"api_endpoint"`
	FieldsMapping map[string]string `yaml:"fields_mapping"`
	RateLimit     int               `yaml:"rate_limit"`
	MaxResults    int               `yaml:"max_results"`
	SearchQuery   string            `yaml:"search_query"`
	DateFrom      string            `yaml:"date_from,omitempty"` // Format: YYYY-MM-DD
	DateTo        string            `yaml:"date_to,omitempty"`   // Format: YYYY-MM-DD
	Enabled       bool              `yaml:"enabled"`
}

// AWSConfig represents AWS service configuration
type AWSConfig struct {
	S3       S3Config       `yaml:"s3"`
	DynamoDB DynamoDBConfig `yaml:"dynamodb"`
	Lambda   LambdaConfig   `yaml:"lambda"`
}

// S3Config represents S3 configuration
type S3Config struct {
	RawDataBucket string `yaml:"raw_data_bucket"`
	ConfigBucket  string `yaml:"config_bucket"`
	RawDataPrefix string `yaml:"raw_data_prefix"`
}

// DynamoDBConfig represents DynamoDB configuration
type DynamoDBConfig struct {
	PapersTable  string `yaml:"papers_table"`
	VectorsTable string `yaml:"vectors_table"`
	Region       string `yaml:"region"`
}

// LambdaConfig represents Lambda configuration
type LambdaConfig struct {
	Timeout int `yaml:"timeout"`
	Memory  int `yaml:"memory"`
}

// ProcessingConfig represents processing configuration
type ProcessingConfig struct {
	BatchSize     int    `yaml:"batch_size"`
	Compression   string `yaml:"compression"`
	RetryAttempts int    `yaml:"retry_attempts"`
	RetryDelay    int    `yaml:"retry_delay"`
}

// VectorizationConfig represents vectorization configuration
type VectorizationConfig struct {
	ModelName     string   `yaml:"model_name"`
	VectorDim     int      `yaml:"vector_dimension"`
	BatchSize     int      `yaml:"batch_size"`
	TextFields    []string `yaml:"text_fields"`
	MaxTextLength int      `yaml:"max_text_length"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level          string `yaml:"level"`
	Structured     bool   `yaml:"structured"`
	IncludeTraceID bool   `yaml:"include_trace_id"`
}

// Manager handles configuration loading and management
type Manager struct {
	s3Client *s3.S3
}

// NewManager creates a new configuration manager
func NewManager() (*Manager, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"), // Default region
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &Manager{
		s3Client: s3.New(sess),
	}, nil
}

// LoadFromS3 loads configuration from S3
func (m *Manager) LoadFromS3(ctx context.Context, bucket, key string) (*Config, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := m.s3Client.GetObjectWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get config from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read config data: %w", err)
	}

	return m.parseConfig(data)
}

// LoadFromFile loads configuration from local file (for testing)
func (m *Manager) LoadFromFile(filePath string) (*Config, error) {
	// This would be used for local development/testing
	// Implementation would read from local file system
	return nil, fmt.Errorf("local file loading not implemented in this version")
}

// LoadFromBytes loads configuration from byte data
func (m *Manager) LoadFromBytes(data []byte) (*Config, error) {
	return m.parseConfig(data)
}

// parseConfig parses YAML configuration data
func (m *Manager) parseConfig(data []byte) (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Set default values for enabled flag if not specified
	for name, source := range config.DataSources {
		if name != "semantic_scholar" { // semantic_scholar is disabled by default
			source.Enabled = true
			config.DataSources[name] = source
		}
	}

	return &config, nil
}

// GetDataSourceConfig returns configuration for a specific data source
func (c *Config) GetDataSourceConfig(sourceName string) (DataSourceConfig, error) {
	source, exists := c.DataSources[sourceName]
	if !exists {
		return DataSourceConfig{}, fmt.Errorf("data source '%s' not found in configuration", sourceName)
	}

	if !source.Enabled {
		return DataSourceConfig{}, fmt.Errorf("data source '%s' is disabled", sourceName)
	}

	return source, nil
}

// GetDefaultConfig returns a default configuration for fallback scenarios
func GetDefaultConfig() *Config {
	return &Config{
		DataSources: map[string]DataSourceConfig{
			"arxiv": {
				APIEndpoint: "http://export.arxiv.org/api/query",
				FieldsMapping: map[string]string{
					"id":         "id",
					"title":      "title",
					"abstract":   "summary",
					"authors":    "author",
					"published":  "published",
					"categories": "category",
				},
				RateLimit:   3,
				MaxResults:  1000,
				SearchQuery: "cat:cs.AI OR cat:cs.LG OR cat:cs.CL",
				Enabled:     true,
			},
		},
		AWS: AWSConfig{
			S3: S3Config{
				RawDataBucket: "pipeline-raw-data",
				ConfigBucket:  "pipeline-config",
				RawDataPrefix: "raw-data",
			},
			DynamoDB: DynamoDBConfig{
				PapersTable:  "Papers",
				VectorsTable: "Vectors",
				Region:       "us-east-1",
			},
			Lambda: LambdaConfig{
				Timeout: 900,
				Memory:  1024,
			},
		},
		Processing: ProcessingConfig{
			BatchSize:     25,
			Compression:   "gzip",
			RetryAttempts: 3,
			RetryDelay:    1,
		},
		Vectorization: VectorizationConfig{
			ModelName:     "sentence-transformers/all-MiniLM-L6-v2",
			VectorDim:     384,
			BatchSize:     10,
			TextFields:    []string{"title", "abstract"},
			MaxTextLength: 1024,
		},
		Logging: LoggingConfig{
			Level:          "INFO",
			Structured:     true,
			IncludeTraceID: true,
		},
	}
}
