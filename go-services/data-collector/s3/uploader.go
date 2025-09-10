package s3

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"data-collector/types"
)

// Uploader handles S3 upload operations
type Uploader struct {
	s3Client *s3.S3
	bucket   string
	prefix   string
}

// NewUploader creates a new S3 uploader
func NewUploader(bucket, prefix string) (*Uploader, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"), // Default region
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &Uploader{
		s3Client: s3.New(sess),
		bucket:   bucket,
		prefix:   prefix,
	}, nil
}

// UploadResult represents the result of an S3 upload operation
type UploadResult struct {
	S3Key          string    `json:"s3_key"`
	CompressedSize int64     `json:"compressed_size"`
	OriginalSize   int64     `json:"original_size"`
	Timestamp      time.Time `json:"timestamp"`
}

// UploadCompressedData uploads compressed data to S3 with timestamp-based naming
func (u *Uploader) UploadCompressedData(ctx context.Context, result *types.CollectionResult) (*UploadResult, error) {
	// Generate S3 key with timestamp
	s3Key := u.generateS3Key(result.Source, result.Timestamp)

	// Convert collection result to JSON
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal collection result: %w", err)
	}

	// Compress the data
	compressedData, err := u.compressData(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to compress data: %w", err)
	}

	// Upload to S3
	input := &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(compressedData),
		ContentType: aws.String("application/gzip"),
		Metadata: map[string]*string{
			"source":         aws.String(result.Source),
			"paper-count":    aws.String(fmt.Sprintf("%d", result.Count)),
			"collection-time": aws.String(result.Timestamp.Format(time.RFC3339)),
		},
	}

	_, err = u.s3Client.PutObjectWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	return &UploadResult{
		S3Key:          s3Key,
		CompressedSize: int64(len(compressedData)),
		OriginalSize:   int64(len(jsonData)),
		Timestamp:      time.Now(),
	}, nil
}

// generateS3Key generates a timestamp-based S3 key
func (u *Uploader) generateS3Key(source string, timestamp time.Time) string {
	// Format: raw-data/YYYY-MM-DD/source-papers-YYYYMMDD-HHMMSS.gz
	dateStr := timestamp.Format("2006-01-02")
	timestampStr := timestamp.Format("20060102-150405")
	
	return fmt.Sprintf("%s/%s/%s-papers-%s.gz", u.prefix, dateStr, source, timestampStr)
}

// compressData compresses data using gzip
func (u *Uploader) compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	
	_, err := gzipWriter.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to write to gzip writer: %w", err)
	}
	
	err = gzipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}
	
	return buf.Bytes(), nil
}

// DecompressData decompresses gzip data (utility function for testing)
func DecompressData(compressedData []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, err = buf.ReadFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read from gzip reader: %w", err)
	}

	return buf.Bytes(), nil
}

// CheckS3KeyExists checks if an S3 key already exists (to avoid duplicate uploads)
func (u *Uploader) CheckS3KeyExists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	}

	_, err := u.s3Client.HeadObjectWithContext(ctx, input)
	if err != nil {
		// If the error is NoSuchKey, the object doesn't exist
		if isNoSuchKeyError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check S3 key existence: %w", err)
	}

	return true, nil
}

// isNoSuchKeyError checks if the error is a NoSuchKey error
func isNoSuchKeyError(err error) bool {
	return err != nil && (err.Error() == "NoSuchKey" || err.Error() == "NotFound")
}