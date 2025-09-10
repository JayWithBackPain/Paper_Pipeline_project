package s3

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"data-collector/types"
)

func TestGenerateS3Key(t *testing.T) {
	uploader := &Uploader{
		prefix: "raw-data",
	}

	timestamp := time.Date(2023, 12, 25, 14, 30, 45, 0, time.UTC)
	key := uploader.generateS3Key("arxiv", timestamp)

	expected := "raw-data/2023-12-25/arxiv-papers-20231225-143045.gz"
	if key != expected {
		t.Errorf("Expected S3 key '%s', got '%s'", expected, key)
	}
}

func TestCompressDecompressData(t *testing.T) {
	originalData := []byte("This is test data for compression and decompression testing.")

	uploader := &Uploader{}
	
	// Test compression
	compressedData, err := uploader.compressData(originalData)
	if err != nil {
		t.Fatalf("Failed to compress data: %v", err)
	}

	if len(compressedData) == 0 {
		t.Error("Compressed data is empty")
	}

	// Compressed data should be different from original
	if string(compressedData) == string(originalData) {
		t.Error("Compressed data is identical to original data")
	}

	// Test decompression
	decompressedData, err := DecompressData(compressedData)
	if err != nil {
		t.Fatalf("Failed to decompress data: %v", err)
	}

	if string(decompressedData) != string(originalData) {
		t.Errorf("Decompressed data doesn't match original. Expected '%s', got '%s'", 
			string(originalData), string(decompressedData))
	}
}

func TestCompressCollectionResult(t *testing.T) {
	// Create test collection result
	papers := []types.Paper{
		{
			ID:       "2301.00001v1",
			Source:   "arxiv",
			Title:    "Test Paper 1",
			Abstract: "This is the abstract for test paper 1.",
			Authors:  []string{"John Doe", "Jane Smith"},
			PublishedDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Categories: []string{"cs.AI", "cs.LG"},
		},
		{
			ID:       "2301.00002v1",
			Source:   "arxiv",
			Title:    "Test Paper 2",
			Abstract: "This is the abstract for test paper 2.",
			Authors:  []string{"Alice Johnson"},
			PublishedDate: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			Categories: []string{"cs.CL"},
		},
	}

	collectionResult := &types.CollectionResult{
		Papers:    papers,
		Source:    "arxiv",
		Count:     len(papers),
		Timestamp: time.Now(),
	}

	// Convert to JSON
	jsonData, err := json.Marshal(collectionResult)
	if err != nil {
		t.Fatalf("Failed to marshal collection result: %v", err)
	}

	uploader := &Uploader{}
	
	// Compress the JSON data
	compressedData, err := uploader.compressData(jsonData)
	if err != nil {
		t.Fatalf("Failed to compress collection result: %v", err)
	}

	// Verify compression worked (compressed should be smaller for this amount of data)
	if len(compressedData) >= len(jsonData) {
		t.Logf("Warning: Compressed data (%d bytes) is not smaller than original (%d bytes). This is normal for small datasets.", 
			len(compressedData), len(jsonData))
	}

	// Decompress and verify
	decompressedData, err := DecompressData(compressedData)
	if err != nil {
		t.Fatalf("Failed to decompress data: %v", err)
	}

	// Parse back to collection result
	var decompressedResult types.CollectionResult
	err = json.Unmarshal(decompressedData, &decompressedResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal decompressed data: %v", err)
	}

	// Verify the data integrity
	if decompressedResult.Source != collectionResult.Source {
		t.Errorf("Source mismatch. Expected '%s', got '%s'", 
			collectionResult.Source, decompressedResult.Source)
	}

	if decompressedResult.Count != collectionResult.Count {
		t.Errorf("Count mismatch. Expected %d, got %d", 
			collectionResult.Count, decompressedResult.Count)
	}

	if len(decompressedResult.Papers) != len(collectionResult.Papers) {
		t.Errorf("Papers count mismatch. Expected %d, got %d", 
			len(collectionResult.Papers), len(decompressedResult.Papers))
	}

	// Verify first paper
	if len(decompressedResult.Papers) > 0 {
		originalPaper := collectionResult.Papers[0]
		decompressedPaper := decompressedResult.Papers[0]

		if decompressedPaper.ID != originalPaper.ID {
			t.Errorf("Paper ID mismatch. Expected '%s', got '%s'", 
				originalPaper.ID, decompressedPaper.ID)
		}

		if decompressedPaper.Title != originalPaper.Title {
			t.Errorf("Paper title mismatch. Expected '%s', got '%s'", 
				originalPaper.Title, decompressedPaper.Title)
		}
	}
}

func TestS3KeyTimestampFormat(t *testing.T) {
	uploader := &Uploader{
		prefix: "test-prefix",
	}

	testCases := []struct {
		timestamp time.Time
		source    string
		expected  string
	}{
		{
			timestamp: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			source:    "arxiv",
			expected:  "test-prefix/2023-01-01/arxiv-papers-20230101-000000.gz",
		},
		{
			timestamp: time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
			source:    "pubmed",
			expected:  "test-prefix/2023-12-31/pubmed-papers-20231231-235959.gz",
		},
	}

	for _, tc := range testCases {
		result := uploader.generateS3Key(tc.source, tc.timestamp)
		if result != tc.expected {
			t.Errorf("For timestamp %v and source %s, expected '%s', got '%s'", 
				tc.timestamp, tc.source, tc.expected, result)
		}
	}
}

func TestS3KeyUniqueness(t *testing.T) {
	uploader := &Uploader{
		prefix: "raw-data",
	}

	baseTime := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	
	// Generate keys with different timestamps (1 second apart)
	key1 := uploader.generateS3Key("arxiv", baseTime)
	key2 := uploader.generateS3Key("arxiv", baseTime.Add(1*time.Second))
	
	if key1 == key2 {
		t.Error("S3 keys should be unique for different timestamps")
	}

	// Generate keys with different sources (same timestamp)
	key3 := uploader.generateS3Key("arxiv", baseTime)
	key4 := uploader.generateS3Key("pubmed", baseTime)
	
	if key3 == key4 {
		t.Error("S3 keys should be unique for different sources")
	}
}

func TestS3KeyFormat(t *testing.T) {
	uploader := &Uploader{
		prefix: "raw-data",
	}

	timestamp := time.Date(2023, 6, 15, 14, 30, 45, 0, time.UTC)
	key := uploader.generateS3Key("arxiv", timestamp)

	// Verify key format components
	if !strings.HasPrefix(key, "raw-data/") {
		t.Errorf("S3 key should start with prefix 'raw-data/', got: %s", key)
	}

	if !strings.Contains(key, "2023-06-15") {
		t.Errorf("S3 key should contain date '2023-06-15', got: %s", key)
	}

	if !strings.Contains(key, "arxiv-papers-") {
		t.Errorf("S3 key should contain 'arxiv-papers-', got: %s", key)
	}

	if !strings.HasSuffix(key, ".gz") {
		t.Errorf("S3 key should end with '.gz', got: %s", key)
	}

	if !strings.Contains(key, "20230615-143045") {
		t.Errorf("S3 key should contain timestamp '20230615-143045', got: %s", key)
	}
}

func TestEmptyDataCompression(t *testing.T) {
	uploader := &Uploader{}
	
	// Test with empty data
	emptyData := []byte("")
	compressedData, err := uploader.compressData(emptyData)
	if err != nil {
		t.Fatalf("Failed to compress empty data: %v", err)
	}

	if len(compressedData) == 0 {
		t.Error("Compressed empty data should not be empty (gzip header)")
	}

	// Decompress and verify
	decompressedData, err := DecompressData(compressedData)
	if err != nil {
		t.Fatalf("Failed to decompress empty data: %v", err)
	}

	if len(decompressedData) != 0 {
		t.Errorf("Decompressed empty data should be empty, got %d bytes", len(decompressedData))
	}
}