# CLAUDE.md

## Project Overview

LLMGateway is a generic, OpenAI-compatible LLM gateway for Go. It provides a unified interface for calling any LLM provider API with multi-provider fallback, automatic retry, circuit breaker protection, and environment-variable-based auto-discovery.

## MANDATORY: No CI/CD Pipelines

No GitHub Actions, GitLab CI/CD, or any automated pipeline may exist in this repository. All builds and tests are run manually.

## Definition of Done

A change is NOT done because code compiles and tests pass. "Done" requires pasted
terminal output from a real run of the real system, produced in the same session as
the change. Coverage and passing suites measure the LLM's model of the product, not
the product.

1. **No self-certification.** *Verified, tested, working, complete, fixed, passing*
   are forbidden in commits, PRs, and agent replies without accompanying pasted
   output from a same-session real-system run.
2. **Demo before code.** Every task begins with the runnable acceptance demo below.
3. **Real system.** Demos run against a real built binary hitting a real upstream
   LLM (or a locally-running compatible mock) — not `httptest.NewServer` or
   `http.RoundTripper` stubs as proof-of-done.
4. **Skips are loud.** `t.Skip(...)` without `SKIP-OK: #<ticket>` fails
   `scripts/no-silent-skips.sh`.
5. **Contract tests on every seam.** Any change touching the Gateway ↔ Provider
   boundary must include a roundtrip test hitting a real HTTP surface.
6. **Evidence in the PR.** PR body contains a fenced `## Demo` block with exact
   command(s) + output.

### Acceptance demo for this module

```bash
# LLMGateway acceptance demo: auto-discover providers from env, execute a real
# chat completion round-trip, assert the response has the expected shape.
set -e
cd "$(dirname "$0")" 2>/dev/null || true

# Build
go build ./... && go vet ./... && go test ./... -race -count=1 -short -timeout 60s

# Live round-trip (skips if no keys present — but visibly, not silently)
if [ -z "${OPENROUTER_API_KEY:-}${OPENAI_API_KEY:-}${GROQ_API_KEY:-}" ]; then
  echo "demo-live-skipped: no OPENROUTER_API_KEY / OPENAI_API_KEY / GROQ_API_KEY set"
  echo "demo-live-skipped: SKIP-OK: no-upstream-keys-available-in-env"
  exit 0
fi

# Write a same-package smoke inside a _demo file so it compiles against the
# local source without a separate module boundary. _demo files are ignored by
# `go test` but are built by `go run`.
cat > /tmp/llmgateway_smoke_test.sh <<'BASHEOF'
#!/usr/bin/env bash
set -e
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp go.mod go.sum *.go "$work/"
# Remove *_test.go so the demo compiles standalone.
find "$work" -name '*_test.go' -delete
cat > "$work/smoke_demo_main.go" <<'GOEOF'
//go:build ignore
// +build ignore

package main

import (
  "context"
  "encoding/json"
  "fmt"
  "os"
  "time"

  llm "digital.vasic.llmgateway"
)

func main() {
  g := llm.NewFromEnv()
  ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
  defer cancel()
  resp, err := g.Complete(ctx, &llm.Request{
    Messages:  []llm.Message{{Role: "user", Content: "Reply with the single word OK"}},
    MaxTokens: 20,
  })
  if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
  b, _ := json.Marshal(resp)
  fmt.Println(string(b))
  if len(resp.Choices) == 0 { os.Exit(2) }
}
GOEOF
cd "$work" && go run smoke_demo_main.go
BASHEOF
chmod +x /tmp/llmgateway_smoke_test.sh
/tmp/llmgateway_smoke_test.sh
```

Expect: `go build`, `go vet`, `go test ./... -short` all pass; the live round-trip
emits a JSON object containing `choices[0].message.content`. Any failure returns
non-zero from the demo.

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
