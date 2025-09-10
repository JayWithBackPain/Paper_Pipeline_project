package dynamodb

import (
	"batch-processor/processor"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDynamoDBAPI is a mock implementation of DynamoDB API
type MockDynamoDBAPI struct {
	dynamodbiface.DynamoDBAPI
	mock.Mock
}

func (m *MockDynamoDBAPI) BatchWriteItemWithContext(ctx context.Context, input *dynamodb.BatchWriteItemInput, opts ...request.Option) (*dynamodb.BatchWriteItemOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*dynamodb.BatchWriteItemOutput), args.Error(1)
}

func TestWriter_BatchUpsert_Success(t *testing.T) {
	mockClient := &MockDynamoDBAPI{}
	writer := NewWriterWithClient(mockClient, "test-table")

	// Create test papers
	papers := []processor.Paper{
		createTestPaper("paper-1", "Test Paper 1"),
		createTestPaper("paper-2", "Test Paper 2"),
	}

	// Mock successful batch write
	mockOutput := &dynamodb.BatchWriteItemOutput{
		UnprocessedItems: map[string][]*dynamodb.WriteRequest{},
	}
	mockClient.On("BatchWriteItemWithContext", mock.Anything, mock.MatchedBy(func(input *dynamodb.BatchWriteItemInput) bool {
		return len(input.RequestItems["test-table"]) == 2
	})).Return(mockOutput, nil)

	// Execute batch upsert
	err := writer.BatchUpsert(context.Background(), papers)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestWriter_BatchUpsert_EmptyInput(t *testing.T) {
	mockClient := &MockDynamoDBAPI{}
	writer := NewWriterWithClient(mockClient, "test-table")

	// Execute with empty papers
	err := writer.BatchUpsert(context.Background(), []processor.Paper{})

	assert.NoError(t, err)
	// No DynamoDB calls should be made
	mockClient.AssertNotCalled(t, "BatchWriteItemWithContext")
}

func TestWriter_BatchUpsert_LargeBatch(t *testing.T) {
	mockClient := &MockDynamoDBAPI{}
	writer := NewWriterWithClient(mockClient, "test-table")

	// Create 30 test papers (should be split into 2 batches: 25 + 5)
	papers := make([]processor.Paper, 30)
	for i := 0; i < 30; i++ {
		papers[i] = createTestPaper(fmt.Sprintf("paper-%d", i+1), fmt.Sprintf("Test Paper %d", i+1))
	}

	// Mock successful batch writes
	mockOutput := &dynamodb.BatchWriteItemOutput{
		UnprocessedItems: map[string][]*dynamodb.WriteRequest{},
	}

	// First batch (25 items)
	mockClient.On("BatchWriteItemWithContext", mock.Anything, mock.MatchedBy(func(input *dynamodb.BatchWriteItemInput) bool {
		return len(input.RequestItems["test-table"]) == 25
	})).Return(mockOutput, nil).Once()

	// Second batch (5 items)
	mockClient.On("BatchWriteItemWithContext", mock.Anything, mock.MatchedBy(func(input *dynamodb.BatchWriteItemInput) bool {
		return len(input.RequestItems["test-table"]) == 5
	})).Return(mockOutput, nil).Once()

	// Execute batch upsert
	err := writer.BatchUpsert(context.Background(), papers)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestWriter_BatchUpsert_WithUnprocessedItems(t *testing.T) {
	mockClient := &MockDynamoDBAPI{}
	writer := NewWriterWithClient(mockClient, "test-table")

	papers := []processor.Paper{
		createTestPaper("paper-1", "Test Paper 1"),
		createTestPaper("paper-2", "Test Paper 2"),
	}

	// First call returns unprocessed items
	firstOutput := &dynamodb.BatchWriteItemOutput{
		UnprocessedItems: map[string][]*dynamodb.WriteRequest{
			"test-table": {
				{
					PutRequest: &dynamodb.PutRequest{
						Item: map[string]*dynamodb.AttributeValue{
							"paper_id": {S: aws.String("paper-2")},
						},
					},
				},
			},
		},
	}

	// Second call (retry) succeeds
	secondOutput := &dynamodb.BatchWriteItemOutput{
		UnprocessedItems: map[string][]*dynamodb.WriteRequest{},
	}

	mockClient.On("BatchWriteItemWithContext", mock.Anything, mock.MatchedBy(func(input *dynamodb.BatchWriteItemInput) bool {
		return len(input.RequestItems["test-table"]) == 2
	})).Return(firstOutput, nil).Once()

	mockClient.On("BatchWriteItemWithContext", mock.Anything, mock.MatchedBy(func(input *dynamodb.BatchWriteItemInput) bool {
		return len(input.RequestItems["test-table"]) == 1
	})).Return(secondOutput, nil).Once()

	// Execute batch upsert
	err := writer.BatchUpsert(context.Background(), papers)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestWriter_BatchUpsertWithStats_Success(t *testing.T) {
	mockClient := &MockDynamoDBAPI{}
	writer := NewWriterWithClient(mockClient, "test-table")

	papers := []processor.Paper{
		createTestPaper("paper-1", "Test Paper 1"),
		createTestPaper("paper-2", "Test Paper 2"),
		createTestPaper("paper-3", "Test Paper 3"),
	}

	// Mock successful batch write
	mockOutput := &dynamodb.BatchWriteItemOutput{
		UnprocessedItems: map[string][]*dynamodb.WriteRequest{},
	}
	mockClient.On("BatchWriteItemWithContext", mock.Anything, mock.Anything).Return(mockOutput, nil)

	// Execute batch upsert with stats
	stats, err := writer.BatchUpsertWithStats(context.Background(), papers)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 3, stats.TotalItems)
	assert.Equal(t, 3, stats.SuccessItems)
	assert.Equal(t, 0, stats.FailedItems)
	assert.Equal(t, 1, stats.BatchCount)
	assert.Equal(t, 1, stats.SuccessBatches)
	assert.Equal(t, 0, stats.FailedBatches)

	mockClient.AssertExpectations(t)
}

func TestWriter_BatchUpsertWithStats_EmptyInput(t *testing.T) {
	mockClient := &MockDynamoDBAPI{}
	writer := NewWriterWithClient(mockClient, "test-table")

	// Execute with empty papers
	stats, err := writer.BatchUpsertWithStats(context.Background(), []processor.Paper{})

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats.TotalItems)
	assert.Equal(t, 0, stats.SuccessItems)
	assert.Equal(t, 0, stats.FailedItems)
	assert.Equal(t, 0, stats.BatchCount)

	// No DynamoDB calls should be made
	mockClient.AssertNotCalled(t, "BatchWriteItemWithContext")
}

func TestNewWriter(t *testing.T) {
	writer := NewWriter("test-table")
	assert.NotNil(t, writer)
	assert.Equal(t, "test-table", writer.tableName)
	assert.NotNil(t, writer.client)
}

func TestNewWriterWithClient(t *testing.T) {
	mockClient := &MockDynamoDBAPI{}
	writer := NewWriterWithClient(mockClient, "test-table")
	
	assert.NotNil(t, writer)
	assert.Equal(t, "test-table", writer.tableName)
	assert.Equal(t, mockClient, writer.client)
}

// Helper function to create test papers
func createTestPaper(id, title string) processor.Paper {
	now := time.Now().Format(time.RFC3339)
	return processor.Paper{
		PaperID:          id,
		Source:           "test",
		Title:            title,
		Abstract:         "Test abstract",
		Authors:          []string{"Test Author"},
		PublishedDate:    "2023-01-01",
		Categories:       []string{"test"},
		TraceID:          "test-trace",
		BatchTimestamp:   now,
		ProcessingStatus: "processed",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}