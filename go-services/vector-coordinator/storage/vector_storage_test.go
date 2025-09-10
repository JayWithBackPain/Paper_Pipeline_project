package storage

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock DynamoDB client for testing
type MockDynamoDBClient struct {
	dynamodbiface.DynamoDBAPI
	mock.Mock
}

func (m *MockDynamoDBClient) BatchWriteItemWithContext(ctx context.Context, input *dynamodb.BatchWriteItemInput, opts ...request.Option) (*dynamodb.BatchWriteItemOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.BatchWriteItemOutput), args.Error(1)
}

func (m *MockDynamoDBClient) PutItemWithContext(ctx context.Context, input *dynamodb.PutItemInput, opts ...request.Option) (*dynamodb.PutItemOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}

func (m *MockDynamoDBClient) GetItemWithContext(ctx context.Context, input *dynamodb.GetItemInput, opts ...request.Option) (*dynamodb.GetItemOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.GetItemOutput), args.Error(1)
}

func createTestStorage() (*VectorStorage, *MockDynamoDBClient) {
	mockClient := &MockDynamoDBClient{}
	storage := NewVectorStorageWithClient(mockClient, "test-vectors-table")
	return storage, mockClient
}

func createTestVectorRecord(paperID string) VectorRecord {
	return VectorRecord{
		PaperID:    paperID,
		VectorType: "title_abstract",
		Embedding:  []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		EmbeddingMetadata: EmbeddingMetadata{
			ModelName:     "test-model",
			ModelVersion:  "v1.0",
			Dimension:     5,
			TextLength:    100,
			Preprocessing: "title_abstract_combination",
		},
		SourceText: SourceText{
			Content:      "Test title. Test abstract.",
			SourceFields: []string{"title", "abstract"},
			Language:     "en",
		},
		ProcessingInfo: ProcessingInfo{
			CreatedAt:        time.Now().UTC().Format(time.RFC3339),
			TraceID:          "test-trace-123",
			ProcessingTimeMs: 150,
		},
	}
}

func TestBatchStoreVectors_Success(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	// Create test records
	records := []VectorRecord{
		createTestVectorRecord("paper1"),
		createTestVectorRecord("paper2"),
	}

	// Setup successful batch write response
	mockOutput := &dynamodb.BatchWriteItemOutput{
		UnprocessedItems: nil, // All items processed successfully
	}

	mockClient.On("BatchWriteItemWithContext", ctx, mock.AnythingOfType("*dynamodb.BatchWriteItemInput")).Return(mockOutput, nil)

	// Execute
	result, err := storage.BatchStoreVectors(ctx, records)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Empty(t, result.FailedItems)
	assert.Empty(t, result.Errors)

	mockClient.AssertExpectations(t)
}

func TestBatchStoreVectors_EmptyRecords(t *testing.T) {
	storage, _ := createTestStorage()
	ctx := context.Background()

	// Execute with empty records
	result, err := storage.BatchStoreVectors(ctx, []VectorRecord{})

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Empty(t, result.FailedItems)
	assert.Empty(t, result.Errors)
}

func TestBatchStoreVectors_InvalidRecords(t *testing.T) {
	storage, _ := createTestStorage()
	ctx := context.Background()

	// Create records with validation errors
	records := []VectorRecord{
		createTestVectorRecord("paper1"), // Valid
		{
			PaperID:    "", // Invalid - empty paper_id
			VectorType: "title_abstract",
			Embedding:  []float64{0.1, 0.2, 0.3},
		},
		{
			PaperID:    "paper3",
			VectorType: "title_abstract",
			Embedding:  []float64{}, // Invalid - empty embedding
		},
	}

	// Execute
	result, err := storage.BatchStoreVectors(ctx, records)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount) // No valid records to process
	assert.Len(t, result.FailedItems, 2)    // Two invalid records
	assert.Len(t, result.Errors, 2)        // Two validation errors
}

func TestBatchStoreVectors_DynamoDBError(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	records := []VectorRecord{
		createTestVectorRecord("paper1"),
	}

	// Setup DynamoDB error
	dynamoError := errors.New("DynamoDB service unavailable")
	mockClient.On("BatchWriteItemWithContext", ctx, mock.AnythingOfType("*dynamodb.BatchWriteItemInput")).Return(nil, dynamoError)

	// Execute
	result, err := storage.BatchStoreVectors(ctx, records)

	// Assertions
	assert.NoError(t, err) // Should not return error, but handle failed items
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Len(t, result.FailedItems, 1)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "DynamoDB service unavailable")

	mockClient.AssertExpectations(t)
}

func TestBatchStoreVectors_UnprocessedItems(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	records := []VectorRecord{
		createTestVectorRecord("paper1"),
		createTestVectorRecord("paper2"),
		createTestVectorRecord("paper3"),
	}

	// Setup response with unprocessed items
	mockOutput := &dynamodb.BatchWriteItemOutput{
		UnprocessedItems: map[string][]*dynamodb.WriteRequest{
			"test-vectors-table": {
				{
					PutRequest: &dynamodb.PutRequest{
						Item: map[string]*dynamodb.AttributeValue{
							"paper_id": {S: aws.String("paper3")},
						},
					},
				},
			},
		},
	}

	mockClient.On("BatchWriteItemWithContext", ctx, mock.AnythingOfType("*dynamodb.BatchWriteItemInput")).Return(mockOutput, nil)

	// Execute
	result, err := storage.BatchStoreVectors(ctx, records)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.SuccessCount) // 3 - 1 unprocessed
	assert.Len(t, result.FailedItems, 1)    // 1 unprocessed item
	assert.Empty(t, result.Errors)

	mockClient.AssertExpectations(t)
}

func TestBatchStoreVectors_UnprocessedItemsHandling(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	records := []VectorRecord{
		createTestVectorRecord("paper1"),
		createTestVectorRecord("paper2"),
	}

	// Response with unprocessed items (no retry, just report as failed)
	output := &dynamodb.BatchWriteItemOutput{
		UnprocessedItems: map[string][]*dynamodb.WriteRequest{
			"test-vectors-table": {
				{
					PutRequest: &dynamodb.PutRequest{
						Item: map[string]*dynamodb.AttributeValue{
							"paper_id": {S: aws.String("paper2")},
						},
					},
				},
			},
		},
	}

	mockClient.On("BatchWriteItemWithContext", ctx, mock.AnythingOfType("*dynamodb.BatchWriteItemInput")).Return(output, nil)

	// Execute
	result, err := storage.BatchStoreVectors(ctx, records)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.SuccessCount) // One item succeeded
	assert.Len(t, result.FailedItems, 1)    // One item failed (unprocessed)
	assert.Empty(t, result.Errors)

	mockClient.AssertExpectations(t)
}

func TestValidateVectorRecord(t *testing.T) {
	storage, _ := createTestStorage()

	// Test valid record
	validRecord := createTestVectorRecord("paper1")
	assert.NoError(t, storage.validateVectorRecord(&validRecord))

	// Test empty paper_id
	invalidRecord1 := validRecord
	invalidRecord1.PaperID = ""
	err := storage.validateVectorRecord(&invalidRecord1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paper_id is empty")

	// Test empty vector_type
	invalidRecord2 := validRecord
	invalidRecord2.VectorType = ""
	err = storage.validateVectorRecord(&invalidRecord2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vector_type is empty")

	// Test empty embedding
	invalidRecord3 := validRecord
	invalidRecord3.Embedding = []float64{}
	err = storage.validateVectorRecord(&invalidRecord3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding vector is empty")

	// Test dimension mismatch
	invalidRecord4 := validRecord
	invalidRecord4.EmbeddingMetadata.Dimension = 10 // Wrong dimension
	err = storage.validateVectorRecord(&invalidRecord4)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dimension mismatch")

	// Test empty model version
	invalidRecord5 := validRecord
	invalidRecord5.EmbeddingMetadata.ModelVersion = ""
	err = storage.validateVectorRecord(&invalidRecord5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model_version is empty")

	// Test empty trace_id
	invalidRecord6 := validRecord
	invalidRecord6.ProcessingInfo.TraceID = ""
	err = storage.validateVectorRecord(&invalidRecord6)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trace_id is empty")

	// Test NaN in embedding
	invalidRecord7 := validRecord
	invalidRecord7.Embedding = []float64{0.1, math.NaN(), 0.3} // NaN
	err = storage.validateVectorRecord(&invalidRecord7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding contains NaN")
}

func TestStoreVector_Success(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	record := createTestVectorRecord("paper1")

	// Setup successful put response
	mockOutput := &dynamodb.PutItemOutput{}
	mockClient.On("PutItemWithContext", ctx, mock.AnythingOfType("*dynamodb.PutItemInput")).Return(mockOutput, nil)

	// Execute
	err := storage.StoreVector(ctx, &record)

	// Assertions
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestStoreVector_NilRecord(t *testing.T) {
	storage, _ := createTestStorage()
	ctx := context.Background()

	// Execute
	err := storage.StoreVector(ctx, nil)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vector record cannot be nil")
}

func TestStoreVector_DynamoDBError(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	record := createTestVectorRecord("paper1")

	// Setup DynamoDB error
	dynamoError := errors.New("DynamoDB put failed")
	mockClient.On("PutItemWithContext", ctx, mock.AnythingOfType("*dynamodb.PutItemInput")).Return(nil, dynamoError)

	// Execute
	err := storage.StoreVector(ctx, &record)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store vector record")
	assert.Contains(t, err.Error(), "DynamoDB put failed")

	mockClient.AssertExpectations(t)
}

func TestGetVectorByPaperID_Success(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	paperID := "paper1"
	vectorType := "title_abstract"

	// Create mock response
	mockOutput := &dynamodb.GetItemOutput{
		Item: map[string]*dynamodb.AttributeValue{
			"paper_id":    {S: aws.String(paperID)},
			"vector_type": {S: aws.String(vectorType)},
			"embedding":   {NS: []*string{aws.String("0.1"), aws.String("0.2"), aws.String("0.3")}},
			"embedding_metadata": {M: map[string]*dynamodb.AttributeValue{
				"model_name":     {S: aws.String("test-model")},
				"model_version":  {S: aws.String("v1.0")},
				"dimension":      {N: aws.String("3")},
				"text_length":    {N: aws.String("100")},
				"preprocessing":  {S: aws.String("title_abstract_combination")},
			}},
			"source_text": {M: map[string]*dynamodb.AttributeValue{
				"content":       {S: aws.String("Test content")},
				"source_fields": {SS: []*string{aws.String("title"), aws.String("abstract")}},
				"language":      {S: aws.String("en")},
			}},
			"processing_info": {M: map[string]*dynamodb.AttributeValue{
				"created_at":         {S: aws.String("2023-01-01T00:00:00Z")},
				"trace_id":           {S: aws.String("test-trace-123")},
				"processing_time_ms": {N: aws.String("150")},
			}},
		},
	}

	mockClient.On("GetItemWithContext", ctx, mock.AnythingOfType("*dynamodb.GetItemInput")).Return(mockOutput, nil)

	// Execute
	record, err := storage.GetVectorByPaperID(ctx, paperID, vectorType)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, paperID, record.PaperID)
	assert.Equal(t, vectorType, record.VectorType)

	mockClient.AssertExpectations(t)
}

func TestGetVectorByPaperID_NotFound(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	paperID := "paper1"
	vectorType := "title_abstract"

	// Create mock response with no item
	mockOutput := &dynamodb.GetItemOutput{
		Item: nil,
	}

	mockClient.On("GetItemWithContext", ctx, mock.AnythingOfType("*dynamodb.GetItemInput")).Return(mockOutput, nil)

	// Execute
	record, err := storage.GetVectorByPaperID(ctx, paperID, vectorType)

	// Assertions
	assert.NoError(t, err)
	assert.Nil(t, record)

	mockClient.AssertExpectations(t)
}

func TestGetVectorByPaperID_DynamoDBError(t *testing.T) {
	storage, mockClient := createTestStorage()
	ctx := context.Background()

	paperID := "paper1"
	vectorType := "title_abstract"

	// Setup DynamoDB error
	dynamoError := errors.New("DynamoDB get failed")
	mockClient.On("GetItemWithContext", ctx, mock.AnythingOfType("*dynamodb.GetItemInput")).Return(nil, dynamoError)

	// Execute
	record, err := storage.GetVectorByPaperID(ctx, paperID, vectorType)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, record)
	assert.Contains(t, err.Error(), "failed to get vector record")
	assert.Contains(t, err.Error(), "DynamoDB get failed")

	mockClient.AssertExpectations(t)
}

func TestCreateVectorRecord(t *testing.T) {
	paperID := "paper1"
	text := "Test title. Test abstract."
	traceID := "test-trace-123"
	embedding := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	modelVersion := "test-model-v1.0"
	processingTimeMs := int64(150)

	// Execute
	record := CreateVectorRecord(paperID, text, traceID, embedding, modelVersion, processingTimeMs)

	// Assertions
	assert.NotNil(t, record)
	assert.Equal(t, paperID, record.PaperID)
	assert.Equal(t, "title_abstract", record.VectorType)
	assert.Equal(t, embedding, record.Embedding)
	assert.Equal(t, traceID, record.ProcessingInfo.TraceID)
	assert.Equal(t, processingTimeMs, record.ProcessingInfo.ProcessingTimeMs)
	assert.Equal(t, modelVersion, record.EmbeddingMetadata.ModelVersion)
	assert.Equal(t, len(embedding), record.EmbeddingMetadata.Dimension)
	assert.Equal(t, len(text), record.EmbeddingMetadata.TextLength)
	assert.Equal(t, text, record.SourceText.Content)
	assert.Equal(t, []string{"title", "abstract"}, record.SourceText.SourceFields)
	assert.Equal(t, "en", record.SourceText.Language)
	assert.NotEmpty(t, record.ProcessingInfo.CreatedAt)
}

func TestExtractModelName(t *testing.T) {
	// Test basic model version
	modelVersion := "test-model-v1.0"
	modelName := extractModelName(modelVersion)
	assert.Equal(t, modelVersion, modelName) // Currently returns the full version

	// Test empty model version
	emptyModelName := extractModelName("")
	assert.Equal(t, "", emptyModelName)
}

func TestNewVectorStorage(t *testing.T) {
	tableName := "test-table"
	storage := NewVectorStorage(tableName)

	assert.NotNil(t, storage)
	assert.Equal(t, tableName, storage.tableName)
	assert.NotNil(t, storage.client)
	assert.NotNil(t, storage.logger)
}

func TestNewVectorStorageWithClient(t *testing.T) {
	mockClient := &MockDynamoDBClient{}
	tableName := "test-table"
	storage := NewVectorStorageWithClient(mockClient, tableName)

	assert.NotNil(t, storage)
	assert.Equal(t, tableName, storage.tableName)
	assert.Equal(t, mockClient, storage.client)
	assert.NotNil(t, storage.logger)
}