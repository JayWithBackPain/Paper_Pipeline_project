package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
	"shared/logger"
)

// Paper represents a research paper record
type Paper struct {
	PaperID       string    `json:"paper_id"`
	Source        string    `json:"source"`
	Title         string    `json:"title"`
	Abstract      string    `json:"abstract"`
	Authors       []string  `json:"authors"`
	PublishedDate string    `json:"published_date"`
	Categories    []string  `json:"categories"`
	RawXML        string    `json:"raw_xml,omitempty"`
	TraceID       string    `json:"trace_id"`
	BatchTimestamp string   `json:"batch_timestamp"`
	ProcessingStatus string `json:"processing_status"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
}

// ProcessResult represents the result of batch processing
type ProcessResult struct {
	TraceID            string              `json:"trace_id"`
	ProcessedCount     int                 `json:"processed_count"`
	Timestamp          time.Time           `json:"timestamp"`
	Status             string              `json:"status"`
	ErrorMessage       string              `json:"error_message,omitempty"`
	DeduplicationStats *DeduplicationStats `json:"deduplication_stats,omitempty"`
	UpsertStats        *UpsertStats        `json:"upsert_stats,omitempty"`
}

// S3EventProcessor handles S3 event processing
type S3EventProcessor struct {
	downloader    S3Downloader
	deduplicator  Deduplicator
	dynamoWriter  DynamoWriter
	logger        Logger
}

// Logger interface for structured logging - using shared logger
type Logger interface {
	WithTraceID(traceID string) *logger.Logger
	Info(message string, metadata ...map[string]interface{})
	InfoWithCount(message string, count int, metadata ...map[string]interface{})
	InfoWithDuration(message string, duration time.Duration, metadata ...map[string]interface{})
	Warn(message string, metadata ...map[string]interface{})
	Error(message string, err error, metadata ...map[string]interface{})
	Debug(message string, metadata ...map[string]interface{})
}

// S3Downloader interface for downloading and decompressing S3 files
type S3Downloader interface {
	DownloadAndDecompress(ctx context.Context, bucket, key string) ([]byte, error)
}

// Deduplicator interface for data deduplication
type Deduplicator interface {
	DeduplicateWithStats(papers []Paper) ([]Paper, DeduplicationStats)
}

// DynamoWriter interface for DynamoDB operations
type DynamoWriter interface {
	BatchUpsertWithStats(ctx context.Context, papers []Paper) (*UpsertStats, error)
}

// DeduplicationStats contains statistics about the deduplication process
type DeduplicationStats struct {
	OriginalCount  int `json:"original_count"`
	UniqueCount    int `json:"unique_count"`
	DuplicateCount int `json:"duplicate_count"`
	InvalidCount   int `json:"invalid_count"`
}

// UpsertStats contains statistics about the upsert operation
type UpsertStats struct {
	TotalItems     int `json:"total_items"`
	SuccessItems   int `json:"success_items"`
	FailedItems    int `json:"failed_items"`
	BatchCount     int `json:"batch_count"`
	SuccessBatches int `json:"success_batches"`
	FailedBatches  int `json:"failed_batches"`
}

// NewS3EventProcessor creates a new S3 event processor
func NewS3EventProcessor(downloader S3Downloader, deduplicator Deduplicator, dynamoWriter DynamoWriter, logger Logger) *S3EventProcessor {
	return &S3EventProcessor{
		downloader:   downloader,
		deduplicator: deduplicator,
		dynamoWriter: dynamoWriter,
		logger:       logger,
	}
}

// ProcessS3Event processes an S3 event and returns processing results
func (p *S3EventProcessor) ProcessS3Event(ctx context.Context, s3Event events.S3Event) (*ProcessResult, error) {
	if len(s3Event.Records) == 0 {
		return nil, fmt.Errorf("no S3 records to process")
	}

	// Generate trace ID for this batch
	traceID := uuid.New().String()
	batchTimestamp := time.Now()
	startTime := time.Now()
	
	// Log processing start
	tracedLogger := p.logger.WithTraceID(traceID)
	tracedLogger.InfoWithCount("Starting batch processing", len(s3Event.Records), map[string]interface{}{
		"event": "processing_start",
	})

	var allPapers []Paper
	var lastError error

	// Process each S3 record
	for _, record := range s3Event.Records {
		bucket := record.S3.Bucket.Name
		key := record.S3.Object.Key
		
		// Log S3 processing (file size is not available from S3 event, so we use 0)
		tracedLogger.Info("Processing S3 object", map[string]interface{}{
			"event":     "s3_processing",
			"bucket":    bucket,
			"key":       key,
			"file_size": 0,
		})

		// Download and decompress file
		data, err := p.downloader.DownloadAndDecompress(ctx, bucket, key)
		if err != nil {
			lastError = fmt.Errorf("failed to download/decompress %s/%s: %w", bucket, key, err)
			tracedLogger.Error("Error occurred during processing", lastError, map[string]interface{}{
				"event":      "error",
				"error_type": "s3_download",
				"context": map[string]interface{}{
					"bucket": bucket,
					"key":    key,
				},
			})
			continue
		}

		// Parse batch data
		papers, err := p.parseBatchData(data, traceID, batchTimestamp)
		if err != nil {
			lastError = fmt.Errorf("failed to parse batch data from %s/%s: %w", bucket, key, err)
			tracedLogger.Error("Error occurred during processing", lastError, map[string]interface{}{
				"event":      "error",
				"error_type": "data_parsing",
				"context": map[string]interface{}{
					"bucket":    bucket,
					"key":       key,
					"data_size": len(data),
				},
			})
			continue
		}

		// Log data parsing success
		tracedLogger.InfoWithCount("Data parsing completed", len(papers), map[string]interface{}{
			"event":  "data_parsing",
			"source": "s3_batch",
		})
		allPapers = append(allPapers, papers...)
	}

	// Initialize result with default values
	result := &ProcessResult{
		TraceID:   traceID,
		Timestamp: batchTimestamp,
		Status:    "success",
	}

	// Deduplicate papers
	if len(allPapers) > 0 {
		uniquePapers, dedupStats := p.deduplicator.DeduplicateWithStats(allPapers)
		result.DeduplicationStats = &dedupStats
		
		// Log deduplication results
		tracedLogger.Info("Deduplication completed", map[string]interface{}{
			"event":               "deduplication",
			"deduplication_stats": dedupStats,
		})
		
		// uniquePapers is already []Paper from the interface
		papers := uniquePapers
		
		// Upsert to DynamoDB
		if len(papers) > 0 {
			upsertStats, err := p.dynamoWriter.BatchUpsertWithStats(ctx, papers)
			if err != nil {
				lastError = fmt.Errorf("failed to upsert papers to DynamoDB: %w", err)
				tracedLogger.Error("Error occurred during processing", lastError, map[string]interface{}{
					"event":      "error",
					"error_type": "dynamodb_upsert",
					"context": map[string]interface{}{
						"paper_count": len(papers),
					},
				})
				result.Status = "failed"
				result.ErrorMessage = lastError.Error()
			} else {
				result.UpsertStats = upsertStats
				
				// Log DynamoDB upsert results
				tracedLogger.Info("DynamoDB upsert completed", map[string]interface{}{
					"event":        "dynamodb_upsert",
					"upsert_stats": upsertStats,
				})
				
				// Extract success count from upsert stats directly
				result.ProcessedCount = upsertStats.SuccessItems
				if upsertStats.FailedItems > 0 {
					result.Status = "partial_success"
					result.ErrorMessage = fmt.Sprintf("%d items failed to upsert", upsertStats.FailedItems)
				}
			}
		} else {
			tracedLogger.Warn("No unique papers to upsert after deduplication", map[string]interface{}{
				"event":        "warning",
				"warning_type": "no_unique_papers",
				"context": map[string]interface{}{
					"original_count": len(allPapers),
				},
			})
			result.ProcessedCount = 0
		}
	} else {
		tracedLogger.Warn("No papers parsed from S3 objects", map[string]interface{}{
			"event":        "warning",
			"warning_type": "no_papers_parsed",
			"context": map[string]interface{}{
				"record_count": len(s3Event.Records),
			},
		})
		result.ProcessedCount = 0
	}

	// Handle parsing errors
	if lastError != nil && result.ProcessedCount == 0 {
		result.Status = "failed"
		if result.ErrorMessage == "" {
			result.ErrorMessage = lastError.Error()
		}
	} else if lastError != nil {
		result.Status = "partial_success"
		if result.ErrorMessage == "" {
			result.ErrorMessage = lastError.Error()
		}
	}

	// Log performance metrics
	processingTime := time.Since(startTime)
	tracedLogger.Info("Performance metrics", map[string]interface{}{
		"event":   "metrics",
		"metrics": map[string]interface{}{
			"processing_time_ms": processingTime.Milliseconds(),
			"total_papers":       len(allPapers),
			"processed_count":    result.ProcessedCount,
			"throughput_per_sec": float64(result.ProcessedCount) / processingTime.Seconds(),
		},
	})

	// Log processing completion
	tracedLogger.Info("Batch processing completed", map[string]interface{}{
		"event":             "processing_complete",
		"processing_result": result,
	})

	return result, nil
}

// parseBatchData parses raw data into Paper structs
func (p *S3EventProcessor) parseBatchData(data []byte, traceID string, batchTimestamp time.Time) ([]Paper, error) {
	var papers []Paper
	
	// Try to parse as JSON array first
	var jsonPapers []map[string]interface{}
	if err := json.Unmarshal(data, &jsonPapers); err == nil {
		// Successfully parsed as JSON array
		for _, paperData := range jsonPapers {
			paper, err := p.convertMapToPaper(paperData, traceID, batchTimestamp)
			if err != nil {
				tracedLogger := p.logger.WithTraceID(traceID)
				tracedLogger.Warn("Failed to convert paper data", map[string]interface{}{
					"event":        "warning",
					"warning_type": "data_conversion",
					"context": map[string]interface{}{
						"error": err.Error(),
					},
				})
				continue
			}
			papers = append(papers, paper)
		}
		return papers, nil
	}

	// Try to parse as newline-delimited JSON
	lines := splitLines(string(data))
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		
		var paperData map[string]interface{}
		if err := json.Unmarshal([]byte(line), &paperData); err != nil {
			tracedLogger := p.logger.WithTraceID(traceID)
			tracedLogger.Warn("Failed to parse line as JSON", map[string]interface{}{
				"event":        "warning",
				"warning_type": "json_parsing",
				"context": map[string]interface{}{
					"line_number": i + 1,
					"error":       err.Error(),
				},
			})
			continue
		}
		
		paper, err := p.convertMapToPaper(paperData, traceID, batchTimestamp)
		if err != nil {
			tracedLogger := p.logger.WithTraceID(traceID)
			tracedLogger.Warn("Failed to convert paper data from line", map[string]interface{}{
				"event":        "warning",
				"warning_type": "data_conversion",
				"context": map[string]interface{}{
					"line_number": i + 1,
					"error":       err.Error(),
				},
			})
			continue
		}
		papers = append(papers, paper)
	}

	if len(papers) == 0 {
		return nil, fmt.Errorf("no valid papers found in data")
	}

	return papers, nil
}

// convertMapToPaper converts a map to Paper struct
func (p *S3EventProcessor) convertMapToPaper(data map[string]interface{}, traceID string, batchTimestamp time.Time) (Paper, error) {
	now := time.Now().Format(time.RFC3339)
	
	paper := Paper{
		TraceID:          traceID,
		BatchTimestamp:   batchTimestamp.Format(time.RFC3339),
		ProcessingStatus: "processed",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Extract paper_id (required)
	if id, ok := data["paper_id"].(string); ok && id != "" {
		paper.PaperID = id
	} else if id, ok := data["id"].(string); ok && id != "" {
		paper.PaperID = id
	} else {
		return paper, fmt.Errorf("missing or invalid paper_id")
	}

	// Extract other fields with defaults
	if source, ok := data["source"].(string); ok {
		paper.Source = source
	} else {
		paper.Source = "unknown"
	}

	if title, ok := data["title"].(string); ok {
		paper.Title = title
	}

	if abstract, ok := data["abstract"].(string); ok {
		paper.Abstract = abstract
	}

	// Handle authors array
	if authorsData, ok := data["authors"]; ok {
		if authorsArray, ok := authorsData.([]interface{}); ok {
			for _, author := range authorsArray {
				if authorStr, ok := author.(string); ok {
					paper.Authors = append(paper.Authors, authorStr)
				}
			}
		}
	}

	if publishedDate, ok := data["published_date"].(string); ok {
		paper.PublishedDate = publishedDate
	}

	// Handle categories array
	if categoriesData, ok := data["categories"]; ok {
		if categoriesArray, ok := categoriesData.([]interface{}); ok {
			for _, category := range categoriesArray {
				if categoryStr, ok := category.(string); ok {
					paper.Categories = append(paper.Categories, categoryStr)
				}
			}
		}
	}

	if rawXML, ok := data["raw_xml"].(string); ok {
		paper.RawXML = rawXML
	}

	return paper, nil
}

// splitLines splits text into lines, handling different line endings
func splitLines(text string) []string {
	// Replace \r\n with \n, then \r with \n
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}