package dynamodb

import (
	"batch-processor/processor"
	"context"
	"fmt"
	"shared/logger"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

const (
	// MaxBatchSize is the maximum number of items per batch write request
	MaxBatchSize = 25
)

// Writer handles DynamoDB write operations
type Writer struct {
	client    dynamodbiface.DynamoDBAPI
	tableName string
	logger    *logger.Logger
}

// NewWriter creates a new DynamoDB writer instance
func NewWriter(tableName string) *Writer {
	sess := session.Must(session.NewSession())
	return &Writer{
		client:    dynamodb.New(sess),
		tableName: tableName,
		logger:    logger.New("dynamodb-writer"),
	}
}

// NewWriterWithClient creates a new DynamoDB writer with custom client (for testing)
func NewWriterWithClient(client dynamodbiface.DynamoDBAPI, tableName string) *Writer {
	return &Writer{
		client:    client,
		tableName: tableName,
		logger:    logger.New("dynamodb-writer"),
	}
}

// BatchUpsert performs batch upsert operations on papers
func (w *Writer) BatchUpsert(ctx context.Context, papers []processor.Paper) error {
	if len(papers) == 0 {
		w.logger.Info("No papers to upsert")
		return nil
	}

	w.logger.InfoWithCount("Starting batch upsert", len(papers), map[string]interface{}{
		"table_name": w.tableName,
	})

	// Process papers in batches of MaxBatchSize
	for i := 0; i < len(papers); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(papers) {
			end = len(papers)
		}

		batch := papers[i:end]
		if err := w.processBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to process batch %d-%d: %w", i, end-1, err)
		}

		w.logger.Info("Successfully processed batch", map[string]interface{}{
			"batch_start": i,
			"batch_end":   end - 1,
			"batch_size":  len(batch),
		})
	}

	w.logger.InfoWithCount("Completed batch upsert", len(papers))
	return nil
}

// processBatch processes a single batch of papers
func (w *Writer) processBatch(ctx context.Context, papers []processor.Paper) error {
	if len(papers) == 0 {
		return nil
	}

	if len(papers) > MaxBatchSize {
		return fmt.Errorf("batch size %d exceeds maximum %d", len(papers), MaxBatchSize)
	}

	// Convert papers to DynamoDB write requests
	writeRequests := make([]*dynamodb.WriteRequest, 0, len(papers))

	for _, paper := range papers {
		// Convert paper to DynamoDB item
		item, err := dynamodbattribute.MarshalMap(paper)
		if err != nil {
			w.logger.Warn("Failed to marshal paper", map[string]interface{}{
				"paper_id": paper.PaperID,
				"error":    err.Error(),
			})
			continue
		}

		// Create put request (upsert)
		writeRequest := &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: item,
			},
		}

		writeRequests = append(writeRequests, writeRequest)
	}

	if len(writeRequests) == 0 {
		return fmt.Errorf("no valid write requests generated from batch")
	}

	// Execute batch write with retry logic
	return w.executeBatchWriteWithRetry(ctx, writeRequests)
}

// executeBatchWriteWithRetry executes batch write with retry for unprocessed items
func (w *Writer) executeBatchWriteWithRetry(ctx context.Context, writeRequests []*dynamodb.WriteRequest) error {
	maxRetries := 3
	currentRequests := writeRequests

	for attempt := 0; attempt < maxRetries && len(currentRequests) > 0; attempt++ {
		if attempt > 0 {
			w.logger.Info("Retrying batch write", map[string]interface{}{
				"attempt":         attempt + 1,
				"max_retries":     maxRetries,
				"items_remaining": len(currentRequests),
			})
		}

		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]*dynamodb.WriteRequest{
				w.tableName: currentRequests,
			},
		}

		result, err := w.client.BatchWriteItemWithContext(ctx, input)
		if err != nil {
			return fmt.Errorf("batch write failed on attempt %d: %w", attempt+1, err)
		}

		// Check for unprocessed items
		if unprocessedItems, exists := result.UnprocessedItems[w.tableName]; exists && len(unprocessedItems) > 0 {
			currentRequests = unprocessedItems
			w.logger.Info("Batch write partially succeeded", map[string]interface{}{
				"unprocessed_items": len(unprocessedItems),
			})
		} else {
			// All items processed successfully
			w.logger.Info("Batch write completed successfully")
			return nil
		}
	}

	// If we reach here, we still have unprocessed items after max retries
	return fmt.Errorf("failed to process %d items after %d retries", len(currentRequests), maxRetries)
}

// BatchUpsertWithStats performs batch upsert and returns statistics
func (w *Writer) BatchUpsertWithStats(ctx context.Context, papers []processor.Paper) (*processor.UpsertStats, error) {
	stats := &processor.UpsertStats{
		TotalItems:    len(papers),
		BatchCount:    (len(papers) + MaxBatchSize - 1) / MaxBatchSize, // Ceiling division
		SuccessItems:  0,
		FailedItems:   0,
	}

	if len(papers) == 0 {
		return stats, nil
	}

	w.logger.InfoWithCount("Starting batch upsert with stats tracking", len(papers))

	// Process papers in batches
	for i := 0; i < len(papers); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(papers) {
			end = len(papers)
		}

		batch := papers[i:end]
		if err := w.processBatch(ctx, batch); err != nil {
			w.logger.Error("Batch failed", err, map[string]interface{}{
				"batch_number": i/MaxBatchSize + 1,
			})
			stats.FailedItems += len(batch)
			stats.FailedBatches++
		} else {
			stats.SuccessItems += len(batch)
			stats.SuccessBatches++
		}
	}

	w.logger.Info("Batch upsert completed", map[string]interface{}{
		"success_items": stats.SuccessItems,
		"failed_items":  stats.FailedItems,
	})
	return stats, nil
}

