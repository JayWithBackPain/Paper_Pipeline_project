package arxiv

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"data-collector/types"
)

// Client represents an arXiv API client
type Client struct {
	httpClient  *http.Client
	baseURL     string
	rateLimit   time.Duration
	lastRequest time.Time
}

// NewClient creates a new arXiv API client
func NewClient(baseURL string, rateLimitPerSecond int) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   baseURL,
		rateLimit: time.Second / time.Duration(rateLimitPerSecond),
	}
}

// SearchParams represents search parameters for arXiv API
type SearchParams struct {
	Query      string
	MaxResults int
	StartIndex int
	DateFrom   *time.Time // Optional: search from this date (inclusive)
	DateTo     *time.Time // Optional: search to this date (inclusive)
}

// Search performs a search query against arXiv API
func (c *Client) Search(ctx context.Context, params SearchParams) (*types.CollectionResult, error) {
	// Rate limiting
	if err := c.waitForRateLimit(); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	// Build query URL
	queryURL, err := c.buildQueryURL(params)
	if err != nil {
		return nil, fmt.Errorf("failed to build query URL: %w", err)
	}

	// Make HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse XML response
	var feed types.ArxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	// Convert to Paper structs
	papers, err := c.convertEntriesToPapers(feed.Entries, string(body))
	if err != nil {
		return nil, fmt.Errorf("failed to convert entries to papers: %w", err)
	}

	return &types.CollectionResult{
		Papers:    papers,
		Source:    "arxiv",
		Count:     len(papers),
		Timestamp: time.Now(),
	}, nil
}

// waitForRateLimit implements rate limiting
func (c *Client) waitForRateLimit() error {
	now := time.Now()
	if c.lastRequest.IsZero() {
		c.lastRequest = now
		return nil
	}

	elapsed := now.Sub(c.lastRequest)
	if elapsed < c.rateLimit {
		waitTime := c.rateLimit - elapsed
		time.Sleep(waitTime)
	}

	c.lastRequest = time.Now()
	return nil
}

// buildQueryURL constructs the query URL for arXiv API
func (c *Client) buildQueryURL(params SearchParams) (string, error) {
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Build the search query with optional date range
	searchQuery := params.Query
	if params.DateFrom != nil || params.DateTo != nil {
		dateQuery := c.buildDateQuery(params.DateFrom, params.DateTo)
		if dateQuery != "" {
			if searchQuery != "" {
				searchQuery = fmt.Sprintf("(%s) AND %s", searchQuery, dateQuery)
			} else {
				searchQuery = dateQuery
			}
		}
	}

	query := baseURL.Query()
	query.Set("search_query", searchQuery)
	query.Set("max_results", strconv.Itoa(params.MaxResults))
	query.Set("start", strconv.Itoa(params.StartIndex))
	query.Set("sortBy", "submittedDate")
	query.Set("sortOrder", "descending")

	baseURL.RawQuery = query.Encode()
	return baseURL.String(), nil
}

// convertEntriesToPapers converts arXiv entries to Paper structs
func (c *Client) convertEntriesToPapers(entries []types.ArxivEntry, rawXML string) ([]types.Paper, error) {
	papers := make([]types.Paper, 0, len(entries))

	for _, entry := range entries {
		paper, err := c.convertEntryToPaper(entry, rawXML)
		if err != nil {
			// Log error but continue processing other entries
			continue
		}
		papers = append(papers, paper)
	}

	return papers, nil
}

// convertEntryToPaper converts a single arXiv entry to Paper struct
func (c *Client) convertEntryToPaper(entry types.ArxivEntry, rawXML string) (types.Paper, error) {
	// Parse published date
	publishedDate, err := time.Parse("2006-01-02T15:04:05Z", entry.Published)
	if err != nil {
		return types.Paper{}, fmt.Errorf("failed to parse published date: %w", err)
	}

	// Extract authors
	authors := make([]string, len(entry.Authors))
	for i, author := range entry.Authors {
		authors[i] = strings.TrimSpace(author.Name)
	}

	// Extract categories
	categories := make([]string, len(entry.Categories))
	for i, category := range entry.Categories {
		categories[i] = category.Term
	}

	// Extract arXiv ID from the full URL
	arxivID := extractArxivID(entry.ID)

	// Find the paper URL
	paperURL := ""
	for _, link := range entry.Links {
		if link.Rel == "alternate" {
			paperURL = link.Href
			break
		}
	}

	return types.Paper{
		ID:            arxivID,
		Source:        "arxiv",
		Title:         strings.TrimSpace(entry.Title),
		Abstract:      strings.TrimSpace(entry.Summary),
		Authors:       authors,
		PublishedDate: publishedDate,
		Categories:    categories,
		RawXML:        rawXML,
		URL:           paperURL,
	}, nil
}

// buildDateQuery constructs date range query for arXiv API
func (c *Client) buildDateQuery(dateFrom, dateTo *time.Time) string {
	// arXiv uses YYYYMMDD format for date queries
	var dateQueries []string

	if dateFrom != nil {
		fromStr := dateFrom.Format("20060102")
		dateQueries = append(dateQueries, fmt.Sprintf("submittedDate:[%s0000 TO *]", fromStr))
	}

	if dateTo != nil {
		toStr := dateTo.Format("20060102")
		dateQueries = append(dateQueries, fmt.Sprintf("submittedDate:[* TO %s2359]", toStr))
	}

	if len(dateQueries) == 2 && dateFrom != nil && dateTo != nil {
		// If both dates are provided, create a single range query
		fromStr := dateFrom.Format("20060102")
		toStr := dateTo.Format("20060102")
		return fmt.Sprintf("submittedDate:[%s0000 TO %s2359]", fromStr, toStr)
	}

	return strings.Join(dateQueries, " AND ")
}

// extractArxivID extracts the arXiv ID from the full URL
func extractArxivID(fullURL string) string {
	// arXiv URLs are typically in format: http://arxiv.org/abs/1234.5678v1
	parts := strings.Split(fullURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullURL
}
