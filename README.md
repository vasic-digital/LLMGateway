# LLMGateway

Generic, OpenAI-compatible LLM gateway for Go. Call any LLM provider API with a single unified interface — multi-provider routing, automatic retry with exponential backoff, circuit breaker protection, and environment-variable-based auto-discovery.

## Features

- **OpenAI-compatible** — works with any provider that implements the `/chat/completions` endpoint (OpenRouter, Groq, DeepSeek, Mistral, Cerebras, NVIDIA, Cohere, SambaNova, and 30+ more)
- **Multi-provider fallback** — try providers in order, first success wins
- **Automatic retry** — exponential backoff with configurable max retries
- **Circuit breaker** — trips after consecutive failures, auto-resets after timeout
- **Env-based discovery** — scans `*_API_KEY` environment variables to auto-register providers
- **Zero dependencies** — only `net/http` and stdlib (testify for tests only)
- **Thread-safe** — all operations safe for concurrent use
- **Tool calling** — full OpenAI function/tool calling support

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    gw "digital.vasic.llmgateway"
)

func main() {
    // Auto-discover providers from environment variables
    gateway := gw.NewFromEnv()

    resp, err := gateway.Complete(context.Background(), &gw.Request{
        Model:    "deepseek/deepseek-chat",
        Messages: []gw.Message{{Role: "user", Content: "Explain circuit breakers in Go"}},
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Content())
    fmt.Printf("Tokens: %d, Provider: %s\n", resp.TotalTokens(), resp.ProviderName)
}
```

## Installation

```sh
go get digital.vasic.llmgateway
```

## Usage

### Manual Provider Registration

```go
gateway := gw.New(
    gw.WithProvider("openrouter", "https://openrouter.ai/api/v1", os.Getenv("OPENROUTER_API_KEY")),
    gw.WithProvider("groq", "https://api.groq.com/openai/v1", os.Getenv("GROQ_API_KEY")),
    gw.WithProvider("deepseek", "https://api.deepseek.com/v1", os.Getenv("DEEPSEEK_API_KEY")),
    gw.WithFallbackOrder("groq", "deepseek", "openrouter"),
    gw.WithMaxRetry(3),
)
```

### Environment Auto-Discovery

```go
// Scans all *_API_KEY env vars and registers matching providers
gateway := gw.NewFromEnv()
```

### Targeting a Specific Provider

```go
resp, err := gateway.Complete(ctx, &gw.Request{
    Provider: "groq",  // use this specific provider
    Model:    "llama-3.3-70b-versatile",
    Messages: []gw.Message{{Role: "user", Content: "Hello"}},
})
```

### Tool Calling

```go
resp, err := gateway.Complete(ctx, &gw.Request{
    Model:    "gpt-4",
    Messages: []gw.Message{{Role: "user", Content: "What's the weather in Paris?"}},
    Tools: []gw.Tool{{
        Type: "function",
        Function: gw.ToolFunction{
            Name:        "get_weather",
            Description: "Get current weather for a city",
            Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{"city": map[string]string{"type": "string"}}},
        },
    }},
})
```

### Runtime Provider Registration

```go
gateway.RegisterProvider("newprovider", "https://api.new.com/v1", "key")
```

## Architecture

```
┌──────────────┐
│  Your App    │
│              │
│  Complete()  │
└──────┬───────┘
       │
┌──────▼───────┐     ┌──────────────┐     ┌──────────────┐
│   Gateway    │────▶│  Provider A  │────▶│ Provider API │
│              │     │  (retry +    │     │ /chat/       │
│  Fallback    │     │   circuit    │     │  completions │
│  routing     │     │   breaker)   │     └──────────────┘
│              │     └──────────────┘
│              │     ┌──────────────┐     ┌──────────────┐
│              │────▶│  Provider B  │────▶│ Provider API │
│              │     │  (fallback)  │     └──────────────┘
└──────────────┘     └──────────────┘
```

## Configuration

### Retry

- Default max retries: 3
- Exponential backoff: 500ms, 1s, 2s, ... capped at 30s
- Retryable: HTTP 429 (rate limit), 5xx (server errors), network errors
- Non-retryable: HTTP 4xx (client errors, except 429)

### Circuit Breaker

- Trips after 5 consecutive failures
- Open for 60 seconds, then half-open (allows one probe)
- Success resets the counter

### Environment Variables

Any environment variable matching `*_API_KEY` with a non-empty value is auto-discovered. Excluded prefixes: `PATREON_`, `WEBHOOK_`, `ADMIN_`, `SNYK_`, `SONAR_`, `SEMGREP_`, `LLMSVERIFIER_`.

## Supported Providers

Any OpenAI-compatible provider works. Known providers with pre-configured URLs:

| Provider | Env Variable | Base URL |
|----------|-------------|----------|
| OpenRouter | `OPENROUTER_API_KEY` | `https://openrouter.ai/api/v1` |
| Groq | `GROQ_API_KEY` | `https://api.groq.com/openai/v1` |
| DeepSeek | `DEEPSEEK_API_KEY` | `https://api.deepseek.com/v1` |
| Cerebras | `CEREBRAS_API_KEY` | `https://api.cerebras.ai/v1` |
| NVIDIA | `NVIDIA_API_KEY` | `https://integrate.api.nvidia.com/v1` |
| Mistral | `MISTRAL_API_KEY` | `https://api.mistral.ai/v1` |
| Cohere | `COHERE_API_KEY` | `https://api.cohere.ai/v2` |
| SambaNova | `SAMBANOVA_API_KEY` | `https://api.sambanova.ai/v1` |
| HuggingFace | `HUGGINGFACE_API_KEY` | `https://api-inference.huggingface.co` |
| And 20+ more... | | |

Unknown providers get `https://api.{name}.com/v1` as fallback URL.

## Testing

```sh
go test ./... -race -count=1
```

## License

See LICENSE file.
