package storage

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/option"
)

// SecretManagerClient wraps the GCP Secret Manager logic
type SecretManagerClient struct {
	client *secretmanager.Client
}

// NewSecretManagerClient initializes the SM client connection
func NewSecretManagerClient(ctx context.Context, opts ...option.ClientOption) (*SecretManagerClient, error) {
	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &SecretManagerClient{client: client}, nil
}

// GetPassword fetches the global UI authentication password from SM
func (sm *SecretManagerClient) GetPassword(ctx context.Context, projectID, secretName string) (string, error) {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, secretName)
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := sm.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", err
	}

	return string(result.Payload.Data), nil
}

// Close gracefully closes the client connection
func (sm *SecretManagerClient) Close() {
	if sm.client != nil {
		sm.client.Close()
	}
}
