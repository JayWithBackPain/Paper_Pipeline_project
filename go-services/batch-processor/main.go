package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"batch-processor/deduplicator"
	"batch-processor/dynamodb"
	"batch-processor/processor"
	"batch-processor/s3"
	"shared/logger"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handleS3Event)
	} else {
		fmt.Println("Batch Processor Service - Local Development Mode")
	}
}

func handleS3Event(ctx context.Context, s3Event events.S3Event) (*processor.ProcessResult, error) {
	// Create shared logger
	appLogger := logger.New("batch-processor")
	contextLogger := appLogger.WithContext(ctx)
	
	contextLogger.InfoWithCount("Processing S3 records", len(s3Event.Records))
	
	// Create S3 downloader
	downloader := s3.NewDownloader()
	
	// Create deduplicator
	dedup := deduplicator.NewDeduplicator()
	
	// Create DynamoDB writer (table name from environment variable)
	tableName := os.Getenv("PAPERS_TABLE_NAME")
	if tableName == "" {
		tableName = "Papers" // Default table name
	}
	dynamoWriter := dynamodb.NewWriter(tableName)
	
	// Create processor
	eventProcessor := processor.NewS3EventProcessor(downloader, dedup, dynamoWriter, contextLogger)
	
	// Process the S3 event
	result, err := eventProcessor.ProcessS3Event(ctx, s3Event)
	if err != nil {
		contextLogger.Error("Error processing S3 event", err)
		return nil, err
	}
	
	// Log the result
	resultJSON, _ := json.Marshal(result)
	contextLogger.Info("Processing completed successfully", map[string]interface{}{
		"result": string(resultJSON),
	})
	
	return result, nil
}