package types

import (
	"encoding/xml"
	"time"
)

// Paper represents a research paper from any data source
type Paper struct {
	ID           string    `json:"id" xml:"id"`
	Source       string    `json:"source"`
	Title        string    `json:"title"`
	Abstract     string    `json:"abstract"`
	Authors      []string  `json:"authors"`
	PublishedDate time.Time `json:"published_date"`
	Categories   []string  `json:"categories"`
	RawXML       string    `json:"raw_xml,omitempty"`
	URL          string    `json:"url,omitempty"`
}

// ArxivFeed represents the root element of arXiv API response
type ArxivFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entries []ArxivEntry `xml:"entry"`
}

// ArxivEntry represents a single paper entry from arXiv API
type ArxivEntry struct {
	ID        string        `xml:"id"`
	Title     string        `xml:"title"`
	Summary   string        `xml:"summary"`
	Published string        `xml:"published"`
	Authors   []ArxivAuthor `xml:"author"`
	Categories []ArxivCategory `xml:"category"`
	Links     []ArxivLink   `xml:"link"`
}

// ArxivAuthor represents an author in arXiv response
type ArxivAuthor struct {
	Name string `xml:"name"`
}

// ArxivCategory represents a category in arXiv response
type ArxivCategory struct {
	Term string `xml:"term,attr"`
}

// ArxivLink represents a link in arXiv response
type ArxivLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

// CollectionResult represents the result of a data collection operation
type CollectionResult struct {
	Papers      []Paper   `json:"papers"`
	Source      string    `json:"source"`
	Count       int       `json:"count"`
	Timestamp   time.Time `json:"timestamp"`
	S3Key       string    `json:"s3_key,omitempty"`
	CompressedSize int64  `json:"compressed_size,omitempty"`
}