package llmgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func okHandler(model string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			ID:    "test-id",
			Model: model,
			Choices: []Choice{{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "Hello from " + model},
				FinishReason: "stop",
			}},
			Usage: Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		})
	}
}

func TestNew_Empty(t *testing.T) {
	gw := New()
	assert.Equal(t, 0, gw.ProviderCount())
	assert.Equal(t, 3, gw.maxRetry)
}

func TestNew_WithProvider(t *testing.T) {
	gw := New(WithProvider("test", "https://example.com", "key"))
	assert.Equal(t, 1, gw.ProviderCount())
	names := gw.ProviderNames()
	assert.Contains(t, names, "test")
}

func TestNew_WithMaxRetry(t *testing.T) {
	gw := New(WithMaxRetry(5))
	assert.Equal(t, 5, gw.maxRetry)
}

func TestNew_WithMaxRetryZero(t *testing.T) {
	gw := New(WithMaxRetry(0))
	assert.Equal(t, 3, gw.maxRetry) // unchanged
}

func TestNew_WithFallbackOrder(t *testing.T) {
	gw := New(WithFallbackOrder("a", "b", "c"))
	assert.Equal(t, []string{"a", "b", "c"}, gw.fallback)
}

func TestGateway_Complete_SpecificProvider(t *testing.T) {
	srv := mockServer(t, okHandler("gpt-4"))
	defer srv.Close()

	gw := New(WithProvider("test", srv.URL, "key"))
	resp, err := gw.Complete(context.Background(), &Request{
		Provider: "test",
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello from gpt-4", resp.Content())
	assert.Equal(t, "test", resp.ProviderName)
	assert.Equal(t, 15, resp.TotalTokens())
	assert.Equal(t, "stop", resp.FinishReason())
}

func TestGateway_Complete_UnknownProvider(t *testing.T) {
	gw := New()
	_, err := gw.Complete(context.Background(), &Request{
		Provider: "nonexistent",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestGateway_Complete_FallbackOrder(t *testing.T) {
	failSrv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad"}`))
	})
	defer failSrv.Close()

	okSrv := mockServer(t, okHandler("fallback-model"))
	defer okSrv.Close()

	gw := New(
		WithProvider("bad", failSrv.URL, "key"),
		WithProvider("good", okSrv.URL, "key"),
		WithFallbackOrder("bad", "good"),
		WithMaxRetry(0),
	)

	resp, err := gw.Complete(context.Background(), &Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "good", resp.ProviderName)
}

func TestGateway_Complete_AllProvidersFail(t *testing.T) {
	failSrv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad"}`))
	})
	defer failSrv.Close()

	gw := New(
		WithProvider("p1", failSrv.URL, "key"),
		WithMaxRetry(0),
	)

	_, err := gw.Complete(context.Background(), &Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all providers failed")
}

func TestGateway_Complete_NoProviders(t *testing.T) {
	gw := New()
	_, err := gw.Complete(context.Background(), &Request{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no providers available")
}

func TestGateway_RegisterProvider(t *testing.T) {
	gw := New()
	assert.Equal(t, 0, gw.ProviderCount())

	gw.RegisterProvider("new", "https://example.com", "key")
	assert.Equal(t, 1, gw.ProviderCount())
	assert.Contains(t, gw.ProviderNames(), "new")
}

func TestResponse_Content_Empty(t *testing.T) {
	r := &Response{}
	assert.Equal(t, "", r.Content())
	assert.Equal(t, "", r.FinishReason())
	assert.Equal(t, 0, r.TotalTokens())
}
