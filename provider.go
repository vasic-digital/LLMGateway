package llmgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

// Provider represents a single LLM API endpoint (e.g. OpenRouter, Groq).
// All providers follow the OpenAI chat completion API format.
type Provider struct {
	name    string
	baseURL string
	apiKey  string
	client  *http.Client

	// Circuit breaker state
	mu               sync.Mutex
	consecutiveFails int
	circuitOpen      bool
	circuitOpenUntil time.Time
	maxFailures      int
	resetTimeout     time.Duration

	// Retry configuration
	maxRetry     int
	initialDelay time.Duration
	maxDelay     time.Duration
}

// NewProvider creates a provider for the given base URL and API key.
func NewProvider(name, baseURL, apiKey string) *Provider {
	return &Provider{
		name:         name,
		baseURL:      baseURL,
		apiKey:       apiKey,
		client:       &http.Client{Timeout: 120 * time.Second},
		maxFailures:  5,
		resetTimeout: 60 * time.Second,
		maxRetry:     3,
		initialDelay: 500 * time.Millisecond,
		maxDelay:     30 * time.Second,
	}
}

// Complete sends a chat completion request to this provider's API.
func (p *Provider) Complete(ctx context.Context, req *Request) (*Response, error) {
	if p.isCircuitOpen() {
		return nil, fmt.Errorf("provider %s: circuit breaker open", p.name)
	}

	var lastErr error
	for attempt := 0; attempt <= p.maxRetry; attempt++ {
		if attempt > 0 {
			delay := p.backoffDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := p.doComplete(ctx, req)
		if err == nil {
			p.recordSuccess()
			resp.ProviderName = p.name
			return resp, nil
		}

		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	p.recordFailure()
	return nil, fmt.Errorf("provider %s: %w", p.name, lastErr)
}

func (p *Provider) doComplete(ctx context.Context, req *Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	for k, v := range req.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &RetryableError{Err: fmt.Errorf("http request: %w", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == 429 {
		return nil, &RetryableError{Err: fmt.Errorf("rate limited (429)")}
	}
	if resp.StatusCode >= 500 {
		return nil, &RetryableError{Err: fmt.Errorf("server error (%d): %s", resp.StatusCode, truncate(string(respBody), 200))}
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("client error (%d): %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var result Response
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// Name returns the provider's name.
func (p *Provider) Name() string { return p.name }

// BaseURL returns the provider's base URL.
func (p *Provider) BaseURL() string { return p.baseURL }

// --- Circuit breaker ---

func (p *Provider) isCircuitOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.circuitOpen {
		return false
	}
	if time.Now().After(p.circuitOpenUntil) {
		// Half-open: allow one probe request
		p.circuitOpen = false
		p.consecutiveFails = p.maxFailures - 1 // next failure re-opens
		return false
	}
	return true
}

func (p *Provider) recordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.consecutiveFails = 0
	p.circuitOpen = false
}

func (p *Provider) recordFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.consecutiveFails++
	if p.consecutiveFails >= p.maxFailures {
		p.circuitOpen = true
		p.circuitOpenUntil = time.Now().Add(p.resetTimeout)
	}
}

// --- Retry helpers ---

func (p *Provider) backoffDelay(attempt int) time.Duration {
	delay := time.Duration(float64(p.initialDelay) * math.Pow(2, float64(attempt-1)))
	if delay > p.maxDelay {
		delay = p.maxDelay
	}
	return delay
}

// RetryableError indicates an error that should be retried.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

func isRetryable(err error) bool {
	var re *RetryableError
	if ok := false; err != nil {
		for e := err; e != nil; e = nil {
			if _, ok = e.(*RetryableError); ok {
				return true
			}
		}
	}
	_ = re
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
