package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"data-collector/arxiv"
	"data-collector/config"
	"data-collector/s3"
	"data-collector/types"
	"shared/logger"

	"github.com/aws/aws-lambda-go/lambda"
)

var (
	appLogger    *logger.Logger
	errorHandler *logger.ErrorHandler
)

func init() {
	appLogger = logger.New("data-collector")
	errorHandler = logger.NewErrorHandler(appLogger)
}

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handleLambda)
	} else {
		fmt.Println("Data Collector Service - Local Development Mode")
		if err := runLocalTest(); err != nil {
			appLogger.Error("Local test failed", err)
			os.Exit(1)
		}
	}
}

func handleLambda(ctx context.Context) error {
	defer func() {
		if err := errorHandler.HandleWithRecovery("lambda handler"); err != nil {
			appLogger.Error("Lambda handler panic recovered", err)
		}
	}()
	
	start := time.Now()
	contextLogger := appLogger.WithContext(ctx)
	
	contextLogger.Info("Data collector lambda handler started")
	
	// Execute the complete data collection pipeline
	result, err := executeDataCollection(ctx, contextLogger)
	if err != nil {
		return errorHandler.Handle(err, "data collection pipeline")
	}
	
	contextLogger.InfoWithDuration("Lambda handler completed successfully", time.Since(start))
	contextLogger.InfoWithCount("Papers collected and uploaded", result.Count)
	
	return nil
}

// executeDataCollection performs the complete data collection pipeline
func executeDataCollection(ctx context.Context, contextLogger *logger.Logger) (*types.CollectionResult, error) {
	start := time.Now()
	
	// 1. Load configuration
	contextLogger.Info("Loading configuration")
	cfg, err := loadConfiguration(ctx)
	if err != nil {
		return nil, logger.WrapError(err, logger.ErrorTypeConfig, "failed to load configuration")
	}
	
	// 2. Get arXiv data source configuration
	arxivConfig, err := cfg.GetDataSourceConfig("arxiv")
	if err != nil {
		return nil, logger.WrapError(err, logger.ErrorTypeConfig, "failed to get arXiv configuration")
	}
	
	contextLogger.Info("Configuration loaded successfully", map[string]interface{}{
		"api_endpoint": arxivConfig.APIEndpoint,
		"max_results":  arxivConfig.MaxResults,
		"rate_limit":   arxivConfig.RateLimit,
	})
	
	// 3. Initialize arXiv client
	arxivClient := arxiv.NewClient(arxivConfig.APIEndpoint, arxivConfig.RateLimit)
	
	// 4. Perform arXiv search
	contextLogger.Info("Starting arXiv API search")
	searchParams := arxiv.SearchParams{
		Query:      arxivConfig.SearchQuery,
		MaxResults: arxivConfig.MaxResults,
		StartIndex: 0,
	}
	
	result, err := arxivClient.Search(ctx, searchParams)
	if err != nil {
		return nil, logger.WrapError(err, logger.ErrorTypeAPI, "arXiv API search failed")
	}
	
	contextLogger.InfoWithCount("Papers retrieved from arXiv", result.Count)
	contextLogger.InfoWithDuration("arXiv API search completed", time.Since(start))
	
	// 5. Initialize S3 uploader
	uploader, err := s3.NewUploader(cfg.AWS.S3.RawDataBucket, cfg.AWS.S3.RawDataPrefix)
	if err != nil {
		return nil, logger.WrapError(err, logger.ErrorTypeS3, "failed to initialize S3 uploader")
	}
	
	// 6. Upload to S3
	contextLogger.Info("Uploading data to S3")
	uploadStart := time.Now()
	
	uploadResult, err := uploader.UploadCompressedData(ctx, result)
	if err != nil {
		return nil, logger.WrapError(err, logger.ErrorTypeS3, "S3 upload failed")
	}
	
	contextLogger.InfoWithDuration("S3 upload completed", time.Since(uploadStart))
	contextLogger.Info("Data uploaded successfully", map[string]interface{}{
		"s3_key":          uploadResult.S3Key,
		"compressed_size": uploadResult.CompressedSize,
		"original_size":   uploadResult.OriginalSize,
		"compression_ratio": float64(uploadResult.CompressedSize) / float64(uploadResult.OriginalSize),
	})
	
	contextLogger.InfoWithDuration("Complete data collection pipeline finished", time.Since(start))
	
	return result, nil
}

// loadConfiguration loads the pipeline configuration
func loadConfiguration(ctx context.Context) (*config.Config, error) {
	configManager, err := config.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}
	
	// Try to load from S3 first, fallback to default config
	configBucket := os.Getenv("CONFIG_BUCKET")
	configKey := os.Getenv("CONFIG_KEY")
	
	if configBucket != "" && configKey != "" {
		cfg, err := configManager.LoadFromS3(ctx, configBucket, configKey)
		if err != nil {
			appLogger.Warn("Failed to load config from S3, using default config", map[string]interface{}{
				"bucket": configBucket,
				"key":    configKey,
				"error":  err.Error(),
			})
			return config.GetDefaultConfig(), nil
		}
		return cfg, nil
	}
	
	// Use default configuration
	appLogger.Info("Using default configuration")
	return config.GetDefaultConfig(), nil
}

func runLocalTest() error {
	appLogger.Info("Starting local development test")
	
	// Execute the complete data collection pipeline in local mode
	ctx := context.Background()
	contextLogger := appLogger.WithContext(ctx)
	
	result, err := executeDataCollection(ctx, contextLogger)
	if err != nil {
		return fmt.Errorf("local test failed: %w", err)
	}
	
	appLogger.Info("Local development test completed successfully", map[string]interface{}{
		"papers_collected": result.Count,
		"source":          result.Source,
		"timestamp":       result.Timestamp.Format(time.RFC3339),
	})
	
	return nil
}