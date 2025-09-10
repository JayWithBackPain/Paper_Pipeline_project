package s3

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockS3API is a mock implementation of S3 API interface
type MockS3API struct {
	s3iface.S3API
	mock.Mock
}

func (m *MockS3API) GetObjectWithContext(ctx context.Context, input *s3.GetObjectInput, opts ...request.Option) (*s3.GetObjectOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func TestDownloader_DownloadAndDecompress_PlainText(t *testing.T) {
	// Test data
	testData := "test content for plain text file"
	
	// Create a reader for test data
	reader := strings.NewReader(testData)
	
	// Mock S3 response
	mockOutput := &s3.GetObjectOutput{
		Body: io.NopCloser(reader),
	}
	
	// Create mock S3 API
	mockAPI := &MockS3API{}
	mockAPI.On("GetObjectWithContext", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
		return *input.Bucket == "test-bucket" && *input.Key == "test-file.txt"
	})).Return(mockOutput, nil)
	
	// Create downloader with mock
	downloader := &Downloader{s3Client: mockAPI}
	
	// Test download
	result, err := downloader.DownloadAndDecompress(context.Background(), "test-bucket", "test-file.txt")
	
	assert.NoError(t, err)
	assert.Equal(t, testData, string(result))
	mockAPI.AssertExpectations(t)
}

func TestDownloader_DownloadAndDecompress_GzipFile(t *testing.T) {
	// Test data
	testData := "test content for gzipped file"
	
	// Create gzipped data
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err := gzipWriter.Write([]byte(testData))
	assert.NoError(t, err)
	err = gzipWriter.Close()
	assert.NoError(t, err)
	
	// Create a reader for gzipped data
	reader := bytes.NewReader(buf.Bytes())
	
	// Mock S3 response
	mockOutput := &s3.GetObjectOutput{
		Body: io.NopCloser(reader),
	}
	
	// Create mock S3 API
	mockAPI := &MockS3API{}
	mockAPI.On("GetObjectWithContext", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
		return *input.Bucket == "test-bucket" && *input.Key == "test-file.gz"
	})).Return(mockOutput, nil)
	
	// Create downloader with mock
	downloader := &Downloader{s3Client: mockAPI}
	
	// Test download and decompress
	result, err := downloader.DownloadAndDecompress(context.Background(), "test-bucket", "test-file.gz")
	
	assert.NoError(t, err)
	assert.Equal(t, testData, string(result))
	mockAPI.AssertExpectations(t)
}

func TestNewDownloader(t *testing.T) {
	downloader := NewDownloader()
	assert.NotNil(t, downloader)
	assert.NotNil(t, downloader.s3Client)
}