package retriever

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"shared/logger"
)

// Paper represents a research paper record from DynamoDB
type Paper struct {
	PaperID       string   `json:"paper_id" dynamodbav:"paper_id"`
	Source        string   `json:"source" dynamodbav:"source"`
	Title         string   `json:"title" dynamodbav:"title"`
	Abstract      string   `json:"abstract" dynamodbav:"abstract"`
	Authors       []string `json:"authors" dynamodbav:"authors"`
	PublishedDate string   `json:"published_date" dynamodbav:"published_date"`
	Categories    []string `json:"categories" dynamodbav:"categories"`
	TraceID       string   `json:"trace_id" dynamodbav:"trace_id"`
	BatchTimestamp string  `json:"batch_timestamp" dynamodbav:"batch_timestamp"`
}

// CombinedText represents the combined title and abstract for vectorization
type CombinedText struct {
	PaperID string `json:"paper_id"`
	Text    string `json:"text"`
}

// DataRetriever handles retrieving papers from DynamoDB by traceID
type DataRetriever struct {
	client    dynamodbiface.DynamoDBAPI
	tableName string
	indexName string
	logger    *logger.Logger
}

// NewDataRetriever creates a new data retriever instance
func NewDataRetriever(tableName, indexName string) *DataRetriever {
	sess := session.Must(session.NewSession())
	return &DataRetriever{
		client:    dynamodb.New(sess),
		tableName: tableName,
		indexName: indexName,
		logger:    logger.New("data-retriever"),
	}
}

// NewDataRetrieverWithClient creates a new data retriever with custom client (for testing)
func NewDataRetrieverWithClient(client dynamodbiface.DynamoDBAPI, tableName, indexName string) *DataRetriever {
	return &DataRetriever{
		client:    client,
		tableName: tableName,
		indexName: indexName,
		logger:    logger.New("data-retriever"),
	}
}



// validatePaper validates the structure and content of a paper record
func (r *DataRetriever) validatePaper(paper *Paper) error {
	if paper.PaperID == "" {
		return fmt.Errorf("paper_id is empty")
	}
	
	if paper.TraceID == "" {
		return fmt.Errorf("trace_id is empty")
	}
	
	if paper.Title == "" && paper.Abstract == "" {
		return fmt.Errorf("both title and abstract are empty")
	}
	
	return nil
}



// GetCombinedTextsByTraceID retrieves papers by traceID and returns combined text for vectorization
func (r *DataRetriever) GetCombinedTextsByTraceID(ctx context.Context, traceID string) ([]CombinedText, error) {
	if traceID == "" {
		return nil, fmt.Errorf("traceID cannot be empty")
	}

	contextLogger := r.logger.WithContext(ctx).WithTraceID(traceID)
	startTime := time.Now()
	
	contextLogger.Info("Starting paper retrieval by traceID", map[string]interface{}{
		"table_name": r.tableName,
		"index_name": r.indexName,
	})

	var allPapers []Paper
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue
	pageCount := 0
	maxPages := 100 // Prevent infinite loops

	// Query with pagination support and error handling
	for pageCount < maxPages {
		pageCount++
		
		input := &dynamodb.QueryInput{
			TableName: aws.String(r.tableName),
			IndexName: aws.String(r.indexName),
			KeyConditionExpression: aws.String("trace_id = :trace_id"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":trace_id": {
					S: aws.String(traceID),
				},
			},
			// Sort by batch_timestamp in descending order (newest first)
			ScanIndexForward: aws.Bool(false),
		}

		// Add pagination token if available
		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		queryStartTime := time.Now()
		result, err := r.client.QueryWithContext(ctx, input)
		queryDuration := time.Since(queryStartTime)
		
		if err != nil {
			contextLogger.Error("Failed to query papers by traceID", err, map[string]interface{}{
				"table_name":     r.tableName,
				"index_name":     r.indexName,
				"page_number":    pageCount,
				"query_duration_ms": queryDuration.Milliseconds(),
			})
			return nil, fmt.Errorf("failed to query papers by traceID on page %d: %w", pageCount, err)
		}

		// Log query performance metrics
		contextLogger.Debug("DynamoDB query completed", map[string]interface{}{
			"page_number":       pageCount,
			"items_returned":    len(result.Items),
			"query_duration_ms": queryDuration.Milliseconds(),
			"consumed_capacity": result.ConsumedCapacity,
		})

		// Convert DynamoDB items to Paper structs with error handling
		var papers []Paper
		if err := dynamodbattribute.UnmarshalListOfMaps(result.Items, &papers); err != nil {
			contextLogger.Error("Failed to unmarshal papers", err, map[string]interface{}{
				"item_count":  len(result.Items),
				"page_number": pageCount,
			})
			return nil, fmt.Errorf("failed to unmarshal papers on page %d: %w", pageCount, err)
		}

		// Validate paper data
		validPapers := make([]Paper, 0, len(papers))
		for i, paper := range papers {
			if err := r.validatePaper(&paper); err != nil {
				contextLogger.Warn("Invalid paper data found", map[string]interface{}{
					"paper_index": i,
					"paper_id":    paper.PaperID,
					"error":       err.Error(),
				})
				continue
			}
			validPapers = append(validPapers, paper)
		}

		allPapers = append(allPapers, validPapers...)

		contextLogger.Info("Retrieved paper batch", map[string]interface{}{
			"page_number":       pageCount,
			"batch_size":        len(papers),
			"valid_papers":      len(validPapers),
			"invalid_papers":    len(papers) - len(validPapers),
			"total_so_far":      len(allPapers),
			"has_more":          result.LastEvaluatedKey != nil,
			"query_duration_ms": queryDuration.Milliseconds(),
		})

		// Check if there are more items to retrieve
		if result.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	// Check if we hit the page limit
	if pageCount >= maxPages {
		contextLogger.Warn("Hit maximum page limit during retrieval", map[string]interface{}{
			"max_pages":     maxPages,
			"papers_found":  len(allPapers),
		})
	}

	totalDuration := time.Since(startTime)
	contextLogger.InfoWithDuration("Completed paper retrieval by traceID", totalDuration, map[string]interface{}{
		"total_papers":      len(allPapers),
		"pages_processed":   pageCount,
		"avg_query_time_ms": totalDuration.Milliseconds() / int64(pageCount),
	})

	// Combine title and abstract text for vectorization
	if len(allPapers) == 0 {
		return nil, nil
	}

	contextLogger.InfoWithCount("Starting text combination", len(allPapers))

	var combinedTexts []CombinedText
	for _, paper := range allPapers {
		// Skip papers without title or abstract
		if paper.Title == "" && paper.Abstract == "" {
			contextLogger.Warn("Skipping paper with empty title and abstract", map[string]interface{}{
				"paper_id": paper.PaperID,
			})
			continue
		}

		// Combine title and abstract with proper formatting
		var textParts []string
		if paper.Title != "" {
			textParts = append(textParts, strings.TrimSpace(paper.Title))
		}
		if paper.Abstract != "" {
			textParts = append(textParts, strings.TrimSpace(paper.Abstract))
		}

		combinedText := CombinedText{
			PaperID: paper.PaperID,
			Text:    strings.Join(textParts, ". "),
		}

		combinedTexts = append(combinedTexts, combinedText)
	}

	contextLogger.InfoWithCount("Completed text combination", len(combinedTexts), map[string]interface{}{
		"original_count": len(allPapers),
		"valid_count":    len(combinedTexts),
		"skipped_count":  len(allPapers) - len(combinedTexts),
	})

	return combinedTexts, nil
}