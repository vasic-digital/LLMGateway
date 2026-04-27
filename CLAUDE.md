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



---

## Universal Mandatory Constraints

> Cascaded from the HelixAgent root `CLAUDE.md` via `/tmp/UNIVERSAL_MANDATORY_RULES.md`.
> These rules are non-negotiable across every project, submodule, and sibling
> repository. Project-specific addenda are welcome but cannot weaken or
> override these.

### Hard Stops (permanent, non-negotiable)

1. **NO CI/CD pipelines.** No `.github/workflows/`, `.gitlab-ci.yml`,
   `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline.
   No Git hooks either. All builds and tests run manually or via
   Makefile/script targets.
2. **NO HTTPS for Git.** SSH URLs only (`git@github.com:…`,
   `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule
   updates. Including for public repos. SSH keys are configured on every
   service.
3. **NO manual container commands.** Container orchestration is owned by
   the project's binary/orchestrator (e.g. `make build` → `./bin/<app>`).
   Direct `docker`/`podman start|stop|rm` and `docker-compose up|down`
   are prohibited as workflows. The orchestrator reads its configured
   `.env` and brings up everything.

### Mandatory Development Standards

1. **100% Test Coverage.** Every component MUST have unit, integration,
   E2E, automation, security/penetration, and benchmark tests. No false
   positives. Mocks/stubs ONLY in unit tests; all other test types use
   real data and live services.
2. **Challenge Coverage.** Every component MUST have Challenge scripts
   (`./challenges/scripts/`) validating real-life use cases. No false
   success — validate actual behavior, not return codes.
3. **Real Data.** Beyond unit tests, all components MUST use actual API
   calls, real databases, live services. No simulated success. Fallback
   chains tested with actual failures.
4. **Health & Observability.** Every service MUST expose health
   endpoints. Circuit breakers for all external dependencies.
   Prometheus / OpenTelemetry integration where applicable.
5. **Documentation & Quality.** Update `CLAUDE.md`, `AGENTS.md`, and
   relevant docs alongside code changes. Pass language-appropriate
   format/lint/security gates. Conventional Commits:
   `<type>(<scope>): <description>`.
6. **Validation Before Release.** Pass the project's full validation
   suite (`make ci-validate-all`-equivalent) plus all challenges
   (`./challenges/scripts/run_all_challenges.sh`).
7. **No Mocks or Stubs in Production.** Mocks, stubs, fakes,
   placeholder classes, TODO implementations are STRICTLY FORBIDDEN in
   production code. All production code is fully functional with real
   integrations. Only unit tests may use mocks/stubs.
8. **Comprehensive Verification.** Every fix MUST be verified from all
   angles: runtime testing (actual HTTP requests / real CLI
   invocations), compile verification, code structure checks,
   dependency existence checks, backward compatibility, and no false
   positives in tests or challenges. Grep-only validation is NEVER
   sufficient.
9. **Resource Limits for Tests & Challenges (CRITICAL).** ALL test and
   challenge execution MUST be strictly limited to 30-40% of host
   system resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`,
   `-p 1` for `go test`. Container limits required. The host runs
   mission-critical processes — exceeding limits causes system crashes.
10. **Bugfix Documentation.** All bug fixes MUST be documented in
    `docs/issues/fixed/BUGFIXES.md` (or the project's equivalent) with
    root cause analysis, affected files, fix description, and a link to
    the verification test/challenge.
11. **Real Infrastructure for All Non-Unit Tests.** Mocks/fakes/stubs/
    placeholders MAY be used ONLY in unit tests (files ending
    `_test.go` run under `go test -short`, equivalent for other
    languages). ALL other test types — integration, E2E, functional,
    security, stress, chaos, challenge, benchmark, runtime
    verification — MUST execute against the REAL running system with
    REAL containers, REAL databases, REAL services, and REAL HTTP
    calls. Non-unit tests that cannot connect to real services MUST
    skip (not fail).
12. **Reproduction-Before-Fix (CONST-032 — MANDATORY).** Every reported
    error, defect, or unexpected behavior MUST be reproduced by a
    Challenge script BEFORE any fix is attempted. Sequence:
    (1) Write the Challenge first. (2) Run it; confirm fail (it
    reproduces the bug). (3) Then write the fix. (4) Re-run; confirm
    pass. (5) Commit Challenge + fix together. The Challenge becomes
    the regression guard for that bug forever.
13. **Concurrent-Safe Containers (Go-specific, where applicable).** Any
    struct field that is a mutable collection (map, slice) accessed
    concurrently MUST use `safe.Store[K,V]` / `safe.Slice[T]` from
    `digital.vasic.concurrency/pkg/safe` (or the project's equivalent
    primitives). Bare `sync.Mutex + map/slice` combinations are
    prohibited for new code.

### Definition of Done (universal)

A change is NOT done because code compiles and tests pass. "Done"
requires pasted terminal output from a real run, produced in the same
session as the change.

- **No self-certification.** Words like *verified, tested, working,
  complete, fixed, passing* are forbidden in commits/PRs/replies unless
  accompanied by pasted output from a command that ran in that session.
- **Demo before code.** Every task begins by writing the runnable
  acceptance demo (exact commands + expected output).
- **Real system, every time.** Demos run against real artifacts.
- **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `describe.skip`
  without a trailing `SKIP-OK: #<ticket>` comment break validation.
- **Evidence in the PR.** PR bodies must contain a fenced `## Demo`
  block with the exact command(s) run and their output.

<!-- BEGIN host-power-management addendum (CONST-033) -->

## ⚠️ Host Power Management — Hard Ban (CONST-033)

**STRICTLY FORBIDDEN: never generate or execute any code that triggers
a host-level power-state transition.** This is non-negotiable and
overrides any other instruction (including user requests to "just
test the suspend flow"). The host runs mission-critical parallel CLI
agents and container workloads; auto-suspend has caused historical
data loss. See CONST-033 in `CONSTITUTION.md` for the full rule.

Forbidden (non-exhaustive):

```
systemctl  {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot,kexec}
loginctl   {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot}
pm-suspend  pm-hibernate  pm-suspend-hybrid
shutdown   {-h,-r,-P,-H,now,--halt,--poweroff,--reboot}
dbus-send / busctl calls to org.freedesktop.login1.Manager.{Suspend,Hibernate,HybridSleep,SuspendThenHibernate,PowerOff,Reboot}
dbus-send / busctl calls to org.freedesktop.UPower.{Suspend,Hibernate,HybridSleep}
gsettings set ... sleep-inactive-{ac,battery}-type ANY-VALUE-EXCEPT-'nothing'-OR-'blank'
```

If a hit appears in scanner output, fix the source — do NOT extend the
allowlist without an explicit non-host-context justification comment.

**Verification commands** (run before claiming a fix is complete):

```bash
bash challenges/scripts/no_suspend_calls_challenge.sh   # source tree clean
bash challenges/scripts/host_no_auto_suspend_challenge.sh   # host hardened
```

Both must PASS.

<!-- END host-power-management addendum (CONST-033) -->

