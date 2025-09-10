package deduplicator

import (
	"batch-processor/processor"
	"shared/logger"
)

// Deduplicator handles data deduplication logic
type Deduplicator struct{
	logger *logger.Logger
}

// NewDeduplicator creates a new deduplicator instance
func NewDeduplicator() *Deduplicator {
	return &Deduplicator{
		logger: logger.New("deduplicator"),
	}
}

// Deduplicate removes duplicate papers based on paper_id
func (d *Deduplicator) Deduplicate(papers []processor.Paper) []processor.Paper {
	if len(papers) == 0 {
		return papers
	}

	seen := make(map[string]bool)
	var deduplicated []processor.Paper
	duplicateCount := 0

	for _, paper := range papers {
		if paper.PaperID == "" {
			d.logger.Warn("Skipping paper with empty paper_id")
			continue
		}

		if !seen[paper.PaperID] {
			seen[paper.PaperID] = true
			deduplicated = append(deduplicated, paper)
		} else {
			duplicateCount++
			d.logger.Debug("Duplicate paper found and removed", map[string]interface{}{
				"paper_id": paper.PaperID,
			})
		}
	}

	d.logger.Info("Deduplication completed", map[string]interface{}{
		"original_count":   len(papers),
		"unique_count":     len(deduplicated),
		"duplicates_removed": duplicateCount,
	})

	return deduplicated
}

// DeduplicateWithStats returns deduplicated papers along with statistics
func (d *Deduplicator) DeduplicateWithStats(papers []processor.Paper) ([]processor.Paper, processor.DeduplicationStats) {
	stats := processor.DeduplicationStats{
		OriginalCount: len(papers),
	}

	if len(papers) == 0 {
		return papers, stats
	}

	seen := make(map[string]bool)
	var deduplicated []processor.Paper

	for _, paper := range papers {
		if paper.PaperID == "" {
			stats.InvalidCount++
			d.logger.Warn("Skipping paper with empty paper_id")
			continue
		}

		if !seen[paper.PaperID] {
			seen[paper.PaperID] = true
			deduplicated = append(deduplicated, paper)
		} else {
			stats.DuplicateCount++
			d.logger.Debug("Duplicate paper found and removed", map[string]interface{}{
				"paper_id": paper.PaperID,
			})
		}
	}

	stats.UniqueCount = len(deduplicated)

	d.logger.Info("Deduplication completed with stats", map[string]interface{}{
		"original_count":   stats.OriginalCount,
		"unique_count":     stats.UniqueCount,
		"duplicate_count":  stats.DuplicateCount,
		"invalid_count":    stats.InvalidCount,
	})

	return deduplicated, stats
}

