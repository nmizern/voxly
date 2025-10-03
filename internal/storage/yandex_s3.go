package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"
	"voxly/pkg/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

type S3Storage struct {
	client *s3.Client
	bucket string
}

// NewS3Storage creates a new S3 storage client
func NewS3Storage(endpoint, accessKey, secretKey, bucket string) (*S3Storage, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           endpoint,
				SigningRegion: "ru-central1",
			}, nil
		})

	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		config.WithRegion("ru-central1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load S3 config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	logger.Info("S3 storage initialized", zap.String("bucket", bucket))

	return &S3Storage{
		client: client,
		bucket: bucket,
	}, nil
}

// UploadFile uploads a file to S3
func (s *S3Storage) UploadFile(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Generate public URL (Yandex Object Storage format)
	url := fmt.Sprintf("https://storage.yandexcloud.net/%s/%s", s.bucket, key)

	logger.Info("File uploaded to S3",
		zap.String("key", key),
		zap.String("url", url))

	return url, nil
}

// GenerateKey generates a unique key for S3 object
func (s *S3Storage) GenerateKey(taskID, extension string) string {
	timestamp := time.Now().Format("2006/01/02")
	return filepath.Join("voice", timestamp, fmt.Sprintf("%s%s", taskID, extension))
}

// DownloadFile downloads a file from S3
func (s *S3Storage) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	logger.Debug("File downloaded from S3",
		zap.String("key", key),
		zap.Int("size", len(data)))

	return data, nil
}

// DeleteFile deletes a file from S3
func (s *S3Storage) DeleteFile(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	logger.Debug("File deleted from S3", zap.String("key", key))

	return nil
}
