package storage

import (
	"context"
	"testing"
	"time"

	"google.golang.org/api/option"
)

func TestGCSClient_NewGCSClient_Error(t *testing.T) {
	ctx := context.Background()
	_, err := NewGCSClient(ctx, option.WithCredentialsFile("doesnotexist.json"))
	if err == nil {
		t.Fatalf("Expected err creating GCS client with invalid creds, got nil")
	}
}

func TestGCSClient_CoverageTimeouts(t *testing.T) {
	// Initialize a client that points nowhere, ensuring fast errors
	ctx := context.Background()
	client, err := NewGCSClient(ctx, option.WithEndpoint("http://127.0.0.1:0"), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("Failed to create dummy client: %v", err)
	}

	// Trigger Reader execution and timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	_, err = client.ReadFile(ctxTimeout, "bucket", "object")
	if err == nil {
		t.Errorf("Expected ReadFile to fail via context deadline")
	}

	// Trigger Write execution
	err = client.WriteFile(ctxTimeout, "bucket", "object", []byte("data"))
	if err == nil {
		t.Errorf("Expected WriteFile to fail via context deadline")
	}

	// Trigger List execution
	_, err = client.ListFiles(ctxTimeout, "bucket", "prefix")
	if err == nil {
		t.Errorf("Expected ListFiles to fail via context deadline")
	}

	// Trigger Delete execution
	err = client.DeleteFile(ctxTimeout, "bucket", "object")
	if err == nil {
		t.Errorf("Expected DeleteFile to fail via context deadline")
	}

	client.Close()

	// Safe nil close test
	var n *GCSClient
	n.Close()
}
