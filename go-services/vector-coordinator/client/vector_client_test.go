package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock HTTP client for testing
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func createTestClient() (*VectorAPIClient, *MockHTTPClient) {
	mockHTTPClient := &MockHTTPClient{}
	client := NewVectorAPIClientWithHTTPClient("http://test-api.com/embed", mockHTTPClient)
	return client, mockHTTPClient
}

func createMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestGenerateEmbedding_Success(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()
	text := "Test text for embedding"

	// Create successful response
	responseBody := `{
		"embedding": [0.1, 0.2, 0.3, 0.4, 0.5],
		"model_version": "test-model-v1.0",
		"dimension": 5,
		"processing_time_ms": 100
	}`
	mockResponse := createMockResponse(200, responseBody)

	mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(mockResponse, nil)

	// Execute
	response, err := client.GenerateEmbedding(ctx, text)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, []float64{0.1, 0.2, 0.3, 0.4, 0.5}, response.Embedding)
	assert.Equal(t, "test-model-v1.0", response.ModelVersion)
	assert.Equal(t, 5, response.Dimension)
	assert.Equal(t, 100, response.ProcessingTimeMs)

	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateEmbedding_EmptyText(t *testing.T) {
	client, _ := createTestClient()
	ctx := context.Background()

	// Execute
	response, err := client.GenerateEmbedding(ctx, "")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "text cannot be empty")
}

func TestGenerateEmbedding_LongTextTruncation(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()
	
	// Create text longer than maxTextLength (10000 chars)
	longText := strings.Repeat("a", 15000)

	responseBody := `{
		"embedding": [0.1, 0.2, 0.3],
		"model_version": "test-model-v1.0",
		"dimension": 3,
		"processing_time_ms": 100
	}`
	mockResponse := createMockResponse(200, responseBody)

	// Verify that the request body contains truncated text
	mockHTTPClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body)) // Reset body for actual use
		return strings.Contains(string(body), `"text":"`) && len(string(body)) < 15000
	})).Return(mockResponse, nil)

	// Execute
	response, err := client.GenerateEmbedding(ctx, longText)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, response)

	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateEmbedding_HTTPError(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()
	text := "Test text"

	// Setup mock to return HTTP error
	httpError := errors.New("connection refused")
	mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, httpError)

	// Execute
	response, err := client.GenerateEmbedding(ctx, text)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "HTTP request failed")
	assert.Contains(t, err.Error(), "connection refused")

	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateEmbedding_APIError(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()
	text := "Test text"

	// Create API error response
	responseBody := `{
		"error": {
			"code": "INVALID_INPUT",
			"message": "Text is too long",
			"timestamp": 1234567890
		}
	}`
	mockResponse := createMockResponse(400, responseBody)

	mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(mockResponse, nil)

	// Execute
	response, err := client.GenerateEmbedding(ctx, text)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "API error (INVALID_INPUT): Text is too long")

	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateEmbedding_InvalidResponse(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()
	text := "Test text"

	// Create response with invalid JSON
	mockResponse := createMockResponse(200, `{"invalid": "json"`)

	mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(mockResponse, nil)

	// Execute
	response, err := client.GenerateEmbedding(ctx, text)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to parse embedding response")

	mockHTTPClient.AssertExpectations(t)
}

func TestGenerateEmbedding_ValidationError(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()
	text := "Test text"

	// Create response with invalid embedding data
	responseBody := `{
		"embedding": [],
		"model_version": "",
		"dimension": 0,
		"processing_time_ms": 100
	}`
	mockResponse := createMockResponse(200, responseBody)

	mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(mockResponse, nil)

	// Execute
	response, err := client.GenerateEmbedding(ctx, text)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid embedding response")

	mockHTTPClient.AssertExpectations(t)
}

func TestValidateEmbeddingResponse(t *testing.T) {
	client, _ := createTestClient()

	// Test valid response
	validResponse := &EmbeddingResponse{
		Embedding:    []float64{0.1, 0.2, 0.3},
		ModelVersion: "test-model-v1.0",
		Dimension:    3,
	}
	assert.NoError(t, client.validateEmbeddingResponse(validResponse))

	// Test nil response
	assert.Error(t, client.validateEmbeddingResponse(nil))

	// Test empty embedding
	emptyEmbedding := &EmbeddingResponse{
		Embedding:    []float64{},
		ModelVersion: "test-model-v1.0",
		Dimension:    0,
	}
	err := client.validateEmbeddingResponse(emptyEmbedding)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding vector is empty")

	// Test dimension mismatch
	dimensionMismatch := &EmbeddingResponse{
		Embedding:    []float64{0.1, 0.2, 0.3},
		ModelVersion: "test-model-v1.0",
		Dimension:    5, // Wrong dimension
	}
	err = client.validateEmbeddingResponse(dimensionMismatch)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dimension mismatch")

	// Test empty model version
	emptyModel := &EmbeddingResponse{
		Embedding:    []float64{0.1, 0.2, 0.3},
		ModelVersion: "",
		Dimension:    3,
	}
	err = client.validateEmbeddingResponse(emptyModel)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model version is empty")

	// Test NaN in embedding
	nanEmbedding := &EmbeddingResponse{
		Embedding:    []float64{0.1, math.NaN(), 0.3}, // NaN
		ModelVersion: "test-model-v1.0",
		Dimension:    3,
	}
	err = client.validateEmbeddingResponse(nanEmbedding)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "embedding contains NaN")
}

func TestGetHealthStatus_Success(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()

	// Create successful health response
	mockResponse := createMockResponse(200, `{"status": "healthy"}`)

	mockHTTPClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == "GET" && strings.Contains(req.URL.Path, "/health")
	})).Return(mockResponse, nil)

	// Execute
	err := client.GetHealthStatus(ctx)

	// Assertions
	assert.NoError(t, err)

	mockHTTPClient.AssertExpectations(t)
}

func TestGetHealthStatus_Failure(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()

	// Create failed health response
	mockResponse := createMockResponse(503, `{"status": "unhealthy"}`)

	mockHTTPClient.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == "GET" && strings.Contains(req.URL.Path, "/health")
	})).Return(mockResponse, nil)

	// Execute
	err := client.GetHealthStatus(ctx)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed with status 503")

	mockHTTPClient.AssertExpectations(t)
}

func TestGetHealthStatus_HTTPError(t *testing.T) {
	client, mockHTTPClient := createTestClient()
	ctx := context.Background()

	// Setup mock to return HTTP error
	httpError := errors.New("connection refused")
	mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, httpError)

	// Execute
	err := client.GetHealthStatus(ctx)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
	assert.Contains(t, err.Error(), "connection refused")

	mockHTTPClient.AssertExpectations(t)
}

func TestNewVectorAPIClient_DefaultSettings(t *testing.T) {
	client := NewVectorAPIClient("http://test-api.com")

	assert.Equal(t, "http://test-api.com", client.baseURL)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.logger)
}

func TestNewVectorAPIClientWithHTTPClient_CustomSettings(t *testing.T) {
	mockHTTPClient := &MockHTTPClient{}
	client := NewVectorAPIClientWithHTTPClient("http://test-api.com", mockHTTPClient)

	assert.Equal(t, "http://test-api.com", client.baseURL)
	assert.Equal(t, mockHTTPClient, client.httpClient)
	assert.NotNil(t, client.logger)
}