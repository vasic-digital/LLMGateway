// Package llmgateway provides a generic, OpenAI-compatible LLM gateway
// for calling any LLM provider API. It supports multi-provider routing,
// automatic retry with exponential backoff, circuit breaker protection,
// and environment-variable-based provider discovery.
//
// Usage:
//
//	gw := llmgateway.New(
//	    llmgateway.WithProvider("openrouter", "https://openrouter.ai/api/v1", os.Getenv("OPENROUTER_API_KEY")),
//	    llmgateway.WithProvider("groq", "https://api.groq.com/openai/v1", os.Getenv("GROQ_API_KEY")),
//	)
//	resp, err := gw.Complete(ctx, &llmgateway.Request{
//	    Model:    "deepseek/deepseek-chat",
//	    Messages: []llmgateway.Message{{Role: "user", Content: "Hello"}},
//	})
package llmgateway

import (
	"context"
	"fmt"
	"sync"
)

// Gateway is the main entry point for LLM API calls. It manages multiple
// providers and routes requests to the appropriate one based on configuration.
type Gateway struct {
	mu        sync.RWMutex
	providers map[string]*Provider
	fallback  []string // ordered list of provider names for fallback
	maxRetry  int
}

// Option configures a Gateway at construction time.
type Option func(*Gateway)

// WithProvider registers a provider with the gateway.
func WithProvider(name, baseURL, apiKey string) Option {
	return func(g *Gateway) {
		g.providers[name] = NewProvider(name, baseURL, apiKey)
	}
}

// WithMaxRetry sets the maximum number of retry attempts per provider.
func WithMaxRetry(n int) Option {
	return func(g *Gateway) {
		if n > 0 {
			g.maxRetry = n
		}
	}
}

// WithFallbackOrder sets the order in which providers are tried on failure.
func WithFallbackOrder(names ...string) Option {
	return func(g *Gateway) {
		g.fallback = names
	}
}

// New creates a Gateway with the given options.
func New(opts ...Option) *Gateway {
	g := &Gateway{
		providers: make(map[string]*Provider),
		maxRetry:  3,
	}
	for _, o := range opts {
		o(g)
	}
	return g
}

// Complete sends a chat completion request to the best available provider.
// If the request specifies a provider (via Provider field), that provider
// is used directly. Otherwise, providers are tried in fallback order.
func (g *Gateway) Complete(ctx context.Context, req *Request) (*Response, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if req.Provider != "" {
		p, ok := g.providers[req.Provider]
		if !ok {
			return nil, fmt.Errorf("provider %q not registered", req.Provider)
		}
		return p.Complete(ctx, req)
	}

	order := g.fallback
	if len(order) == 0 {
		for name := range g.providers {
			order = append(order, name)
		}
	}

	var lastErr error
	for _, name := range order {
		p, ok := g.providers[name]
		if !ok {
			continue
		}
		resp, err := p.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed: %w", lastErr)
	}
	return nil, fmt.Errorf("no providers available")
}

// RegisterProvider adds a provider at runtime (thread-safe).
func (g *Gateway) RegisterProvider(name, baseURL, apiKey string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.providers[name] = NewProvider(name, baseURL, apiKey)
}

// ProviderNames returns the names of all registered providers.
func (g *Gateway) ProviderNames() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	names := make([]string, 0, len(g.providers))
	for name := range g.providers {
		names = append(names, name)
	}
	return names
}

// ProviderCount returns the number of registered providers.
func (g *Gateway) ProviderCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.providers)
}
