package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newRetryTestClient(maxRetries int, perTry time.Duration) *http.Client {
	return &http.Client{
		Transport: &retryTransport{
			inner:         http.DefaultTransport,
			maxRetries:    maxRetries,
			perTryTimeout: perTry,
			waitMin:       time.Millisecond,
			waitMax:       5 * time.Millisecond,
		},
	}
}

func doReq(t *testing.T, c *http.Client, method, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestRetry_GET5xxThenOK(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp := doReq(t, newRetryTestClient(3, 2*time.Second), http.MethodGet, srv.URL)
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after retry, got %v", resp)
	}
	if got := atomic.LoadInt32(&n); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestRetry_POST5xxNotRetried(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	resp := doReq(t, newRetryTestClient(3, 2*time.Second), http.MethodPost, srv.URL)
	if resp == nil || resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 returned, got %v", resp)
	}
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Fatalf("POST 5xx must not retry: expected 1 attempt, got %d", got)
	}
}

func TestRetry_POST429Retried(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	resp := doReq(t, newRetryTestClient(3, 2*time.Second), http.MethodPost, srv.URL)
	if resp == nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 after 429 retry, got %v", resp)
	}
	if got := atomic.LoadInt32(&n); got != 2 {
		t.Fatalf("429 should retry (not processed): expected 2 attempts, got %d", got)
	}
}

func TestRetry_PerAttemptTimeoutThenBounded(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&n, 1)
		time.Sleep(200 * time.Millisecond) // exceeds per-attempt timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp := doReq(t, newRetryTestClient(2, 30*time.Millisecond), http.MethodGet, srv.URL)
	if resp != nil {
		t.Fatalf("expected timeout error, got status %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&n); got != 3 { // 1 initial + 2 retries
		t.Fatalf("expected 3 bounded attempts, got %d", got)
	}
}
