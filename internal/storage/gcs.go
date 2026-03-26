package storage

import (
	"context"
	"io/ioutil"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// GCSClient wraps Google Cloud Storage logic for Preset CRUD and Map Lookups
type GCSClient struct {
	client *storage.Client
}

// NewGCSClient initializes the Storage client connection
func NewGCSClient(ctx context.Context, opts ...option.ClientOption) (*GCSClient, error) {
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &GCSClient{client: client}, nil
}

// ReadFile reads the JSON content of an object in a GCS bucket
func (g *GCSClient) ReadFile(ctx context.Context, bucket, object string) ([]byte, error) {
	reader, err := g.client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

// WriteFile writes JSON data to an object in a GCS bucket
func (g *GCSClient) WriteFile(ctx context.Context, bucket, object string, data []byte) error {
	writer := g.client.Bucket(bucket).Object(object).NewWriter(ctx)
	defer writer.Close()

	_, err := writer.Write(data)
	return err
}

// Close gracefully closes the client
func (g *GCSClient) Close() {
	if g.client != nil {
		g.client.Close()
	}
}
