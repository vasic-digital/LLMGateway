# CLAUDE.md

## Project Overview

LLMGateway is a generic, OpenAI-compatible LLM gateway for Go. It provides a unified interface for calling any LLM provider API with multi-provider fallback, automatic retry, circuit breaker protection, and environment-variable-based auto-discovery.

## MANDATORY: No CI/CD Pipelines

No GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository. All builds and tests are run manually.

## Build and Test

```bash
go build ./...
go test ./... -race -count=1
go vet ./...
```

## Architecture

- `gateway.go` — Gateway: multi-provider router with fallback ordering
- `provider.go` — Provider: OpenAI-compatible HTTP client with retry + circuit breaker
- `types.go` — Request/Response types (OpenAI chat completion format)
- `discover.go` — Environment-variable-based auto-discovery of providers

## Key Design Decisions

- **Zero external dependencies** (only stdlib + testify for tests)
- **OpenAI format everywhere** — all providers speak the same /chat/completions protocol
- **Thread-safe** — Gateway and Provider use sync.RWMutex/Mutex
- **Decoupled** — no project-specific types; usable in any Go project
- **Circuit breaker per provider** — 5 failures → open for 60s → half-open probe
- **Exponential backoff** — 500ms base, 2x multiplier, 30s cap
