package uploader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Client handles uploading payloads to the analytics endpoint with retry logic.
type Client struct {
	endpoint   string
	timeout    time.Duration
	maxRetries int
	backoff    time.Duration
}

// NewClient creates an upload client.
//
//   - endpoint: the full URL to POST payloads to.
//   - timeout: per-request HTTP timeout.
//   - backoff: base duration for exponential backoff between retries.
//   - maxRetries: maximum number of retry attempts after the initial request.
func NewClient(endpoint string, timeout, backoff time.Duration, maxRetries int) *Client {
	return &Client{
		endpoint:   endpoint,
		timeout:    timeout,
		maxRetries: maxRetries,
		backoff:    backoff,
	}
}

// Upload marshals the payload to JSON and POSTs it to the configured endpoint.
// It retries on 5xx errors and transient network failures with exponential
// backoff. 4xx errors (except 409 Conflict, treated as success) are not
// retried.
func (c *Client) Upload(ctx context.Context, payload *Payload, authHeader string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			wait := c.backoff * (1 << (attempt - 1))
			slog.Info("retrying upload",
				"attempt", attempt+1,
				"backoff", wait.String(),
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		lastErr = c.doRequest(ctx, body, payload.Metadata.BatchID, authHeader)
		if lastErr == nil {
			return nil
		}

		// If the error is a non-retryable HTTP status, return immediately.
		if he, ok := lastErr.(*httpError); ok && !he.retryable {
			return lastErr
		}

		slog.Warn("upload attempt failed",
			"attempt", attempt+1,
			"error", lastErr,
		)
	}

	return fmt.Errorf("upload failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// doRequest performs a single HTTP POST and interprets the response.
func (c *Client) doRequest(ctx context.Context, body []byte, batchID, authHeader string) error {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Batch-ID", batchID)

	slog.Debug("sending upload request",
		"endpoint", c.endpoint,
		"batch_id", batchID,
		"body_bytes", len(body),
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Network errors are retryable.
		return &httpError{
			status:    0,
			message:   err.Error(),
			retryable: true,
		}
	}
	defer resp.Body.Close()

	// Read a bounded amount of the response body for diagnostics.
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	slog.Debug("upload response",
		"status", resp.StatusCode,
		"body", string(respBody),
	)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusConflict:
		// 409 Conflict means the batch was already received; treat as success.
		slog.Info("batch already uploaded (409 Conflict)", "batch_id", batchID)
		return nil
	case resp.StatusCode >= 500:
		return &httpError{
			status:    resp.StatusCode,
			message:   fmt.Sprintf("server error %d: %s", resp.StatusCode, string(respBody)),
			retryable: true,
		}
	default:
		// 4xx (other than 409) — do not retry.
		return &httpError{
			status:    resp.StatusCode,
			message:   fmt.Sprintf("client error %d: %s", resp.StatusCode, string(respBody)),
			retryable: false,
		}
	}
}

// httpError captures an HTTP response status for retry decisions.
type httpError struct {
	status    int
	message   string
	retryable bool
}

func (e *httpError) Error() string {
	return e.message
}
