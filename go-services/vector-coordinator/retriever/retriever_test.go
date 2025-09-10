package retriever

import (
	"context"
	"errors"
	"testing"

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

func (m *MockDynamoDBClient) QueryWithContext(ctx context.Context, input *dynamodb.QueryInput, opts ...request.Option) (*dynamodb.QueryOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.QueryOutput), args.Error(1)
}

func createTestRetriever() (*DataRetriever, *MockDynamoDBClient) {
	mockClient := &MockDynamoDBClient{}
	retriever := NewDataRetrieverWithClient(mockClient, "test-table", "test-index")
	return retriever, mockClient
}

func TestGetPapersByTraceID_Success(t *testing.T) {
	retriever, mockClient := createTestRetriever()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Create mock response
	mockOutput := &dynamodb.QueryOutput{
		Items: []map[string]*dynamodb.AttributeValue{
			{
				"paper_id": {S: aws.String("paper1")},
				"trace_id": {S: aws.String(traceID)},
				"title":    {S: aws.String("Test Title 1")},
				"abstract": {S: aws.String("Test Abstract 1")},
				"source":   {S: aws.String("arxiv")},
				"authors":  {SS: []*string{aws.String("Author 1")}},
				"published_date": {S: aws.String("2023-01-01")},
				"categories":     {SS: []*string{aws.String("cs.AI")}},
				"batch_timestamp": {S: aws.String("2023-01-01T00:00:00Z")},
			},
			{
				"paper_id": {S: aws.String("paper2")},
				"trace_id": {S: aws.String(traceID)},
				"title":    {S: aws.String("Test Title 2")},
				"abstract": {S: aws.String("Test Abstract 2")},
				"source":   {S: aws.String("arxiv")},
				"authors":  {SS: []*string{aws.String("Author 2")}},
				"published_date": {S: aws.String("2023-01-02")},
				"categories":     {SS: []*string{aws.String("cs.ML")}},
				"batch_timestamp": {S: aws.String("2023-01-01T00:00:00Z")},
			},
		},
		LastEvaluatedKey: nil, // No pagination
	}

	mockClient.On("QueryWithContext", ctx, mock.AnythingOfType("*dynamodb.QueryInput")).Return(mockOutput, nil)

	// Execute
	papers, err := retriever.GetPapersByTraceID(ctx, traceID)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, papers, 2)
	assert.Equal(t, "paper1", papers[0].PaperID)
	assert.Equal(t, "Test Title 1", papers[0].Title)
	assert.Equal(t, "Test Abstract 1", papers[0].Abstract)
	assert.Equal(t, traceID, papers[0].TraceID)
	assert.Equal(t, "paper2", papers[1].PaperID)

	mockClient.AssertExpectations(t)
}

func TestGetPapersByTraceID_EmptyTraceID(t *testing.T) {
	retriever, _ := createTestRetriever()
	ctx := context.Background()

	// Execute
	papers, err := retriever.GetPapersByTraceID(ctx, "")

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, papers)
	assert.Contains(t, err.Error(), "traceID cannot be empty")
}

func TestGetPapersByTraceID_QueryError(t *testing.T) {
	retriever, mockClient := createTestRetriever()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock to return error
	queryError := errors.New("DynamoDB query failed")
	mockClient.On("QueryWithContext", ctx, mock.AnythingOfType("*dynamodb.QueryInput")).Return(nil, queryError)

	// Execute
	papers, err := retriever.GetPapersByTraceID(ctx, traceID)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, papers)
	assert.Contains(t, err.Error(), "failed to query papers by traceID")
	assert.Contains(t, err.Error(), "DynamoDB query failed")

	mockClient.AssertExpectations(t)
}

func TestGetPapersByTraceID_UnmarshalError(t *testing.T) {
	retriever, mockClient := createTestRetriever()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Create mock response with completely invalid structure that will cause unmarshal error
	mockOutput := &dynamodb.QueryOutput{
		Items: []map[string]*dynamodb.AttributeValue{
			{
				"invalid_structure": {M: map[string]*dynamodb.AttributeValue{
					"nested": {SS: []*string{aws.String("invalid")}},
				}},
			},
		},
		LastEvaluatedKey: nil,
	}

	mockClient.On("QueryWithContext", ctx, mock.AnythingOfType("*dynamodb.QueryInput")).Return(mockOutput, nil)

	// Execute
	papers, err := retriever.GetPapersByTraceID(ctx, traceID)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, papers)
	assert.Contains(t, err.Error(), "failed to unmarshal papers")

	mockClient.AssertExpectations(t)
}

func TestGetPapersByTraceID_WithPagination(t *testing.T) {
	retriever, mockClient := createTestRetriever()
	ctx := context.Background()
	traceID := "test-trace-123"

	// First page response
	firstPageOutput := &dynamodb.QueryOutput{
		Items: []map[string]*dynamodb.AttributeValue{
			{
				"paper_id": {S: aws.String("paper1")},
				"trace_id": {S: aws.String(traceID)},
				"title":    {S: aws.String("Test Title 1")},
				"abstract": {S: aws.String("Test Abstract 1")},
				"source":   {S: aws.String("arxiv")},
				"authors":  {SS: []*string{aws.String("Author 1")}},
				"published_date": {S: aws.String("2023-01-01")},
				"categories":     {SS: []*string{aws.String("cs.AI")}},
				"batch_timestamp": {S: aws.String("2023-01-01T00:00:00Z")},
			},
		},
		LastEvaluatedKey: map[string]*dynamodb.AttributeValue{
			"paper_id": {S: aws.String("paper1")},
		},
	}

	// Second page response
	secondPageOutput := &dynamodb.QueryOutput{
		Items: []map[string]*dynamodb.AttributeValue{
			{
				"paper_id": {S: aws.String("paper2")},
				"trace_id": {S: aws.String(traceID)},
				"title":    {S: aws.String("Test Title 2")},
				"abstract": {S: aws.String("Test Abstract 2")},
				"source":   {S: aws.String("arxiv")},
				"authors":  {SS: []*string{aws.String("Author 2")}},
				"published_date": {S: aws.String("2023-01-02")},
				"categories":     {SS: []*string{aws.String("cs.ML")}},
				"batch_timestamp": {S: aws.String("2023-01-01T00:00:00Z")},
			},
		},
		LastEvaluatedKey: nil, // End of pagination
	}

	// Setup mock expectations for pagination
	mockClient.On("QueryWithContext", ctx, mock.MatchedBy(func(input *dynamodb.QueryInput) bool {
		return input.ExclusiveStartKey == nil // First call
	})).Return(firstPageOutput, nil)

	mockClient.On("QueryWithContext", ctx, mock.MatchedBy(func(input *dynamodb.QueryInput) bool {
		return input.ExclusiveStartKey != nil // Second call with pagination token
	})).Return(secondPageOutput, nil)

	// Execute
	papers, err := retriever.GetPapersByTraceID(ctx, traceID)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, papers, 2)
	assert.Equal(t, "paper1", papers[0].PaperID)
	assert.Equal(t, "paper2", papers[1].PaperID)

	mockClient.AssertExpectations(t)
}

func TestGetPapersByTraceID_InvalidPaperData(t *testing.T) {
	retriever, mockClient := createTestRetriever()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Create mock response with valid and invalid papers
	mockOutput := &dynamodb.QueryOutput{
		Items: []map[string]*dynamodb.AttributeValue{
			{
				"paper_id": {S: aws.String("paper1")},
				"trace_id": {S: aws.String(traceID)},
				"title":    {S: aws.String("Test Title 1")},
				"abstract": {S: aws.String("Test Abstract 1")},
				"source":   {S: aws.String("arxiv")},
				"authors":  {SS: []*string{aws.String("Author 1")}},
				"published_date": {S: aws.String("2023-01-01")},
				"categories":     {SS: []*string{aws.String("cs.AI")}},
				"batch_timestamp": {S: aws.String("2023-01-01T00:00:00Z")},
			},
			{
				"paper_id": {S: aws.String("")}, // Invalid - empty paper_id
				"trace_id": {S: aws.String(traceID)},
				"title":    {S: aws.String("Test Title 2")},
				"abstract": {S: aws.String("Test Abstract 2")},
				"source":   {S: aws.String("arxiv")},
				"authors":  {SS: []*string{aws.String("Author 2")}},
				"published_date": {S: aws.String("2023-01-02")},
				"categories":     {SS: []*string{aws.String("cs.ML")}},
				"batch_timestamp": {S: aws.String("2023-01-01T00:00:00Z")},
			},
			{
				"paper_id": {S: aws.String("paper3")},
				"trace_id": {S: aws.String("")}, // Invalid - empty trace_id
				"title":    {S: aws.String("Test Title 3")},
				"abstract": {S: aws.String("Test Abstract 3")},
				"source":   {S: aws.String("arxiv")},
				"authors":  {SS: []*string{aws.String("Author 3")}},
				"published_date": {S: aws.String("2023-01-03")},
				"categories":     {SS: []*string{aws.String("cs.CV")}},
				"batch_timestamp": {S: aws.String("2023-01-01T00:00:00Z")},
			},
		},
		LastEvaluatedKey: nil,
	}

	mockClient.On("QueryWithContext", ctx, mock.AnythingOfType("*dynamodb.QueryInput")).Return(mockOutput, nil)

	// Execute
	papers, err := retriever.GetPapersByTraceID(ctx, traceID)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, papers, 1) // Only valid paper should be returned
	assert.Equal(t, "paper1", papers[0].PaperID)

	mockClient.AssertExpectations(t)
}

func TestValidatePaper(t *testing.T) {
	retriever, _ := createTestRetriever()

	// Test valid paper
	validPaper := &Paper{
		PaperID: "paper1",
		TraceID: "trace123",
		Title:   "Test Title",
		Abstract: "Test Abstract",
	}
	assert.NoError(t, retriever.validatePaper(validPaper))

	// Test empty paper_id
	invalidPaper1 := &Paper{
		PaperID: "",
		TraceID: "trace123",
		Title:   "Test Title",
		Abstract: "Test Abstract",
	}
	err := retriever.validatePaper(invalidPaper1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paper_id is empty")

	// Test empty trace_id
	invalidPaper2 := &Paper{
		PaperID: "paper1",
		TraceID: "",
		Title:   "Test Title",
		Abstract: "Test Abstract",
	}
	err = retriever.validatePaper(invalidPaper2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trace_id is empty")

	// Test empty title and abstract
	invalidPaper3 := &Paper{
		PaperID: "paper1",
		TraceID: "trace123",
		Title:   "",
		Abstract: "",
	}
	err = retriever.validatePaper(invalidPaper3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both title and abstract are empty")

	// Test paper with only title (should be valid)
	validPaper2 := &Paper{
		PaperID: "paper1",
		TraceID: "trace123",
		Title:   "Test Title",
		Abstract: "",
	}
	assert.NoError(t, retriever.validatePaper(validPaper2))

	// Test paper with only abstract (should be valid)
	validPaper3 := &Paper{
		PaperID: "paper1",
		TraceID: "trace123",
		Title:   "",
		Abstract: "Test Abstract",
	}
	assert.NoError(t, retriever.validatePaper(validPaper3))
}

func TestCombineTitleAndAbstract(t *testing.T) {
	retriever, _ := createTestRetriever()

	papers := []Paper{
		{
			PaperID:  "paper1",
			Title:    "Title 1",
			Abstract: "Abstract 1",
		},
		{
			PaperID:  "paper2",
			Title:    "Title 2",
			Abstract: "", // Empty abstract
		},
		{
			PaperID:  "paper3",
			Title:    "",
			Abstract: "Abstract 3", // Empty title
		},
		{
			PaperID:  "paper4",
			Title:    "",
			Abstract: "", // Both empty - should be skipped
		},
	}

	// Execute
	combinedTexts := retriever.CombineTitleAndAbstract(papers)

	// Assertions
	assert.Len(t, combinedTexts, 3) // paper4 should be skipped
	
	assert.Equal(t, "paper1", combinedTexts[0].PaperID)
	assert.Equal(t, "Title 1. Abstract 1", combinedTexts[0].Text)
	
	assert.Equal(t, "paper2", combinedTexts[1].PaperID)
	assert.Equal(t, "Title 2", combinedTexts[1].Text)
	
	assert.Equal(t, "paper3", combinedTexts[2].PaperID)
	assert.Equal(t, "Abstract 3", combinedTexts[2].Text)
}

func TestCombineTitleAndAbstract_EmptyInput(t *testing.T) {
	retriever, _ := createTestRetriever()

	// Execute with empty slice
	combinedTexts := retriever.CombineTitleAndAbstract([]Paper{})

	// Assertions
	assert.Nil(t, combinedTexts)
}

func TestGetCombinedTextsByTraceID_Success(t *testing.T) {
	retriever, mockClient := createTestRetriever()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Create mock response
	mockOutput := &dynamodb.QueryOutput{
		Items: []map[string]*dynamodb.AttributeValue{
			{
				"paper_id": {S: aws.String("paper1")},
				"trace_id": {S: aws.String(traceID)},
				"title":    {S: aws.String("Test Title 1")},
				"abstract": {S: aws.String("Test Abstract 1")},
				"source":   {S: aws.String("arxiv")},
				"authors":  {SS: []*string{aws.String("Author 1")}},
				"published_date": {S: aws.String("2023-01-01")},
				"categories":     {SS: []*string{aws.String("cs.AI")}},
				"batch_timestamp": {S: aws.String("2023-01-01T00:00:00Z")},
			},
		},
		LastEvaluatedKey: nil,
	}

	mockClient.On("QueryWithContext", ctx, mock.AnythingOfType("*dynamodb.QueryInput")).Return(mockOutput, nil)

	// Execute
	combinedTexts, err := retriever.GetCombinedTextsByTraceID(ctx, traceID)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, combinedTexts, 1)
	assert.Equal(t, "paper1", combinedTexts[0].PaperID)
	assert.Equal(t, "Test Title 1. Test Abstract 1", combinedTexts[0].Text)

	mockClient.AssertExpectations(t)
}

func TestGetCombinedTextsByTraceID_RetrievalError(t *testing.T) {
	retriever, mockClient := createTestRetriever()
	ctx := context.Background()
	traceID := "test-trace-123"

	// Setup mock to return error
	queryError := errors.New("DynamoDB query failed")
	mockClient.On("QueryWithContext", ctx, mock.AnythingOfType("*dynamodb.QueryInput")).Return(nil, queryError)

	// Execute
	combinedTexts, err := retriever.GetCombinedTextsByTraceID(ctx, traceID)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, combinedTexts)
	assert.Contains(t, err.Error(), "failed to query papers by traceID")

	mockClient.AssertExpectations(t)
}