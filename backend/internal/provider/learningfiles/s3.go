// Package learningfiles supplies private object storage and malware scanning.
package learningfiles

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/config"
)

type Store struct {
	client            *s3.Client
	quarantine, clean string
}

func NewStore(ctx context.Context, cfg config.Config) (*Store, error) {
	if cfg.S3BucketClean == "" || cfg.S3BucketQuarantine == "" {
		return nil, fmt.Errorf("file storage: private buckets are required")
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.S3Region)}
	if cfg.S3AccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("file storage: loading credentials: %w", err)
	}
	awsCfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.S3ForcePathStyle
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &Store{client: client, quarantine: cfg.S3BucketQuarantine, clean: cfg.S3BucketClean}, nil
}
func (s *Store) bucket(clean bool) string {
	if clean {
		return s.clean
	}
	return s.quarantine
}
func (s *Store) Put(ctx context.Context, key, media string, data []byte, clean bool) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket(clean)), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String(media), ContentLength: aws.Int64(int64(len(data)))})
	return err
}
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.clean), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}
func (s *Store) Delete(ctx context.Context, key string, clean bool) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket(clean)), Key: aws.String(key)})
	return err
}
