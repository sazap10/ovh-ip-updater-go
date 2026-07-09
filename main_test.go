package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v7"
)

// fastBackoff returns retry options that retry almost instantly and cap the
// number of attempts, so backoff behaviour can be exercised without waiting on
// the real exponential schedule.
func fastBackoff(maxTries uint) []backoff.RetryOption {
	return []backoff.RetryOption{
		backoff.WithBackOff(backoff.NewConstantBackOff(time.Millisecond)),
		backoff.WithMaxTries(maxTries),
	}
}

func TestGetIPAddress_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("203.0.113.7"))
	}))
	defer srv.Close()

	ip, err := getIPAddress(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "203.0.113.7" {
		t.Fatalf("got %q, want %q", ip, "203.0.113.7")
	}
}

func TestGetIPAddress_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := getIPAddress(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("expected an error for a non-200 response, got nil")
	}
}

// Backoff retries on transient failures and eventually returns the success
// value once the operation stops erroring.
func TestGetIPAddressWithRetry_EventualSuccess(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("198.51.100.2"))
	}))
	defer srv.Close()

	ip, err := getIPAddressWithRetry(context.Background(), srv.Client(), srv.URL, fastBackoff(5)...)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "198.51.100.2" {
		t.Fatalf("got %q, want %q", ip, "198.51.100.2")
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("expected operation to be attempted 3 times, got %d", got)
	}
}

// When retries are exhausted the caller-facing error wraps the underlying
// failure. v7 returns a *backoff.RetryError carrying the cause and last error,
// both of which must remain discoverable through errors.As/errors.Is.
func TestGetIPAddressWithRetry_Exhausted(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := getIPAddressWithRetry(context.Background(), srv.Client(), srv.URL, fastBackoff(3)...)
	if err == nil {
		t.Fatal("expected an error after retries are exhausted, got nil")
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("expected 3 attempts before giving up, got %d", got)
	}

	// v7 wraps every failure in a *RetryError; confirm it survives main.go's
	// errors.Wrapf and reports the exhausted cause.
	var retryErr *backoff.RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("expected a *backoff.RetryError in the chain, got %T: %v", err, err)
	}
	if !errors.Is(err, backoff.ErrExhausted) {
		t.Fatalf("expected ErrExhausted cause, got %v", retryErr.Cause)
	}
}

// A permanent error stops retrying immediately rather than consuming the full
// attempt budget.
func TestGetIPAddressWithRetry_Permanent(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	operation := func() (string, error) {
		_, err := getIPAddress(context.Background(), srv.Client(), srv.URL)
		return "", backoff.Permanent(err)
	}
	_, err := backoff.Retry(context.Background(), operation, fastBackoff(5)...)
	if err == nil {
		t.Fatal("expected a permanent error, got nil")
	}
	if !errors.Is(err, backoff.ErrPermanent) {
		t.Fatalf("expected ErrPermanent cause, got %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected a permanent error to stop after 1 attempt, got %d", got)
	}
}

// A cancelled context stops the retry loop and surfaces the cancellation cause.
func TestGetIPAddressWithRetry_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := getIPAddressWithRetry(ctx, srv.Client(), srv.URL, fastBackoff(5)...)
	if err == nil {
		t.Fatal("expected an error for a cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled in the chain, got %v", err)
	}
}

func TestSetDyndnsIPAddressWithRetry_Success(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if got := r.URL.Query().Get("myip"); got != "192.0.2.10" {
			t.Errorf("myip query = %q, want %q", got, "192.0.2.10")
		}
		if _, _, ok := r.BasicAuth(); !ok {
			t.Error("expected basic auth to be set on the request")
		}
		_, _ = w.Write([]byte("good 192.0.2.10"))
	}))
	defer srv.Close()

	req := dynDNSRequest{
		ipAddress:  "192.0.2.10",
		domainName: "example.com",
		username:   "user",
		password:   "pass",
	}
	if err := setDyndnsIPAddressWithRetry(context.Background(), srv.Client(), srv.URL, req, fastBackoff(3)...); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected a single successful attempt, got %d", got)
	}
}

func TestSetDyndnsIPAddressWithRetry_Exhausted(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	req := dynDNSRequest{ipAddress: "192.0.2.10", domainName: "example.com"}
	err := setDyndnsIPAddressWithRetry(context.Background(), srv.Client(), srv.URL, req, fastBackoff(2)...)
	if err == nil {
		t.Fatal("expected an error after retries are exhausted, got nil")
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected 2 attempts before giving up, got %d", got)
	}
	if !errors.Is(err, backoff.ErrExhausted) {
		t.Fatalf("expected ErrExhausted cause, got %v", err)
	}
}
