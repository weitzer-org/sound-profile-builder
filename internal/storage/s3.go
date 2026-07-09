package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client implements the storage.Client interface against any S3-compatible
// object store: Cloudflare R2 (prod), MinIO (local Docker dev), AWS S3, or
// Google Cloud Storage via its S3 interoperability mode. This keeps the app
// portable across runtimes without touching the GCS backend in gcs.go.
type S3Client struct {
	client *s3.Client
}

// NewS3Client builds an S3-compatible client from environment configuration:
//
//	S3_ENDPOINT           e.g. http://minio:9000 or https://<acct>.r2.cloudflarestorage.com
//	S3_REGION             defaults to "auto" (correct for Cloudflare R2)
//	S3_ACCESS_KEY_ID      access key
//	S3_SECRET_ACCESS_KEY  secret key
func NewS3Client(ctx context.Context) (*S3Client, error) {
	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "auto" // Cloudflare R2 expects "auto"
	}

	accessKey := os.Getenv("S3_ACCESS_KEY_ID")
	secretKey := os.Getenv("S3_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("s3 backend requires S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY")
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load S3 config: %w", err)
	}

	endpoint := os.Getenv("S3_ENDPOINT")
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		// Path-style addressing keeps MinIO and custom endpoints working
		// without requiring per-bucket virtual-hosted DNS.
		o.UsePathStyle = true
		// Only add checksums when the operation requires them — the default
		// (when_supported) sends CRC32 trailers that some S3-compatible
		// stores (older MinIO, strict R2 configs) reject.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})

	return &S3Client{client: client}, nil
}

// ReadFile reads the contents of an object.
func (s *S3Client) ReadFile(ctx context.Context, bucket, object string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

// WriteFile writes data to an object, creating or overwriting it.
func (s *S3Client) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   bytes.NewReader(data),
	})
	return err
}

// ListFiles lists object keys under a prefix, paginating through all results.
func (s *S3Client) ListFiles(ctx context.Context, bucket, prefix string) ([]string, error) {
	var files []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			name := aws.ToString(obj.Key)
			// Mirror the GCS client: ignore the synthetic prefix "directory".
			if name != prefix {
				files = append(files, name)
			}
		}
	}
	return files, nil
}

// DeleteFile removes an object.
func (s *S3Client) DeleteFile(ctx context.Context, bucket, object string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	return err
}

// Close satisfies the storage.Client interface; the S3 client needs no cleanup.
func (s *S3Client) Close() {}
