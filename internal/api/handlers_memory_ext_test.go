package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/storage"
)

func TestHandleDeleteMemory_Success(t *testing.T) {
	mockStorage := &mockErrorClient{mockClient: newMockClient()}
	store := storage.NewPresetStore(mockStorage, "b")
	memStore := storage.NewMemoryStore(mockStorage, "bm")
	s := NewServer(store, memStore, mockStorage, &mockSecretFetcher{}, nil, nil)

	// Save a dummy memory
	ctx := context.Background()
	dummy := &storage.Memory{ID: "test_id", Artist: "Test Artist", Critique: "Too bright", Action: "Darken it"}
	memStore.Save(ctx, dummy)

	// Verify it exists first
	listBefore, _ := memStore.List(ctx)
	if len(listBefore) != 1 {
		t.Fatalf("Expected 1 memory before delete, got %d", len(listBefore))
	}

	req, _ := http.NewRequest(http.MethodDelete, "/api/memory/delete?id=test_id", nil)
	rr := httptest.NewRecorder()
	s.handleDeleteMemory().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", rr.Code)
	}

	// Verify it was deleted from store
	listAfter, _ := memStore.List(ctx)
	if len(listAfter) != 0 {
		t.Errorf("Expected 0 memories after delete, got %d", len(listAfter))
	}

	// Verify response contains "No learned rules yet"
	if !strings.Contains(rr.Body.String(), "No learned rules yet") {
		t.Errorf("Expected 'No learned rules yet' message, got %s", rr.Body.String())
	}
}

func TestHandleDeleteMemory_Errors(t *testing.T) {
	// 1. Missing ID (400)
	{
		mockStorage := &mockErrorClient{mockClient: newMockClient()}
		memStore := storage.NewMemoryStore(mockStorage, "bm")
		s := NewServer(nil, memStore, mockStorage, &mockSecretFetcher{}, nil, nil)

		req, _ := http.NewRequest(http.MethodDelete, "/api/memory/delete", nil)
		rr := httptest.NewRecorder()
		s.handleDeleteMemory().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for missing ID, got %d", rr.Code)
		}
	}

	// 2. Delete fail (500)
	{
		mockStorage := &mockErrorClient{mockClient: newMockClient(), failDelete: true}
		memStore := storage.NewMemoryStore(mockStorage, "bm")
		s := NewServer(nil, memStore, mockStorage, &mockSecretFetcher{}, nil, nil)

		req, _ := http.NewRequest(http.MethodDelete, "/api/memory/delete?id=test_id", nil)
		rr := httptest.NewRecorder()
		s.handleDeleteMemory().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected 500 for delete fail, got %d", rr.Code)
		}
	}

	// 3. List fail during re-rendering (500)
	{
		mockStorage := &mockErrorClient{mockClient: newMockClient()}
		memStore := storage.NewMemoryStore(mockStorage, "bm")
		s := NewServer(nil, memStore, mockStorage, &mockSecretFetcher{}, nil, nil)

		// Save dummy so delete works and moves to list
		memStore.Save(context.Background(), &storage.Memory{ID: "test_id"})

		// Now fail list
		mockStorage.failList = true

		req, _ := http.NewRequest(http.MethodDelete, "/api/memory/delete?id=test_id", nil)
		rr := httptest.NewRecorder()
		s.handleDeleteMemory().ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected 500 for list fail after delete, got %d", rr.Code)
		}
	}
}
