package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"shared/logger"
)

// VectorRecord represents a vector record to be stored in DynamoDB
type VectorRecord struct {
	PaperID   string    `json:"paper_id" dynamodbav:"paper_id"`
	VectorType string   `json:"vector_type" dynamodbav:"vector_type"`
	Embedding []float64 `json:"embedding" dynamodbav:"embedding"`
	EmbeddingMetadata EmbeddingMetadata `json:"embedding_metadata" dynamodbav:"embedding_metadata"`
	SourceText SourceText `json:"source_text" dynamodbav:"source_text"`
	ProcessingInfo ProcessingInfo `json:"processing_info" dynamodbav:"processing_info"`
}

// EmbeddingMetadata contains metadata about the embedding model and process
type EmbeddingMetadata struct {
	ModelName      string `json:"model_name" dynamodbav:"model_name"`
	ModelVersion   string `json:"model_version" dynamodbav:"model_version"`
	Dimension      int    `json:"dimension" dynamodbav:"dimension"`
	TextLength     int    `json:"text_length" dynamodbav:"text_length"`
	Preprocessing  string `json:"preprocessing" dynamodbav:"preprocessing"`
}

// SourceText contains information about the source text used for vectorization
type SourceText struct {
	Content      string   `json:"content" dynamodbav:"content"`
	SourceFields []string `json:"source_fields" dynamodbav:"source_fields"`
	Language     string   `json:"language" dynamodbav:"language"`
}

// ProcessingInfo contains information about the processing context
type ProcessingInfo struct {
	CreatedAt        string `json:"created_at" dynamodbav:"created_at"`
	TraceID          string `json:"trace_id" dynamodbav:"trace_id"`
	ProcessingTimeMs int64  `json:"processing_time_ms" dynamodbav:"processing_time_ms"`
}

// VectorStorage handles storing vector records in DynamoDB
type VectorStorage struct {
	client    dynamodbiface.DynamoDBAPI
	tableName string
	logger    *logger.Logger
}

// BatchWriteResult contains the results of a batch write operation
type BatchWriteResult struct {
	SuccessCount int
	FailedItems  []VectorRecord
	Errors       []error
}

// NewVectorStorage creates a new vector storage instance
func NewVectorStorage(tableName string) *VectorStorage {
	sess := session.Must(session.NewSession())
	return &VectorStorage{
		client:    dynamodb.New(sess),
		tableName: tableName,
		logger:    logger.New("vector-storage"),
	}
}

// NewVectorStorageWithClient creates a new vector storage with custom client (for testing)
func NewVectorStorageWithClient(client dynamodbiface.DynamoDBAPI, tableName string) *VectorStorage {
	return &VectorStorage{
		client:    client,
		tableName: tableName,
		logger:    logger.New("vector-storage"),
	}
}

// CreateVectorRecord creates a VectorRecord from embedding data
func CreateVectorRecord(paperID, text, traceID string, embedding []float64, modelVersion string, processingTimeMs int64) *VectorRecord {
	now := time.Now().UTC().Format(time.RFC3339)
	
	return &VectorRecord{
		PaperID:    paperID,
		VectorType: "title_abstract", // Default vector type for title+abstract combination
		Embedding:  embedding,
		EmbeddingMetadata: EmbeddingMetadata{
			ModelName:     extractModelName(modelVersion),
			ModelVersion:  modelVersion,
			Dimension:     len(embedding),
			TextLength:    len(text),
			Preprocessing: "title_abstract_combination",
		},
		SourceText: SourceText{
			Content:      text,
			SourceFields: []string{"title", "abstract"},
			Language:     "en", // Default to English, could be detected in future
		},
		ProcessingInfo: ProcessingInfo{
			CreatedAt:        now,
			TraceID:          traceID,
			ProcessingTimeMs: processingTimeMs,
		},
	}
}

// extractModelName extracts the base model name from the full model version string
func extractModelName(modelVersion string) string {
	// For now, return the full version as the name
	// In the future, this could parse more complex version strings
	return modelVersion
}



// BatchStoreVectors stores multiple vector records in batches
func (s *VectorStorage) BatchStoreVectors(ctx context.Context, records []VectorRecord) (*BatchWriteResult, error) {
	if len(records) == 0 {
		return &BatchWriteResult{}, nil
	}

	contextLogger := s.logger.WithContext(ctx)
	contextLogger.InfoWithCount("Starting batch vector storage", len(records))

	result := &BatchWriteResult{
		SuccessCount: 0,
		FailedItems:  []VectorRecord{},
		Errors:       []error{},
	}

	// Process records in batches of 25 (DynamoDB limit)
	const batchSize = 25
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		batchResult, err := s.processBatch(ctx, batch)
		if err != nil {
			contextLogger.Error("Batch processing failed", err, map[string]interface{}{
				"batch_start": i,
				"batch_size":  len(batch),
			})
			result.Errors = append(result.Errors, err)
			result.FailedItems = append(result.FailedItems, batch...)
			continue
		}

		result.SuccessCount += batchResult.SuccessCount
		result.FailedItems = append(result.FailedItems, batchResult.FailedItems...)
		result.Errors = append(result.Errors, batchResult.Errors...)
	}

	contextLogger.InfoWithCount("Completed batch vector storage", result.SuccessCount, map[string]interface{}{
		"total_records":  len(records),
		"success_count":  result.SuccessCount,
		"failed_count":   len(result.FailedItems),
		"error_count":    len(result.Errors),
	})

	return result, nil
}

// processBatch processes a single batch of vector records
func (s *VectorStorage) processBatch(ctx context.Context, records []VectorRecord) (*BatchWriteResult, error) {
	contextLogger := s.logger.WithContext(ctx)
	
	result := &BatchWriteResult{
		SuccessCount: 0,
		FailedItems:  []VectorRecord{},
		Errors:       []error{},
	}

	// Validate records before processing
	validRecords := make([]VectorRecord, 0, len(records))
	for i, record := range records {
		if err := s.validateVectorRecord(&record); err != nil {
			contextLogger.Warn("Invalid vector record found", map[string]interface{}{
				"record_index": i,
				"paper_id":     record.PaperID,
				"error":        err.Error(),
			})
			result.FailedItems = append(result.FailedItems, record)
			result.Errors = append(result.Errors, fmt.Errorf("invalid record %s: %w", record.PaperID, err))
			continue
		}
		validRecords = append(validRecords, record)
	}

	if len(validRecords) == 0 {
		contextLogger.Warn("No valid records to process in batch")
		return result, nil
	}

	// Prepare batch write request
	writeRequests := make([]*dynamodb.WriteRequest, 0, len(validRecords))
	
	for _, record := range validRecords {
		item, err := dynamodbattribute.MarshalMap(record)
		if err != nil {
			contextLogger.Error("Failed to marshal record in batch", err, map[string]interface{}{
				"paper_id": record.PaperID,
			})
			result.FailedItems = append(result.FailedItems, record)
			result.Errors = append(result.Errors, fmt.Errorf("failed to marshal record %s: %w", record.PaperID, err))
			continue
		}

		writeRequest := &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: item,
			},
		}
		writeRequests = append(writeRequests, writeRequest)
	}

	if len(writeRequests) == 0 {
		contextLogger.Warn("No valid write requests to process")
		return result, nil
	}

	// Execute batch write (single attempt, let Step Function handle retries)
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			s.tableName: writeRequests,
		},
	}

	startTime := time.Now()
	output, err := s.client.BatchWriteItemWithContext(ctx, input)
	duration := time.Since(startTime)

	contextLogger.Debug("Batch write completed", map[string]interface{}{
		"items_requested":   len(writeRequests),
		"duration_ms":       duration.Milliseconds(),
		"consumed_capacity": output.ConsumedCapacity,
	})

	if err != nil {
		contextLogger.Error("Batch write failed", err, map[string]interface{}{
			"items_requested": len(writeRequests),
		})
		// Add all items to failed items
		result.FailedItems = append(result.FailedItems, validRecords...)
		result.Errors = append(result.Errors, fmt.Errorf("batch write failed: %w", err))
		return result, nil
	}

	// Calculate success count
	totalRequested := len(writeRequests)
	unprocessedCount := 0
	if output.UnprocessedItems != nil {
		if unprocessedItems, exists := output.UnprocessedItems[s.tableName]; exists {
			unprocessedCount = len(unprocessedItems)
		}
	}
	result.SuccessCount = totalRequested - unprocessedCount

	// Handle unprocessed items (add to failed items for Step Function to retry)
	if unprocessedCount > 0 {
		contextLogger.Warn("Some items were not processed", map[string]interface{}{
			"unprocessed_count": unprocessedCount,
			"total_count":       totalRequested,
		})
		
		// Add unprocessed items to failed items (approximate mapping)
		startIndex := result.SuccessCount
		for i := 0; i < unprocessedCount && startIndex+i < len(validRecords); i++ {
			result.FailedItems = append(result.FailedItems, validRecords[startIndex+i])
		}
	}

	contextLogger.InfoWithDuration("Batch write completed", duration, map[string]interface{}{
		"total_records":     len(records),
		"valid_records":     len(validRecords),
		"success_count":     result.SuccessCount,
		"unprocessed_count": unprocessedCount,
		"failed_count":      len(result.FailedItems),
	})

	return result, nil
}

// validateVectorRecord validates the structure and content of a vector record
func (s *VectorStorage) validateVectorRecord(record *VectorRecord) error {
	if record.PaperID == "" {
		return fmt.Errorf("paper_id is empty")
	}
	
	if record.VectorType == "" {
		return fmt.Errorf("vector_type is empty")
	}
	
	if len(record.Embedding) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}
	
	if record.EmbeddingMetadata.Dimension != len(record.Embedding) {
		return fmt.Errorf("dimension mismatch: metadata says %d, embedding has %d", 
			record.EmbeddingMetadata.Dimension, len(record.Embedding))
	}
	
	if record.EmbeddingMetadata.ModelVersion == "" {
		return fmt.Errorf("model_version is empty")
	}
	
	if record.ProcessingInfo.TraceID == "" {
		return fmt.Errorf("trace_id is empty")
	}
	
	// Validate embedding values
	for i, val := range record.Embedding {
		if val != val { // Check for NaN
			return fmt.Errorf("embedding contains NaN at index %d", i)
		}
	}
	
	return nil
}

