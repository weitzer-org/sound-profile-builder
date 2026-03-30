package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/api/option"
)

func TestSecretManagerClient_GetPassword_Success(t *testing.T) {
	// Create a mock HTTP server returning 200 OK and expected JSON Payload
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{ "name": "projects/foo/secrets/bar/versions/latest", "payload": { "data": "c2VjcmV0LWhpZGRlbg==" } }`))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client, err := NewSecretManagerClient(ctx, option.WithEndpoint(mockServer.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	secret, err := client.GetPassword(ctx, "foo", "bar")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if secret != "secret-hidden" {
		t.Errorf("Expected secret-hidden, got %s", secret)
	}
}

func TestSecretManagerClient_GetPassword_Error(t *testing.T) {
	// Create a mock HTTP server returning 404
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{ "error": { "code": 404, "message": "Not found" } }`))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client, err := NewSecretManagerClient(ctx, option.WithEndpoint(mockServer.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	_, err = client.GetPassword(ctx, "foo", "bar")
	if err == nil {
		t.Fatalf("Expected error for 404, got nil")
	}
}

func TestSecretManagerClient_NewClient_Error(t *testing.T) {
	ctx := context.Background()
	// Pass an invalid config credential file approach to trigger err
	_, err := NewSecretManagerClient(ctx, option.WithCredentialsFile("doesnotexist.json"))
	if err == nil {
		t.Fatalf("Expected err creating client with invalid creds, got nil")
	}
}

func TestSecretManagerClient_Close(t *testing.T) {
	ctx := context.Background()
	client, _ := NewSecretManagerClient(ctx, option.WithoutAuthentication())
	// Ensure Close does not panic
	client.Close()

	// Ensure calling close on nil is safe (as handled in the package)
	var nilClient *SecretManagerClient
	nilClient = &SecretManagerClient{client: nil}
	nilClient.Close()
}
