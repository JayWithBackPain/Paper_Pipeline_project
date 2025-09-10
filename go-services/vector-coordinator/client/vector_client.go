package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"shared/logger"
)

// HTTPClient interface for making HTTP requests (allows mocking)
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// VectorAPIClient handles HTTP communication with the Python vectorization API
type VectorAPIClient struct {
	baseURL    string
	httpClient HTTPClient
	logger     *logger.Logger
}

// EmbeddingRequest represents the request payload for the vectorization API
type EmbeddingRequest struct {
	Text string `json:"text"`
}

// EmbeddingResponse represents the response from the vectorization API
type EmbeddingResponse struct {
	Embedding       []float64 `json:"embedding"`
	ModelVersion    string    `json:"model_version"`
	Dimension       int       `json:"dimension"`
	ProcessingTimeMs int      `json:"processing_time_ms"`
}

// APIError represents an error response from the vectorization API
type APIError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Timestamp int64  `json:"timestamp"`
	} `json:"error"`
}

// NewVectorAPIClient creates a new HTTP client for the vectorization API
func NewVectorAPIClient(baseURL string) *VectorAPIClient {
	return &VectorAPIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
				MaxIdleConnsPerHost: 5,
			},
		},
		logger: logger.New("vector-api-client"),
	}
}

// NewVectorAPIClientWithHTTPClient creates a client with a custom HTTP client (for testing)
func NewVectorAPIClientWithHTTPClient(baseURL string, httpClient HTTPClient) *VectorAPIClient {
	return &VectorAPIClient{
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     logger.New("vector-api-client"),
	}
}

// GenerateEmbedding calls the Python API to generate an embedding for the given text
func (c *VectorAPIClient) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResponse, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	contextLogger := c.logger.WithContext(ctx)
	startTime := time.Now()

	contextLogger.Info("Starting embedding generation", map[string]interface{}{
		"text_length": len(text),
		"api_url":     c.baseURL,
	})

	// Validate text length (prevent extremely long texts)
	const maxTextLength = 10000 // Adjust based on model limits
	if len(text) > maxTextLength {
		contextLogger.Warn("Text length exceeds maximum", map[string]interface{}{
			"text_length": len(text),
			"max_length":  maxTextLength,
		})
		text = text[:maxTextLength] // Truncate text
	}

	// Prepare request payload
	request := EmbeddingRequest{
		Text: text,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		contextLogger.Error("Failed to marshal request", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		contextLogger.Error("Failed to create HTTP request", err)
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Make HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		contextLogger.Error("HTTP request failed", err, map[string]interface{}{
			"url": c.baseURL,
		})
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		contextLogger.Error("Failed to read response body", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	duration := time.Since(startTime)

	// Log request metrics
	contextLogger.Debug("HTTP request completed", map[string]interface{}{
		"status_code":         resp.StatusCode,
		"request_duration_ms": duration.Milliseconds(),
		"response_size":       len(responseBody),
	})

	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		var apiError APIError
		if err := json.Unmarshal(responseBody, &apiError); err != nil {
			contextLogger.Error("Failed to parse error response", err, map[string]interface{}{
				"status_code":   resp.StatusCode,
				"response_body": string(responseBody),
			})
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(responseBody))
		}

		contextLogger.Error("API returned error", nil, map[string]interface{}{
			"status_code":   resp.StatusCode,
			"error_code":    apiError.Error.Code,
			"error_message": apiError.Error.Message,
		})
		return nil, fmt.Errorf("API error (%s): %s", apiError.Error.Code, apiError.Error.Message)
	}

	// Parse successful response
	var embeddingResponse EmbeddingResponse
	if err := json.Unmarshal(responseBody, &embeddingResponse); err != nil {
		contextLogger.Error("Failed to parse embedding response", err, map[string]interface{}{
			"response_body": string(responseBody),
		})
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}

	// Validate response
	if err := c.validateEmbeddingResponse(&embeddingResponse); err != nil {
		contextLogger.Error("Invalid embedding response", err)
		return nil, fmt.Errorf("invalid embedding response: %w", err)
	}

	contextLogger.InfoWithDuration("Successfully generated embedding", duration, map[string]interface{}{
		"embedding_dimension":    embeddingResponse.Dimension,
		"model_version":          embeddingResponse.ModelVersion,
		"api_processing_time_ms": embeddingResponse.ProcessingTimeMs,
	})

	return &embeddingResponse, nil
}

// validateEmbeddingResponse validates the structure and content of the embedding response
func (c *VectorAPIClient) validateEmbeddingResponse(response *EmbeddingResponse) error {
	if response == nil {
		return fmt.Errorf("response is nil")
	}

	if len(response.Embedding) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}

	if response.Dimension != len(response.Embedding) {
		return fmt.Errorf("dimension mismatch: expected %d, got %d", response.Dimension, len(response.Embedding))
	}

	if response.ModelVersion == "" {
		return fmt.Errorf("model version is empty")
	}

	// Validate that embedding contains valid float values
	for i, val := range response.Embedding {
		if val != val { // Check for NaN
			return fmt.Errorf("embedding contains NaN at index %d", i)
		}
	}

	return nil
}

