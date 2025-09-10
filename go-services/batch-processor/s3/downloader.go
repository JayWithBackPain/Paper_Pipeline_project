package s3

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// Downloader handles S3 file downloads and decompression
type Downloader struct {
	s3Client s3iface.S3API
}

// NewDownloader creates a new S3 downloader instance
func NewDownloader() *Downloader {
	sess := session.Must(session.NewSession())
	return &Downloader{
		s3Client: s3.New(sess),
	}
}

// DownloadAndDecompress downloads a file from S3 and decompresses it if it's gzipped
func (d *Downloader) DownloadAndDecompress(ctx context.Context, bucket, key string) ([]byte, error) {
	// Download file from S3
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := d.s3Client.GetObjectWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to download S3 object %s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	// Read the content
	var reader io.Reader = result.Body

	// Check if file is gzipped based on extension or content type
	if strings.HasSuffix(key, ".gz") || strings.HasSuffix(key, ".gzip") {
		gzipReader, err := gzip.NewReader(result.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader for %s/%s: %w", bucket, key, err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read all content
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read content from %s/%s: %w", bucket, key, err)
	}

	return data, nil
}