package arxiv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"data-collector/types"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://test.com", 3)
	
	if client.baseURL != "http://test.com" {
		t.Errorf("Expected baseURL 'http://test.com', got '%s'", client.baseURL)
	}
	
	expectedRateLimit := time.Second / 3
	if client.rateLimit != expectedRateLimit {
		t.Errorf("Expected rateLimit %v, got %v", expectedRateLimit, client.rateLimit)
	}
	
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestBuildQueryURL(t *testing.T) {
	client := NewClient("http://export.arxiv.org/api/query", 3)
	
	params := SearchParams{
		Query:      "cat:cs.AI",
		MaxResults: 100,
		StartIndex: 0,
	}
	
	url, err := client.buildQueryURL(params)
	if err != nil {
		t.Fatalf("Failed to build query URL: %v", err)
	}
	
	if !strings.Contains(url, "search_query=cat%3Acs.AI") {
		t.Errorf("URL should contain encoded search query, got: %s", url)
	}
	
	if !strings.Contains(url, "max_results=100") {
		t.Errorf("URL should contain max_results=100, got: %s", url)
	}
	
	if !strings.Contains(url, "start=0") {
		t.Errorf("URL should contain start=0, got: %s", url)
	}
}

func TestExtractArxivID(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"http://arxiv.org/abs/2301.00001v1", "2301.00001v1"},
		{"https://arxiv.org/abs/1234.5678v2", "1234.5678v2"},
		{"2301.00001v1", "2301.00001v1"},
		{"", ""},
	}
	
	for _, tc := range testCases {
		result := extractArxivID(tc.input)
		if result != tc.expected {
			t.Errorf("extractArxivID(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestConvertEntryToPaper(t *testing.T) {
	client := NewClient("http://test.com", 3)
	
	entry := types.ArxivEntry{
		ID:        "http://arxiv.org/abs/2301.00001v1",
		Title:     "  Test Paper Title  ",
		Summary:   "  This is a test abstract.  ",
		Published: "2023-01-01T00:00:00Z",
		Authors: []types.ArxivAuthor{
			{Name: "John Doe"},
			{Name: "Jane Smith"},
		},
		Categories: []types.ArxivCategory{
			{Term: "cs.AI"},
			{Term: "cs.LG"},
		},
		Links: []types.ArxivLink{
			{Href: "http://arxiv.org/abs/2301.00001v1", Rel: "alternate", Type: "text/html"},
		},
	}
	
	paper, err := client.convertEntryToPaper(entry, "<xml>test</xml>")
	if err != nil {
		t.Fatalf("Failed to convert entry to paper: %v", err)
	}
	
	if paper.ID != "2301.00001v1" {
		t.Errorf("Expected ID '2301.00001v1', got '%s'", paper.ID)
	}
	
	if paper.Source != "arxiv" {
		t.Errorf("Expected source 'arxiv', got '%s'", paper.Source)
	}
	
	if paper.Title != "Test Paper Title" {
		t.Errorf("Expected title 'Test Paper Title', got '%s'", paper.Title)
	}
	
	if paper.Abstract != "This is a test abstract." {
		t.Errorf("Expected abstract 'This is a test abstract.', got '%s'", paper.Abstract)
	}
	
	if len(paper.Authors) != 2 {
		t.Errorf("Expected 2 authors, got %d", len(paper.Authors))
	}
	
	if paper.Authors[0] != "John Doe" {
		t.Errorf("Expected first author 'John Doe', got '%s'", paper.Authors[0])
	}
	
	if len(paper.Categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(paper.Categories))
	}
	
	if paper.Categories[0] != "cs.AI" {
		t.Errorf("Expected first category 'cs.AI', got '%s'", paper.Categories[0])
	}
	
	if paper.URL != "http://arxiv.org/abs/2301.00001v1" {
		t.Errorf("Expected URL 'http://arxiv.org/abs/2301.00001v1', got '%s'", paper.URL)
	}
	
	expectedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	if !paper.PublishedDate.Equal(expectedTime) {
		t.Errorf("Expected published date %v, got %v", expectedTime, paper.PublishedDate)
	}
}

func TestSearchWithMockServer(t *testing.T) {
	// Create mock server
	mockResponse := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2301.00001v1</id>
    <title>Test Paper Title</title>
    <summary>This is a test abstract for the paper.</summary>
    <published>2023-01-01T00:00:00Z</published>
    <author>
      <name>John Doe</name>
    </author>
    <category term="cs.AI" />
    <link href="http://arxiv.org/abs/2301.00001v1" rel="alternate" type="text/html" />
  </entry>
</feed>`
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()
	
	client := NewClient(server.URL, 10) // High rate limit for testing
	
	params := SearchParams{
		Query:      "cat:cs.AI",
		MaxResults: 10,
		StartIndex: 0,
	}
	
	ctx := context.Background()
	result, err := client.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if result.Source != "arxiv" {
		t.Errorf("Expected source 'arxiv', got '%s'", result.Source)
	}
	
	if result.Count != 1 {
		t.Errorf("Expected count 1, got %d", result.Count)
	}
	
	if len(result.Papers) != 1 {
		t.Errorf("Expected 1 paper, got %d", len(result.Papers))
	}
	
	paper := result.Papers[0]
	if paper.ID != "2301.00001v1" {
		t.Errorf("Expected paper ID '2301.00001v1', got '%s'", paper.ID)
	}
	
	if paper.Title != "Test Paper Title" {
		t.Errorf("Expected paper title 'Test Paper Title', got '%s'", paper.Title)
	}
}

func TestSearchWithHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	
	client := NewClient(server.URL, 10)
	
	params := SearchParams{
		Query:      "cat:cs.AI",
		MaxResults: 10,
		StartIndex: 0,
	}
	
	ctx := context.Background()
	_, err := client.Search(ctx, params)
	if err == nil {
		t.Error("Expected error for HTTP 500, got nil")
	}
	
	if !strings.Contains(err.Error(), "API returned status 500") {
		t.Errorf("Expected error message about status 500, got: %v", err)
	}
}

func TestRateLimiting(t *testing.T) {
	client := NewClient("http://test.com", 2) // 2 requests per second
	
	start := time.Now()
	
	// First request should not wait
	err := client.waitForRateLimit()
	if err != nil {
		t.Fatalf("First rate limit wait failed: %v", err)
	}
	
	// Second request should wait
	err = client.waitForRateLimit()
	if err != nil {
		t.Fatalf("Second rate limit wait failed: %v", err)
	}
	
	elapsed := time.Since(start)
	expectedMinWait := time.Second / 2 // 500ms for 2 requests per second
	
	if elapsed < expectedMinWait {
		t.Errorf("Rate limiting not working properly. Expected at least %v, got %v", expectedMinWait, elapsed)
	}
}