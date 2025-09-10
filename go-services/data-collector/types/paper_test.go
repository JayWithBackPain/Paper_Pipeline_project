package types

import (
	"encoding/xml"
	"testing"
	"time"
)

func TestArxivFeedUnmarshal(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2301.00001v1</id>
    <title>Test Paper Title</title>
    <summary>This is a test abstract for the paper.</summary>
    <published>2023-01-01T00:00:00Z</published>
    <author>
      <name>John Doe</name>
    </author>
    <author>
      <name>Jane Smith</name>
    </author>
    <category term="cs.AI" />
    <category term="cs.LG" />
    <link href="http://arxiv.org/abs/2301.00001v1" rel="alternate" type="text/html" />
  </entry>
</feed>`

	var feed ArxivFeed
	err := xml.Unmarshal([]byte(xmlData), &feed)
	if err != nil {
		t.Fatalf("Failed to unmarshal XML: %v", err)
	}

	if len(feed.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(feed.Entries))
	}

	entry := feed.Entries[0]
	if entry.ID != "http://arxiv.org/abs/2301.00001v1" {
		t.Errorf("Expected ID 'http://arxiv.org/abs/2301.00001v1', got '%s'", entry.ID)
	}

	if entry.Title != "Test Paper Title" {
		t.Errorf("Expected title 'Test Paper Title', got '%s'", entry.Title)
	}

	if entry.Summary != "This is a test abstract for the paper." {
		t.Errorf("Expected summary 'This is a test abstract for the paper.', got '%s'", entry.Summary)
	}

	if len(entry.Authors) != 2 {
		t.Errorf("Expected 2 authors, got %d", len(entry.Authors))
	}

	if entry.Authors[0].Name != "John Doe" {
		t.Errorf("Expected first author 'John Doe', got '%s'", entry.Authors[0].Name)
	}

	if len(entry.Categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(entry.Categories))
	}

	if entry.Categories[0].Term != "cs.AI" {
		t.Errorf("Expected first category 'cs.AI', got '%s'", entry.Categories[0].Term)
	}
}

func TestPaperStruct(t *testing.T) {
	now := time.Now()
	paper := Paper{
		ID:            "2301.00001v1",
		Source:        "arxiv",
		Title:         "Test Paper",
		Abstract:      "Test abstract",
		Authors:       []string{"John Doe", "Jane Smith"},
		PublishedDate: now,
		Categories:    []string{"cs.AI", "cs.LG"},
		RawXML:        "<xml>test</xml>",
		URL:           "http://arxiv.org/abs/2301.00001v1",
	}

	if paper.ID != "2301.00001v1" {
		t.Errorf("Expected ID '2301.00001v1', got '%s'", paper.ID)
	}

	if paper.Source != "arxiv" {
		t.Errorf("Expected source 'arxiv', got '%s'", paper.Source)
	}

	if len(paper.Authors) != 2 {
		t.Errorf("Expected 2 authors, got %d", len(paper.Authors))
	}

	if len(paper.Categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(paper.Categories))
	}
}

func TestCollectionResult(t *testing.T) {
	now := time.Now()
	papers := []Paper{
		{
			ID:     "1",
			Source: "arxiv",
			Title:  "Paper 1",
		},
		{
			ID:     "2",
			Source: "arxiv",
			Title:  "Paper 2",
		},
	}

	result := CollectionResult{
		Papers:         papers,
		Source:         "arxiv",
		Count:          len(papers),
		Timestamp:      now,
		S3Key:          "test-key",
		CompressedSize: 1024,
	}

	if result.Count != 2 {
		t.Errorf("Expected count 2, got %d", result.Count)
	}

	if result.Source != "arxiv" {
		t.Errorf("Expected source 'arxiv', got '%s'", result.Source)
	}

	if len(result.Papers) != 2 {
		t.Errorf("Expected 2 papers, got %d", len(result.Papers))
	}
}