package deduplicator

import (
	"batch-processor/processor"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicator_Deduplicate_NoDuplicates(t *testing.T) {
	dedup := NewDeduplicator()
	
	papers := []processor.Paper{
		{PaperID: "paper-1", Title: "Paper 1"},
		{PaperID: "paper-2", Title: "Paper 2"},
		{PaperID: "paper-3", Title: "Paper 3"},
	}

	result := dedup.Deduplicate(papers)

	assert.Equal(t, 3, len(result))
	assert.Equal(t, "paper-1", result[0].PaperID)
	assert.Equal(t, "paper-2", result[1].PaperID)
	assert.Equal(t, "paper-3", result[2].PaperID)
}

func TestDeduplicator_Deduplicate_WithDuplicates(t *testing.T) {
	dedup := NewDeduplicator()
	
	papers := []processor.Paper{
		{PaperID: "paper-1", Title: "Paper 1"},
		{PaperID: "paper-2", Title: "Paper 2"},
		{PaperID: "paper-1", Title: "Paper 1 Duplicate"}, // Duplicate
		{PaperID: "paper-3", Title: "Paper 3"},
		{PaperID: "paper-2", Title: "Paper 2 Duplicate"}, // Duplicate
	}

	result := dedup.Deduplicate(papers)

	assert.Equal(t, 3, len(result))
	
	// Check that we kept the first occurrence of each paper
	paperIDs := make([]string, len(result))
	for i, paper := range result {
		paperIDs[i] = paper.PaperID
	}
	
	assert.Contains(t, paperIDs, "paper-1")
	assert.Contains(t, paperIDs, "paper-2")
	assert.Contains(t, paperIDs, "paper-3")
	
	// Verify we kept the first occurrence (original title, not duplicate)
	for _, paper := range result {
		switch paper.PaperID {
		case "paper-1":
			assert.Equal(t, "Paper 1", paper.Title)
		case "paper-2":
			assert.Equal(t, "Paper 2", paper.Title)
		}
	}
}

func TestDeduplicator_Deduplicate_EmptyInput(t *testing.T) {
	dedup := NewDeduplicator()
	
	papers := []processor.Paper{}
	result := dedup.Deduplicate(papers)

	assert.Equal(t, 0, len(result))
}

func TestDeduplicator_Deduplicate_EmptyPaperID(t *testing.T) {
	dedup := NewDeduplicator()
	
	papers := []processor.Paper{
		{PaperID: "paper-1", Title: "Paper 1"},
		{PaperID: "", Title: "Paper with empty ID"}, // Should be skipped
		{PaperID: "paper-2", Title: "Paper 2"},
	}

	result := dedup.Deduplicate(papers)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, "paper-1", result[0].PaperID)
	assert.Equal(t, "paper-2", result[1].PaperID)
}

func TestDeduplicator_DeduplicateWithStats_Success(t *testing.T) {
	dedup := NewDeduplicator()
	
	papers := []processor.Paper{
		{PaperID: "paper-1", Title: "Paper 1"},
		{PaperID: "paper-2", Title: "Paper 2"},
		{PaperID: "paper-1", Title: "Paper 1 Duplicate"}, // Duplicate
		{PaperID: "", Title: "Invalid paper"},            // Invalid
		{PaperID: "paper-3", Title: "Paper 3"},
	}

	result, stats := dedup.DeduplicateWithStats(papers)

	// Check results
	assert.Equal(t, 3, len(result))
	
	// Check stats
	assert.Equal(t, 5, stats.OriginalCount)
	assert.Equal(t, 3, stats.UniqueCount)
	assert.Equal(t, 1, stats.DuplicateCount)
	assert.Equal(t, 1, stats.InvalidCount)
}

func TestDeduplicator_DeduplicateWithStats_EmptyInput(t *testing.T) {
	dedup := NewDeduplicator()
	
	papers := []processor.Paper{}
	result, stats := dedup.DeduplicateWithStats(papers)

	assert.Equal(t, 0, len(result))
	assert.Equal(t, 0, stats.OriginalCount)
	assert.Equal(t, 0, stats.UniqueCount)
	assert.Equal(t, 0, stats.DuplicateCount)
	assert.Equal(t, 0, stats.InvalidCount)
}

func TestNewDeduplicator(t *testing.T) {
	dedup := NewDeduplicator()
	assert.NotNil(t, dedup)
}

// Helper function to create test papers
func createTestPaper(id, title string) processor.Paper {
	now := time.Now().Format(time.RFC3339)
	return processor.Paper{
		PaperID:          id,
		Source:           "test",
		Title:            title,
		Abstract:         "Test abstract",
		Authors:          []string{"Test Author"},
		PublishedDate:    "2023-01-01",
		Categories:       []string{"test"},
		TraceID:          "test-trace",
		BatchTimestamp:   now,
		ProcessingStatus: "processed",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestDeduplicator_Deduplicate_ComplexScenario(t *testing.T) {
	dedup := NewDeduplicator()
	
	papers := []processor.Paper{
		createTestPaper("paper-1", "First Paper"),
		createTestPaper("paper-2", "Second Paper"),
		createTestPaper("paper-1", "First Paper Duplicate"),
		createTestPaper("paper-3", "Third Paper"),
		createTestPaper("paper-2", "Second Paper Duplicate"),
		createTestPaper("paper-1", "First Paper Another Duplicate"),
	}

	result, stats := dedup.DeduplicateWithStats(papers)

	// Should have 3 unique papers
	assert.Equal(t, 3, len(result))
	assert.Equal(t, 6, stats.OriginalCount)
	assert.Equal(t, 3, stats.UniqueCount)
	assert.Equal(t, 3, stats.DuplicateCount)
	assert.Equal(t, 0, stats.InvalidCount)
	
	// Verify the first occurrence is kept
	for _, paper := range result {
		switch paper.PaperID {
		case "paper-1":
			assert.Equal(t, "First Paper", paper.Title)
		case "paper-2":
			assert.Equal(t, "Second Paper", paper.Title)
		case "paper-3":
			assert.Equal(t, "Third Paper", paper.Title)
		}
	}
}