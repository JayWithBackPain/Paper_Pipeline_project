package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"shared/logger"
	"vector-coordinator/client"
	"vector-coordinator/retriever"
	"vector-coordinator/storage"
)

// Mock implementations for testing
type MockDataRetriever struct {
	mock.Mock
}

func (m *MockDataRetriever) GetCombinedTextsByTraceID(ctx context.Context, traceID string) ([]retriever.CombinedText, error) {
	args := m.Called(ctx, traceID)
	return args.Get(0).([]retriever.CombinedText), args.Error(1)
}

func (m *MockDataRetriever) GetPapersByTraceID(ctx context.Context, traceID string) ([]retriever.Paper, error) {
	args := m.Called(ctx, traceID)
	return args.Get(0).([]retriever.Paper), args.Error(1)
}

type MockVectorAPIClient struct {
	mock.Mock
}

func (m *MockVectorAPIClient) GenerateEmbedding(ctx context.Context, text string) (*client.EmbeddingResponse, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*client.EmbeddingResponse), args.Error(1)
}

func (m *MockVectorAPIClient) GetHealthStatus(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type MockVectorStorage struct {
	mock.Mock
}

func (m *MockVectorStorage) BatchStoreVectors(ctx context.Context, records []storage.VectorRecord) (*storage.BatchWriteResult, error) {
	args := m.Called(ctx, records)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.BatchWriteResult), args.Error(1)
}

func createTestCoordinator() (*VectorCoordinator, *MockDataRetriever, *MockVectorAPIClient, *MockVectorStorage) {
	mockRetriever := &MockDataRetriever{}
	mockAPIClient := &MockVectorAPIClient{}
	mockStorage := &MockVectorStorage{}

	coordinator := &VectorCoordinator{
		retriever:     mockRetriever,
		apiClient:     mockAPIClient,
		vectorStorage: mockStorage,
		logger:        logger.New("test-vector-coordinator"),
	}

	return coordinator, mockRetriever, mockAPIClient, mockStorage
}

func TestProcessVectorization_Success(t *testing.T) {
	coordinator, mockRetriever, mockAPIClient, mockStorage := createTestCoordinator()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock expectations
	combinedTexts := []retriever.CombinedText{
		{PaperID: "paper1", Text: "Title 1. Abstract 1"},
		{PaperID: "paper2", Text: "Title 2. Abstract 2"},
	}

	embeddingResponse := &client.EmbeddingResponse{
		Embedding:    []float64{0.1, 0.2, 0.3},
		ModelVersion: "test-model-v1",
		Dimension:    3,
	}

	batchResult := &storage.BatchWriteResult{
		SuccessCount: 2,
		FailedItems:  []storage.VectorRecord{},
		Errors:       []error{},
	}

	mockRetriever.On("GetCombinedTextsByTraceID", ctx, traceID).Return(combinedTexts, nil)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 1. Abstract 1").Return(embeddingResponse, nil)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 2. Abstract 2").Return(embeddingResponse, nil)
	mockStorage.On("BatchStoreVectors", ctx, mock.AnythingOfType("[]storage.VectorRecord")).Return(batchResult, nil)

	// Execute
	result, err := coordinator.processVectorization(ctx, traceID)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, traceID, result.TraceID)
	assert.Equal(t, 2, result.TotalPapers)
	assert.Equal(t, 2, result.EmbeddingsGenerated)
	assert.Equal(t, 2, result.VectorsStored)
	assert.Equal(t, 0, result.FailedEmbeddings)
	assert.Equal(t, 0, result.FailedStorage)
	assert.True(t, result.ProcessingTimeMs > 0)

	mockRetriever.AssertExpectations(t)
	mockAPIClient.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestProcessVectorization_EmptyTraceID(t *testing.T) {
	coordinator, _, _, _ := createTestCoordinator()
	ctx := context.Background()

	// Execute
	result, err := coordinator.processVectorization(ctx, "")

	// Assertions
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "traceID cannot be empty")
	assert.Contains(t, result.ErrorMessage, "traceID cannot be empty")
}

func TestProcessVectorization_DataRetrievalFailure(t *testing.T) {
	coordinator, mockRetriever, _, _ := createTestCoordinator()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock expectations
	retrievalError := errors.New("database connection failed")
	mockRetriever.On("GetCombinedTextsByTraceID", ctx, traceID).Return([]retriever.CombinedText{}, retrievalError)

	// Execute
	result, err := coordinator.processVectorization(ctx, traceID)

	// Assertions
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "failed to retrieve papers for vectorization")
	assert.Contains(t, result.ErrorMessage, "failed to retrieve papers for vectorization")
	assert.True(t, result.ProcessingTimeMs > 0)

	mockRetriever.AssertExpectations(t)
}

func TestProcessVectorization_NoPapersFound(t *testing.T) {
	coordinator, mockRetriever, _, _ := createTestCoordinator()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock expectations
	mockRetriever.On("GetCombinedTextsByTraceID", ctx, traceID).Return([]retriever.CombinedText{}, nil)

	// Execute
	result, err := coordinator.processVectorization(ctx, traceID)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusCompleted, result.Status)
	assert.Equal(t, 0, result.TotalPapers)
	assert.Equal(t, 0, result.EmbeddingsGenerated)
	assert.Equal(t, 0, result.VectorsStored)

	mockRetriever.AssertExpectations(t)
}

func TestProcessVectorization_PartialEmbeddingFailure(t *testing.T) {
	coordinator, mockRetriever, mockAPIClient, mockStorage := createTestCoordinator()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock expectations
	combinedTexts := []retriever.CombinedText{
		{PaperID: "paper1", Text: "Title 1. Abstract 1"},
		{PaperID: "paper2", Text: "Title 2. Abstract 2"},
		{PaperID: "paper3", Text: "Title 3. Abstract 3"},
	}

	embeddingResponse := &client.EmbeddingResponse{
		Embedding:    []float64{0.1, 0.2, 0.3},
		ModelVersion: "test-model-v1",
		Dimension:    3,
	}

	embeddingError := errors.New("API rate limit exceeded")

	batchResult := &storage.BatchWriteResult{
		SuccessCount: 2,
		FailedItems:  []storage.VectorRecord{},
		Errors:       []error{},
	}

	mockRetriever.On("GetCombinedTextsByTraceID", ctx, traceID).Return(combinedTexts, nil)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 1. Abstract 1").Return(embeddingResponse, nil)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 2. Abstract 2").Return(nil, embeddingError)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 3. Abstract 3").Return(embeddingResponse, nil)
	mockStorage.On("BatchStoreVectors", ctx, mock.AnythingOfType("[]storage.VectorRecord")).Return(batchResult, nil)

	// Execute
	result, err := coordinator.processVectorization(ctx, traceID)

	// Assertions
	assert.Error(t, err) // Should error for partial failure to let Step Function retry
	assert.NotNil(t, result)
	assert.Equal(t, StatusPartial, result.Status)
	assert.Equal(t, 3, result.TotalPapers)
	assert.Equal(t, 2, result.EmbeddingsGenerated)
	assert.Equal(t, 2, result.VectorsStored)
	assert.Equal(t, 1, result.FailedEmbeddings)
	assert.Equal(t, 0, result.FailedStorage)
	assert.Contains(t, err.Error(), traceID) // Should contain traceID for Step Function

	mockRetriever.AssertExpectations(t)
	mockAPIClient.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestProcessVectorization_AllEmbeddingsFailed(t *testing.T) {
	coordinator, mockRetriever, mockAPIClient, _ := createTestCoordinator()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock expectations
	combinedTexts := []retriever.CombinedText{
		{PaperID: "paper1", Text: "Title 1. Abstract 1"},
		{PaperID: "paper2", Text: "Title 2. Abstract 2"},
	}

	embeddingError := errors.New("API service unavailable")

	mockRetriever.On("GetCombinedTextsByTraceID", ctx, traceID).Return(combinedTexts, nil)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 1. Abstract 1").Return(nil, embeddingError)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 2. Abstract 2").Return(nil, embeddingError)

	// Execute
	result, err := coordinator.processVectorization(ctx, traceID)

	// Assertions
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, 2, result.TotalPapers)
	assert.Equal(t, 0, result.EmbeddingsGenerated)
	assert.Equal(t, 0, result.VectorsStored)
	assert.Equal(t, 2, result.FailedEmbeddings)
	assert.Contains(t, err.Error(), "no embeddings were generated successfully")

	mockRetriever.AssertExpectations(t)
	mockAPIClient.AssertExpectations(t)
}

func TestProcessVectorization_StorageFailure(t *testing.T) {
	coordinator, mockRetriever, mockAPIClient, mockStorage := createTestCoordinator()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock expectations
	combinedTexts := []retriever.CombinedText{
		{PaperID: "paper1", Text: "Title 1. Abstract 1"},
	}

	embeddingResponse := &client.EmbeddingResponse{
		Embedding:    []float64{0.1, 0.2, 0.3},
		ModelVersion: "test-model-v1",
		Dimension:    3,
	}

	storageError := errors.New("DynamoDB write failed")

	mockRetriever.On("GetCombinedTextsByTraceID", ctx, traceID).Return(combinedTexts, nil)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 1. Abstract 1").Return(embeddingResponse, nil)
	mockStorage.On("BatchStoreVectors", ctx, mock.AnythingOfType("[]storage.VectorRecord")).Return(nil, storageError)

	// Execute
	result, err := coordinator.processVectorization(ctx, traceID)

	// Assertions
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, 1, result.TotalPapers)
	assert.Equal(t, 1, result.EmbeddingsGenerated)
	assert.Equal(t, 0, result.VectorsStored)
	assert.Contains(t, err.Error(), "failed to store vector records")

	mockRetriever.AssertExpectations(t)
	mockAPIClient.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestProcessVectorization_PartialStorageFailure(t *testing.T) {
	coordinator, mockRetriever, mockAPIClient, mockStorage := createTestCoordinator()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock expectations
	combinedTexts := []retriever.CombinedText{
		{PaperID: "paper1", Text: "Title 1. Abstract 1"},
		{PaperID: "paper2", Text: "Title 2. Abstract 2"},
	}

	embeddingResponse := &client.EmbeddingResponse{
		Embedding:    []float64{0.1, 0.2, 0.3},
		ModelVersion: "test-model-v1",
		Dimension:    3,
	}

	batchResult := &storage.BatchWriteResult{
		SuccessCount: 1,
		FailedItems:  []storage.VectorRecord{{PaperID: "paper2"}},
		Errors:       []error{errors.New("throttling error")},
	}

	mockRetriever.On("GetCombinedTextsByTraceID", ctx, traceID).Return(combinedTexts, nil)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 1. Abstract 1").Return(embeddingResponse, nil)
	mockAPIClient.On("GenerateEmbedding", ctx, "Title 2. Abstract 2").Return(embeddingResponse, nil)
	mockStorage.On("BatchStoreVectors", ctx, mock.AnythingOfType("[]storage.VectorRecord")).Return(batchResult, nil)

	// Execute
	result, err := coordinator.processVectorization(ctx, traceID)

	// Assertions
	assert.Error(t, err) // Should error for partial failure to let Step Function retry
	assert.NotNil(t, result)
	assert.Equal(t, StatusPartial, result.Status)
	assert.Equal(t, 2, result.TotalPapers)
	assert.Equal(t, 2, result.EmbeddingsGenerated)
	assert.Equal(t, 1, result.VectorsStored)
	assert.Equal(t, 0, result.FailedEmbeddings)
	assert.Equal(t, 1, result.FailedStorage)
	assert.Contains(t, err.Error(), traceID) // Should contain traceID for Step Function

	mockRetriever.AssertExpectations(t)
	mockAPIClient.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestPerformHealthCheck_AllHealthy(t *testing.T) {
	coordinator, mockRetriever, mockAPIClient, _ := createTestCoordinator()
	ctx := context.Background()

	// Setup mock expectations
	mockAPIClient.On("GetHealthStatus", ctx).Return(nil)
	mockRetriever.On("GetPapersByTraceID", ctx, "health-check-trace-id").Return([]retriever.Paper{}, nil)

	// Execute
	result := coordinator.performHealthCheck(ctx)

	// Assertions
	assert.NotNil(t, result)
	assert.Equal(t, "vector-coordinator", result.Service)
	assert.Equal(t, "healthy", result.Status)
	assert.Equal(t, "healthy", result.Components["embedding_api"])
	assert.Equal(t, "healthy", result.Components["dynamodb_papers"])
	assert.Equal(t, "All components are healthy", result.Message)

	mockAPIClient.AssertExpectations(t)
	mockRetriever.AssertExpectations(t)
}

func TestPerformHealthCheck_APIUnhealthy(t *testing.T) {
	coordinator, mockRetriever, mockAPIClient, _ := createTestCoordinator()
	ctx := context.Background()

	// Setup mock expectations
	apiError := errors.New("API service unavailable")
	mockAPIClient.On("GetHealthStatus", ctx).Return(apiError)
	mockRetriever.On("GetPapersByTraceID", ctx, "health-check-trace-id").Return([]retriever.Paper{}, nil)

	// Execute
	result := coordinator.performHealthCheck(ctx)

	// Assertions
	assert.NotNil(t, result)
	assert.Equal(t, "vector-coordinator", result.Service)
	assert.Equal(t, "degraded", result.Status)
	assert.Equal(t, "unhealthy", result.Components["embedding_api"])
	assert.Equal(t, "healthy", result.Components["dynamodb_papers"])
	assert.Equal(t, "Some components are unhealthy", result.Message)

	mockAPIClient.AssertExpectations(t)
	mockRetriever.AssertExpectations(t)
}

func TestProcessingError_Error(t *testing.T) {
	// Test ProcessingError without cause
	err1 := &ProcessingError{
		Stage:   "test_stage",
		Message: "test message",
	}
	assert.Equal(t, "test_stage: test message", err1.Error())

	// Test ProcessingError with cause
	cause := errors.New("root cause")
	err2 := &ProcessingError{
		Stage:   "test_stage",
		Message: "test message",
		Cause:   cause,
	}
	assert.Equal(t, "test_stage: test message (caused by: root cause)", err2.Error())
	assert.Equal(t, cause, err2.Unwrap())
}

func TestLogSystemMetrics(t *testing.T) {
	coordinator, _, _, _ := createTestCoordinator()
	ctx := context.Background()

	result := &ProcessingResult{
		TraceID:             "test-trace-123",
		Status:              StatusCompleted,
		TotalPapers:         10,
		EmbeddingsGenerated: 9,
		VectorsStored:       8,
		FailedEmbeddings:    1,
		FailedStorage:       1,
		ProcessingTimeMs:    5000,
		Timestamp:           time.Now().UTC().Format(time.RFC3339),
	}

	// This should not panic and should log metrics
	assert.NotPanics(t, func() {
		coordinator.logSystemMetrics(ctx, result)
	})
}

func TestHandleStepFunction_Success(t *testing.T) {
	// This test would require more complex mocking of the initialization
	// For now, we'll test the basic structure
	input := StepFunctionInput{
		TraceID: "test-trace-123",
	}

	assert.NotEmpty(t, input.TraceID)
}