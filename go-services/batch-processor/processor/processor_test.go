package processor

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"shared/logger"
)

// Simple mock implementations for testing
type SimpleMockDownloader struct {
	mock.Mock
}

func (m *SimpleMockDownloader) DownloadAndDecompress(ctx context.Context, bucket, key string) ([]byte, error) {
	args := m.Called(ctx, bucket, key)
	return args.Get(0).([]byte), args.Error(1)
}

type SimpleMockDeduplicator struct {
	mock.Mock
}

func (m *SimpleMockDeduplicator) DeduplicateWithStats(papers []Paper) ([]Paper, DeduplicationStats) {
	args := m.Called(papers)
	return args.Get(0).([]Paper), args.Get(1).(DeduplicationStats)
}

type SimpleMockWriter struct {
	mock.Mock
}

func (m *SimpleMockWriter) BatchUpsertWithStats(ctx context.Context, papers []Paper) (*UpsertStats, error) {
	args := m.Called(ctx, papers)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UpsertStats), args.Error(1)
}

type SimpleMockLogger struct {
	mock.Mock
}

func (m *SimpleMockLogger) WithTraceID(traceID string) *logger.Logger {
	// For testing, we can return a real logger or create a more sophisticated mock
	// For simplicity, let's return a real logger for now
	return logger.New("test-service").WithTraceID(traceID)
}

func (m *SimpleMockLogger) Info(message string, metadata ...map[string]interface{}) {
	m.Called(message, metadata)
}

func (m *SimpleMockLogger) InfoWithCount(message string, count int, metadata ...map[string]interface{}) {
	m.Called(message, count, metadata)
}

func (m *SimpleMockLogger) InfoWithDuration(message string, duration time.Duration, metadata ...map[string]interface{}) {
	m.Called(message, duration, metadata)
}

func (m *SimpleMockLogger) Warn(message string, metadata ...map[string]interface{}) {
	m.Called(message, metadata)
}

func (m *SimpleMockLogger) Error(message string, err error, metadata ...map[string]interface{}) {
	m.Called(message, err, metadata)
}

func (m *SimpleMockLogger) Debug(message string, metadata ...map[string]interface{}) {
	m.Called(message, metadata)
}

func (m *SimpleMockLogger) LogDataParsing(traceID string, parsedCount int, source string) {
	m.Called(traceID, parsedCount, source)
}

func (m *SimpleMockLogger) LogDeduplication(traceID string, stats interface{}) {
	m.Called(traceID, stats)
}

func (m *SimpleMockLogger) LogDynamoDBUpsert(traceID string, stats interface{}) {
	m.Called(traceID, stats)
}

func (m *SimpleMockLogger) LogProcessingComplete(traceID string, result interface{}) {
	m.Called(traceID, result)
}

func (m *SimpleMockLogger) LogError(traceID string, errorType string, err error, context map[string]interface{}) {
	m.Called(traceID, errorType, err, context)
}

func (m *SimpleMockLogger) LogWarning(traceID string, warningType string, message string, context map[string]interface{}) {
	m.Called(traceID, warningType, message, context)
}

func (m *SimpleMockLogger) LogMetrics(traceID string, metrics map[string]interface{}) {
	m.Called(traceID, metrics)
}

func TestS3EventProcessor_ProcessS3Event_EmptyRecords_Simple(t *testing.T) {
	mockDownloader := &SimpleMockDownloader{}
	mockDeduplicator := &SimpleMockDeduplicator{}
	mockWriter := &SimpleMockWriter{}
	mockLogger := &SimpleMockLogger{}
	processor := NewS3EventProcessor(mockDownloader, mockDeduplicator, mockWriter, mockLogger)

	// Create empty S3 event
	s3Event := events.S3Event{Records: []events.S3EventRecord{}}

	// Process event
	result, err := processor.ProcessS3Event(context.Background(), s3Event)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no S3 records to process")
}

func TestNewS3EventProcessor_Simple(t *testing.T) {
	mockDownloader := &SimpleMockDownloader{}
	mockDeduplicator := &SimpleMockDeduplicator{}
	mockWriter := &SimpleMockWriter{}
	mockLogger := &SimpleMockLogger{}
	processor := NewS3EventProcessor(mockDownloader, mockDeduplicator, mockWriter, mockLogger)

	assert.NotNil(t, processor)
	assert.Equal(t, mockDownloader, processor.downloader)
	assert.Equal(t, mockDeduplicator, processor.deduplicator)
	assert.Equal(t, mockWriter, processor.dynamoWriter)
	assert.Equal(t, mockLogger, processor.logger)
}