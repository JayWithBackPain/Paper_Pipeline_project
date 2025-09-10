package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"shared/logger"
	"vector-coordinator/client"
	"vector-coordinator/retriever"
	"vector-coordinator/storage"
)

type StepFunctionInput struct {
	TraceID string `json:"trace_id"`
}

// DataRetrieverInterface defines the interface for data retrieval
type DataRetrieverInterface interface {
	GetCombinedTextsByTraceID(ctx context.Context, traceID string) ([]retriever.CombinedText, error)
}

// VectorAPIClientInterface defines the interface for vector API client
type VectorAPIClientInterface interface {
	GenerateEmbedding(ctx context.Context, text string) (*client.EmbeddingResponse, error)
}

// VectorStorageInterface defines the interface for vector storage
type VectorStorageInterface interface {
	BatchStoreVectors(ctx context.Context, records []storage.VectorRecord) (*storage.BatchWriteResult, error)
}

type VectorCoordinator struct {
	retriever     DataRetrieverInterface
	apiClient     VectorAPIClientInterface
	vectorStorage VectorStorageInterface
	logger        *logger.Logger
}

// ProcessingStatus represents the status of vectorization processing
type ProcessingStatus string

const (
	StatusStarted    ProcessingStatus = "started"
	StatusInProgress ProcessingStatus = "in_progress"
	StatusCompleted  ProcessingStatus = "completed"
	StatusFailed     ProcessingStatus = "failed"
	StatusPartial    ProcessingStatus = "partial_success"
)

// ProcessingResult represents the result of vectorization processing
type ProcessingResult struct {
	TraceID           string           `json:"trace_id"`
	Status            ProcessingStatus `json:"status"`
	TotalPapers       int              `json:"total_papers"`
	EmbeddingsGenerated int            `json:"embeddings_generated"`
	VectorsStored     int              `json:"vectors_stored"`
	FailedEmbeddings  int              `json:"failed_embeddings"`
	FailedStorage     int              `json:"failed_storage"`
	ProcessingTimeMs  int64            `json:"processing_time_ms"`
	ErrorMessage      string           `json:"error_message,omitempty"`
	Timestamp         string           `json:"timestamp"`
}

// ProcessingError represents a structured error with context
type ProcessingError struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *ProcessingError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Stage, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Stage, e.Message)
}

func (e *ProcessingError) Unwrap() error {
	return e.Cause
}

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handleStepFunction)
	} else {
		fmt.Println("Vector Coordinator Service - Local Development Mode")
	}
}

func handleStepFunction(ctx context.Context, input StepFunctionInput) (*ProcessingResult, error) {
	// Initialize components
	papersTableName := getEnvOrDefault("PAPERS_TABLE_NAME", "papers-table")
	indexName := getEnvOrDefault("TRACE_ID_INDEX_NAME", "trace-id-index")
	vectorsTableName := getEnvOrDefault("VECTORS_TABLE_NAME", "vectors-table")
	embeddingAPIURL := getEnvOrDefault("EMBEDDING_API_URL", "https://embedding-api.example.com/embed")
	
	coordinator := &VectorCoordinator{
		retriever:     retriever.NewDataRetriever(papersTableName, indexName),
		apiClient:     client.NewVectorAPIClient(embeddingAPIURL),
		vectorStorage: storage.NewVectorStorage(vectorsTableName),
		logger:        logger.New("vector-coordinator"),
	}
	
	result, err := coordinator.processVectorization(ctx, input.TraceID)
	if err != nil {
		// Return both result (for partial success) and error
		return result, err
	}
	
	return result, nil
}

func (vc *VectorCoordinator) processVectorization(ctx context.Context, traceID string) (*ProcessingResult, error) {
	startTime := time.Now()
	contextLogger := vc.logger.WithContext(ctx).WithTraceID(traceID)
	
	// Initialize result tracking
	result := &ProcessingResult{
		TraceID:   traceID,
		Status:    StatusStarted,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	contextLogger.Info("Starting vectorization processing", map[string]interface{}{
		"status": result.Status,
	})
	
	// Validate input
	if traceID == "" {
		err := &ProcessingError{
			Stage:   "validation",
			Message: "traceID cannot be empty",
		}
		result.Status = StatusFailed
		result.ErrorMessage = err.Error()
		contextLogger.Error("Input validation failed", err)
		return result, err
	}
	
	// Update status to in progress
	result.Status = StatusInProgress
	contextLogger.Info("Retrieving papers for vectorization", map[string]interface{}{
		"status": result.Status,
	})
	
	// Retrieve papers and combine text with error handling
	combinedTexts, err := vc.retriever.GetCombinedTextsByTraceID(ctx, traceID)
	if err != nil {
		processingErr := &ProcessingError{
			Stage:   "data_retrieval",
			Message: "failed to retrieve papers for vectorization",
			Cause:   err,
		}
		result.Status = StatusFailed
		result.ErrorMessage = processingErr.Error()
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		contextLogger.Error("Failed to retrieve and combine texts", processingErr)
		return result, processingErr
	}
	
	result.TotalPapers = len(combinedTexts)
	contextLogger.InfoWithCount("Retrieved papers for vectorization", result.TotalPapers, map[string]interface{}{
		"status": result.Status,
	})
	
	// Handle case where no papers are found
	if result.TotalPapers == 0 {
		result.Status = StatusCompleted
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		contextLogger.Info("No papers found for vectorization - processing completed", map[string]interface{}{
			"status": result.Status,
			"processing_time_ms": result.ProcessingTimeMs,
		})
		return result, nil
	}
	
	// Generate embeddings with progress tracking and error handling
	contextLogger.Info("Starting embedding generation", map[string]interface{}{
		"total_papers": result.TotalPapers,
		"status": result.Status,
	})
	
	vectorRecords := make([]storage.VectorRecord, 0, len(combinedTexts))
	embeddingErrors := make([]error, 0)
	
	for i, combinedText := range combinedTexts {
		embeddingStartTime := time.Now()
		
		// Log progress every 10 papers or at the end
		if (i+1)%10 == 0 || i == len(combinedTexts)-1 {
			contextLogger.Info("Embedding generation progress", map[string]interface{}{
				"processed": i + 1,
				"total":     len(combinedTexts),
				"progress_percent": float64(i+1) / float64(len(combinedTexts)) * 100,
			})
		}
		
		// Generate embedding using the API client with error handling
		embeddingResponse, err := vc.apiClient.GenerateEmbedding(ctx, combinedText.Text)
		if err != nil {
			embeddingErr := &ProcessingError{
				Stage:   "embedding_generation",
				Message: fmt.Sprintf("failed to generate embedding for paper %s", combinedText.PaperID),
				Cause:   err,
			}
			embeddingErrors = append(embeddingErrors, embeddingErr)
			result.FailedEmbeddings++
			
			contextLogger.Error("Failed to generate embedding", embeddingErr, map[string]interface{}{
				"paper_id": combinedText.PaperID,
				"progress": fmt.Sprintf("%d/%d", i+1, len(combinedTexts)),
			})
			// Continue with other papers instead of failing the entire batch
			continue
		}
		
		processingTimeMs := time.Since(embeddingStartTime).Milliseconds()
		
		// Create vector record
		vectorRecord := storage.CreateVectorRecord(
			combinedText.PaperID,
			combinedText.Text,
			traceID,
			embeddingResponse.Embedding,
			embeddingResponse.ModelVersion,
			processingTimeMs,
		)
		
		vectorRecords = append(vectorRecords, *vectorRecord)
		result.EmbeddingsGenerated++
		
		contextLogger.Debug("Generated embedding", map[string]interface{}{
			"paper_id":            combinedText.PaperID,
			"embedding_dimension": embeddingResponse.Dimension,
			"processing_time_ms":  processingTimeMs,
			"progress":            fmt.Sprintf("%d/%d", i+1, len(combinedTexts)),
		})
	}
	
	contextLogger.InfoWithCount("Completed embedding generation", result.EmbeddingsGenerated, map[string]interface{}{
		"total_papers":       result.TotalPapers,
		"successful_embeddings": result.EmbeddingsGenerated,
		"failed_embeddings":  result.FailedEmbeddings,
		"success_rate":       float64(result.EmbeddingsGenerated) / float64(result.TotalPapers) * 100,
	})
	
	// Check if we have any embeddings to store
	if len(vectorRecords) == 0 {
		processingErr := &ProcessingError{
			Stage:   "embedding_generation",
			Message: "no embeddings were generated successfully",
		}
		result.Status = StatusFailed
		result.ErrorMessage = processingErr.Error()
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		contextLogger.Error("No embeddings generated", processingErr, map[string]interface{}{
			"total_papers": result.TotalPapers,
			"failed_embeddings": result.FailedEmbeddings,
		})
		return result, processingErr
	}
	
	// Store vector records in batch with progress tracking
	contextLogger.InfoWithCount("Starting vector storage", len(vectorRecords))
	batchResult, err := vc.vectorStorage.BatchStoreVectors(ctx, vectorRecords)
	if err != nil {
		processingErr := &ProcessingError{
			Stage:   "vector_storage",
			Message: "failed to store vector records",
			Cause:   err,
		}
		result.Status = StatusFailed
		result.ErrorMessage = processingErr.Error()
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		contextLogger.Error("Failed to store vector records", processingErr)
		return result, processingErr
	}
	
	// Update result with storage statistics
	result.VectorsStored = batchResult.SuccessCount
	result.FailedStorage = len(batchResult.FailedItems)
	result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
	
	// Determine final status based on success/failure rates
	if result.FailedEmbeddings == 0 && result.FailedStorage == 0 {
		result.Status = StatusCompleted
	} else if result.VectorsStored > 0 {
		result.Status = StatusPartial
	} else {
		result.Status = StatusFailed
		result.ErrorMessage = "all vectorization operations failed"
	}
	
	// Log comprehensive final results
	contextLogger.InfoWithCount("Vectorization processing completed", result.VectorsStored, map[string]interface{}{
		"status":               result.Status,
		"total_papers":         result.TotalPapers,
		"embeddings_generated": result.EmbeddingsGenerated,
		"vectors_stored":       result.VectorsStored,
		"failed_embeddings":    result.FailedEmbeddings,
		"failed_storage":       result.FailedStorage,
		"processing_time_ms":   result.ProcessingTimeMs,
		"embedding_success_rate": float64(result.EmbeddingsGenerated) / float64(result.TotalPapers) * 100,
		"storage_success_rate":   float64(result.VectorsStored) / float64(result.EmbeddingsGenerated) * 100,
		"overall_success_rate":   float64(result.VectorsStored) / float64(result.TotalPapers) * 100,
	})
	
	// Log warnings for partial failures
	if result.FailedEmbeddings > 0 {
		contextLogger.Warn("Some embeddings failed to generate", map[string]interface{}{
			"failed_count": result.FailedEmbeddings,
			"total_count":  result.TotalPapers,
		})
	}
	
	if result.FailedStorage > 0 {
		contextLogger.Warn("Some vector records failed to store", map[string]interface{}{
			"failed_count": result.FailedStorage,
			"total_count":  result.EmbeddingsGenerated,
		})
		
		// Log details of storage errors
		for i, storageErr := range batchResult.Errors {
			contextLogger.Error("Storage error detail", storageErr, map[string]interface{}{
				"error_index": i + 1,
				"total_errors": len(batchResult.Errors),
			})
		}
	}
	
	// Log system metrics for monitoring
	vc.logSystemMetrics(ctx, result)
	
	// Return error for failures (Step Function will handle retries)
	if result.Status == StatusFailed {
		return result, &ProcessingError{
			Stage:   "overall_processing",
			Message: fmt.Sprintf("vectorization failed for traceID %s: %s", traceID, result.ErrorMessage),
		}
	}
	
	// Also return error for partial failures to let Step Function decide on retry
	if result.Status == StatusPartial {
		return result, &ProcessingError{
			Stage:   "partial_processing",
			Message: fmt.Sprintf("partial vectorization failure for traceID %s: %d/%d papers processed successfully", 
				traceID, result.VectorsStored, result.TotalPapers),
		}
	}
	
	return result, nil
}



// logSystemMetrics logs system-level metrics for monitoring
func (vc *VectorCoordinator) logSystemMetrics(ctx context.Context, result *ProcessingResult) {
	contextLogger := vc.logger.WithContext(ctx).WithTraceID(result.TraceID)
	
	// Log processing metrics
	contextLogger.Info("Processing metrics", map[string]interface{}{
		"metric_type":          "processing_summary",
		"trace_id":             result.TraceID,
		"total_papers":         result.TotalPapers,
		"embeddings_generated": result.EmbeddingsGenerated,
		"vectors_stored":       result.VectorsStored,
		"failed_embeddings":    result.FailedEmbeddings,
		"failed_storage":       result.FailedStorage,
		"processing_time_ms":   result.ProcessingTimeMs,
		"status":               result.Status,
	})
	
	// Log success rates as metrics
	if result.TotalPapers > 0 {
		embeddingSuccessRate := float64(result.EmbeddingsGenerated) / float64(result.TotalPapers) * 100
		contextLogger.Info("Embedding success rate", map[string]interface{}{
			"metric_type":            "success_rate",
			"metric_name":            "embedding_success_rate",
			"value":                  embeddingSuccessRate,
			"total_papers":           result.TotalPapers,
			"successful_embeddings":  result.EmbeddingsGenerated,
		})
	}
	
	if result.EmbeddingsGenerated > 0 {
		storageSuccessRate := float64(result.VectorsStored) / float64(result.EmbeddingsGenerated) * 100
		contextLogger.Info("Storage success rate", map[string]interface{}{
			"metric_type":           "success_rate",
			"metric_name":           "storage_success_rate",
			"value":                 storageSuccessRate,
			"total_embeddings":      result.EmbeddingsGenerated,
			"successful_storage":    result.VectorsStored,
		})
	}
	
	// Log overall success rate
	if result.TotalPapers > 0 {
		overallSuccessRate := float64(result.VectorsStored) / float64(result.TotalPapers) * 100
		contextLogger.Info("Overall success rate", map[string]interface{}{
			"metric_type":        "success_rate",
			"metric_name":        "overall_success_rate",
			"value":              overallSuccessRate,
			"total_papers":       result.TotalPapers,
			"successful_vectors": result.VectorsStored,
		})
	}
	
	// Log processing throughput
	if result.ProcessingTimeMs > 0 {
		throughputPerSecond := float64(result.VectorsStored) / (float64(result.ProcessingTimeMs) / 1000.0)
		contextLogger.Info("Processing throughput", map[string]interface{}{
			"metric_type":           "throughput",
			"metric_name":           "vectors_per_second",
			"value":                 throughputPerSecond,
			"vectors_stored":        result.VectorsStored,
			"processing_time_ms":    result.ProcessingTimeMs,
		})
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}