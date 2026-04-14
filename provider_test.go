package llmgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_Complete_Success(t *testing.T) {
	srv := mockServer(t, okHandler("test-model"))
	defer srv.Close()

	p := NewProvider("test", srv.URL, "test-key")
	resp, err := p.Complete(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello from test-model", resp.Content())
	assert.Equal(t, "test", resp.ProviderName)
}

func TestProvider_Complete_AuthHeader(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		okHandler("m")(w, r)
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "my-secret-key")
	_, err := p.Complete(context.Background(), &Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
}

func TestProvider_Complete_NoAPIKey(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		okHandler("m")(w, r)
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "")
	_, err := p.Complete(context.Background(), &Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
}

func TestProvider_Complete_ExtraHeaders(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "myapp", r.Header.Get("HTTP-Referer"))
		assert.Equal(t, "MyApp", r.Header.Get("X-Title"))
		okHandler("m")(w, r)
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "key")
	_, err := p.Complete(context.Background(), &Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hi"}},
		ExtraHeaders: map[string]string{
			"HTTP-Referer": "myapp",
			"X-Title":      "MyApp",
		},
	})
	require.NoError(t, err)
}

func TestProvider_Complete_ClientError(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid model"}`))
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "key")
	p.maxRetry = 0
	_, err := p.Complete(context.Background(), &Request{
		Model:    "bad",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client error (400)")
}

func TestProvider_Complete_ServerErrorRetries(t *testing.T) {
	var attempts int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"oops"}`))
			return
		}
		okHandler("m")(w, r)
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "key")
	p.maxRetry = 3
	p.initialDelay = 1 * time.Millisecond
	resp, err := p.Complete(context.Background(), &Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello from m", resp.Content())
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestProvider_Complete_RateLimitRetries(t *testing.T) {
	var attempts int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		okHandler("m")(w, r)
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "key")
	p.maxRetry = 2
	p.initialDelay = 1 * time.Millisecond
	resp, err := p.Complete(context.Background(), &Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts))
	assert.Equal(t, "Hello from m", resp.Content())
}

func TestProvider_Complete_ContextCancelled(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	p := NewProvider("test", srv.URL, "key")
	p.maxRetry = 0
	_, err := p.Complete(ctx, &Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
}

func TestProvider_CircuitBreaker(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad"}`))
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "key")
	p.maxRetry = 0
	p.maxFailures = 2
	p.resetTimeout = 100 * time.Millisecond

	req := &Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}

	// Trip the circuit breaker
	p.Complete(context.Background(), req)
	p.Complete(context.Background(), req)

	// Should be open now
	_, err := p.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker open")

	// Wait for reset
	time.Sleep(150 * time.Millisecond)

	// Half-open: should try again
	_, err = p.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "circuit breaker open")
}

func TestProvider_Complete_BadJSON(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "key")
	p.maxRetry = 0
	_, err := p.Complete(context.Background(), &Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestProvider_Complete_ConnectionRefused(t *testing.T) {
	p := NewProvider("test", "http://localhost:1", "key")
	p.maxRetry = 0
	_, err := p.Complete(context.Background(), &Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
}

func TestProvider_Complete_RequestBody(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "gpt-4", req.Model)
		assert.Len(t, req.Messages, 1)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "hi", req.Messages[0].Content)
		assert.Equal(t, 100, req.MaxTokens)
		okHandler("gpt-4")(w, r)
	})
	defer srv.Close()

	p := NewProvider("test", srv.URL, "key")
	_, err := p.Complete(context.Background(), &Request{
		Model:     "gpt-4",
		Messages:  []Message{{Role: "user", Content: "hi"}},
		MaxTokens: 100,
	})
	require.NoError(t, err)
}

func TestProvider_Name(t *testing.T) {
	p := NewProvider("myname", "url", "key")
	assert.Equal(t, "myname", p.Name())
}

func TestProvider_BaseURL(t *testing.T) {
	p := NewProvider("n", "https://api.example.com/v1", "key")
	assert.Equal(t, "https://api.example.com/v1", p.BaseURL())
}

func TestRetryableError(t *testing.T) {
	err := &RetryableError{Err: assert.AnError}
	assert.Equal(t, assert.AnError.Error(), err.Error())
	assert.Equal(t, assert.AnError, err.Unwrap())
	assert.True(t, isRetryable(err))
}

func TestIsRetryable_NonRetryable(t *testing.T) {
	assert.False(t, isRetryable(assert.AnError))
	assert.False(t, isRetryable(nil))
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 5))
	assert.Equal(t, "ab...", truncate("abcde", 2))
	assert.Equal(t, "abcde", truncate("abcde", 10))
}

func TestBackoffDelay(t *testing.T) {
	p := NewProvider("test", "", "")
	p.initialDelay = 100 * time.Millisecond
	p.maxDelay = 1 * time.Second

	d1 := p.backoffDelay(1)
	d2 := p.backoffDelay(2)
	d3 := p.backoffDelay(3)

	assert.Equal(t, 100*time.Millisecond, d1)
	assert.Equal(t, 200*time.Millisecond, d2)
	assert.Equal(t, 400*time.Millisecond, d3)

	// Should cap at maxDelay
	d10 := p.backoffDelay(20)
	assert.Equal(t, 1*time.Second, d10)
}
