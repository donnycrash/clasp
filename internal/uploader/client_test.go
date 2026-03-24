package uploader

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestUpload_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second, 1*time.Millisecond, 3)
	payload := &Payload{
		Metadata: PayloadMetadata{BatchID: "test-batch-1"},
		Sessions: []PayloadSession{},
	}

	err := client.Upload(context.Background(), payload, "Bearer token123")
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
}

func TestUpload_Created(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second, 1*time.Millisecond, 3)
	payload := &Payload{
		Metadata: PayloadMetadata{BatchID: "test-batch-2"},
		Sessions: []PayloadSession{},
	}

	err := client.Upload(context.Background(), payload, "Bearer token123")
	if err != nil {
		t.Fatalf("Upload returned error for 201: %v", err)
	}
}

func TestUpload_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second, 1*time.Millisecond, 3)
	payload := &Payload{
		Metadata: PayloadMetadata{BatchID: "test-batch-3"},
		Sessions: []PayloadSession{},
	}

	err := client.Upload(context.Background(), payload, "Bearer token123")
	if err != nil {
		t.Fatalf("Upload returned error for 409 Conflict: %v", err)
	}
}

func TestUpload_ClientError(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second, 1*time.Millisecond, 3)
	payload := &Payload{
		Metadata: PayloadMetadata{BatchID: "test-batch-4"},
		Sessions: []PayloadSession{},
	}

	err := client.Upload(context.Background(), payload, "Bearer token123")
	if err == nil {
		t.Fatal("Upload should return error for 400")
	}
	if !strings.Contains(err.Error(), "client error 400") {
		t.Errorf("error = %q, want it to contain 'client error 400'", err.Error())
	}
	// Should not retry on 4xx.
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("request count = %d, want 1 (no retries for 4xx)", got)
	}
}

func TestUpload_ServerError_ThenSuccess(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second, 1*time.Millisecond, 3)
	payload := &Payload{
		Metadata: PayloadMetadata{BatchID: "test-batch-5"},
		Sessions: []PayloadSession{},
	}

	err := client.Upload(context.Background(), payload, "Bearer token123")
	if err != nil {
		t.Fatalf("Upload should succeed after retry, got: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Errorf("request count = %d, want 2", got)
	}
}

func TestUpload_AllRetriesFail(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server down"))
	}))
	defer srv.Close()

	maxRetries := 2
	client := NewClient(srv.URL, 5*time.Second, 1*time.Millisecond, maxRetries)
	payload := &Payload{
		Metadata: PayloadMetadata{BatchID: "test-batch-6"},
		Sessions: []PayloadSession{},
	}

	err := client.Upload(context.Background(), payload, "Bearer token123")
	if err == nil {
		t.Fatal("Upload should return error when all retries fail")
	}
	if !strings.Contains(err.Error(), "upload failed after") {
		t.Errorf("error = %q, want it to contain 'upload failed after'", err.Error())
	}
	// Initial attempt + maxRetries retries.
	expectedCalls := int32(maxRetries + 1)
	if got := atomic.LoadInt32(&callCount); got != expectedCalls {
		t.Errorf("request count = %d, want %d", got, expectedCalls)
	}
}

func TestUpload_RequestHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer my-secret-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer my-secret-token")
		}
		if batchID := r.Header.Get("X-Batch-ID"); batchID != "batch-abc-123" {
			t.Errorf("X-Batch-ID = %q, want %q", batchID, "batch-abc-123")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second, 1*time.Millisecond, 0)
	payload := &Payload{
		Metadata: PayloadMetadata{BatchID: "batch-abc-123"},
		Sessions: []PayloadSession{},
	}

	err := client.Upload(context.Background(), payload, "Bearer my-secret-token")
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
}

func TestUpload_RequestBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}

		var received Payload
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("unmarshalling request body: %v", err)
		}

		if received.Metadata.ToolName != "clasp" {
			t.Errorf("body ToolName = %q, want %q", received.Metadata.ToolName, "clasp")
		}
		if received.Metadata.BatchID != "batch-body-test" {
			t.Errorf("body BatchID = %q, want %q", received.Metadata.BatchID, "batch-body-test")
		}
		if len(received.Sessions) != 1 {
			t.Errorf("body Sessions length = %d, want 1", len(received.Sessions))
		} else if received.Sessions[0].SessionID != "s1" {
			t.Errorf("body Sessions[0].SessionID = %q, want %q", received.Sessions[0].SessionID, "s1")
		}
		if received.StatsSummary == nil {
			t.Error("body StatsSummary should not be nil")
		} else if received.StatsSummary.PeriodStart != "2025-01-01" {
			t.Errorf("body PeriodStart = %q", received.StatsSummary.PeriodStart)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second, 1*time.Millisecond, 0)
	payload := &Payload{
		Metadata: PayloadMetadata{
			ToolName: "clasp",
			BatchID:  "batch-body-test",
		},
		StatsSummary: &PayloadStats{
			PeriodStart: "2025-01-01",
			PeriodEnd:   "2025-01-31",
		},
		Sessions: []PayloadSession{
			{SessionID: "s1"},
		},
	}

	err := client.Upload(context.Background(), payload, "Bearer token")
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
}

func TestUpload_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 5*time.Second, 100*time.Millisecond, 5)
	payload := &Payload{
		Metadata: PayloadMetadata{BatchID: "test-ctx-cancel"},
		Sessions: []PayloadSession{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the retry backoff will detect cancellation.
	cancel()

	err := client.Upload(ctx, payload, "Bearer token")
	if err == nil {
		t.Fatal("Upload should return error when context is cancelled")
	}
}
