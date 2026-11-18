// Copyright (c) HashiCorp, Inc.

package provider

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// The upstream NetBird REST SDK defaults to http.DefaultClient (no timeout)
// and performs a single Do() with no retries. Against a flaky management API
// that means a stalled response blocks forever (hanging plan/apply), and
// transient 429/5xx/connection errors fail the whole run. This transport adds
// a per-attempt timeout plus bounded, idempotency-aware retries with backoff.
type retryTransport struct {
	inner         http.RoundTripper
	maxRetries    int
	perTryTimeout time.Duration
	waitMin       time.Duration
	waitMax       time.Duration
}

// newRetryingHTTPClient builds an *http.Client (satisfies the SDK's HttpClient
// interface) backed by retryTransport. No client-level Timeout is set: each
// attempt is bounded by perTryTimeout via context, so the retry budget is not
// cut short by a single overall deadline.
func newRetryingHTTPClient(perTryTimeout time.Duration, maxRetries int) *http.Client {
	// Guard against a non-positive timeout, which would make every attempt
	// expire immediately and trigger a retry storm. Callers normally enforce
	// this (request_timeout >= 1), but keep the constructor safe on its own.
	if perTryTimeout <= 0 {
		perTryTimeout = defaultRequestTimeoutSeconds * time.Second
	}
	return &http.Client{
		Transport: &retryTransport{
			inner:         http.DefaultTransport,
			maxRetries:    maxRetries,
			perTryTimeout: perTryTimeout,
			waitMin:       1 * time.Second,
			waitMax:       15 * time.Second,
		},
	}
}

// idempotent reports whether a method is safe to retry after a 5xx or a
// transport error. POST/PATCH are excluded: a request that succeeded
// server-side but failed on the response could be double-applied (e.g. a
// duplicate group/peer/setup-key) if retried.
func idempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func (t *retryTransport) shouldRetry(method string, resp *http.Response, err error) bool {
	if err != nil {
		// Transport error or per-attempt timeout. 429 not reached here.
		return idempotent(method)
	}
	if resp == nil {
		return false
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		// Rate limited: the request was not processed, safe for any method.
		return true
	case resp.StatusCode >= 500 && resp.StatusCode <= 599:
		return idempotent(method)
	default:
		return false
	}
}

// backoff returns the wait before the next attempt: exponential (waitMin<<n)
// capped at waitMax, with jitter. A Retry-After header (429/503) takes
// precedence when present.
func (t *retryTransport) backoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 {
				d := time.Duration(secs) * time.Second
				if d > t.waitMax {
					d = t.waitMax
				}
				return d
			}
		}
	}
	wait := t.waitMin << uint(attempt)
	if wait <= 0 || wait > t.waitMax {
		wait = t.waitMax
	}
	// Full jitter in [wait/2, wait].
	half := wait / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}

func drain(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Buffer the body once so each attempt can replay it.
	var bodyBytes []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
		bodyBytes = b
	}

	var resp *http.Response
	var err error

	for attempt := 0; ; attempt++ {
		attemptReq := req.Clone(req.Context())
		if bodyBytes != nil {
			attemptReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			attemptReq.ContentLength = int64(len(bodyBytes))
		}

		// Per-attempt timeout derived from the caller's context, so Terraform
		// cancellation (Ctrl-C) still aborts immediately.
		ctx, cancel := context.WithTimeout(req.Context(), t.perTryTimeout)
		resp, err = t.inner.RoundTrip(attemptReq.WithContext(ctx))

		retry := t.shouldRetry(attemptReq.Method, resp, err) &&
			attempt < t.maxRetries &&
			req.Context().Err() == nil // outer cancel/deadline -> stop

		if !retry {
			if err != nil {
				cancel()
				return nil, err
			}
			// Defer cancel until the caller closes the body, otherwise the
			// per-attempt timer would abort an in-progress body read.
			resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
			return resp, nil
		}

		// Retrying: free this attempt's resources, then wait.
		drain(resp)
		wait := t.backoff(attempt, resp)
		cancel()

		timer := time.NewTimer(wait)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
}

// cancelOnCloseBody ties the per-attempt context's cancel to body Close so the
// context is released once the caller is done reading the response.
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
