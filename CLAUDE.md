# ArguSeek - AI Agent Guide

Operational guidance for AI agents. For user docs, see [README.md](README.md).

## Tech Stack

**Runtime:** Go 1.23.0+, static binary (`CGO_ENABLED=0`), dual transport (stdio/HTTP)
**Dependencies:** goquery (HTML), html-to-markdown, pdfcpu, yaml.v3, native net/http
**External:** Google Custom Search API, Gemini API (optimization, synthesis, bias detection)
**Protocol:** MCP (Model Context Protocol) via JSON-RPC 2.0

---

## Quick Commands

```bash
# Install via Homebrew (Recommended for macOS/Linux)
brew tap ArguSeek/arguseek
brew install arguseek
arguseek -version

# Build & Install from Source
make build              # → ./bin/server
make install            # Global (sudo) → /usr/local/bin/arguseek
make install-user       # User → ~/bin/arguseek

# Run
./bin/server            # Stdio mode (MCP clients)
./bin/server -http      # HTTP mode (containers)
DEBUG=true ./bin/server -http

# Test
go run tools/qa-harness/main.go local
go test -v ./...

# Deploy (ALWAYS dry-run first)
make deploy-dev-dry && make deploy-dev
make deploy-prod-dry && make deploy-prod

# Release
git tag -a v0.2.0 -m "Description" && git push origin v0.2.0
make release
```

**CRITICAL:** Stdio mode reserves stdout for MCP. All logs MUST go to stderr.

---

## Architecture

### Layers
```
cmd/server/main.go → internal/mcp/handler.go → internal/agent/agent.go → Preprocessor/WebFetcher/BiasAnalyzer
```

### Core Patterns
1. **Transport Abstraction** - `ProcessRequest()` is transport-agnostic, wrapped by HTTP/stdio handlers
2. **Concurrent Pipeline** - Query optimization, search, fetching, bias analysis run in parallel (10s vs 19s)
3. **Two-Phase Fallback** - Fetch 15 primary URLs, then 15 backup if <12 succeed
4. **Graceful Degradation** - Preprocessing/search failures fallback to original query, partial success > total failure
5. **Interface-Based** - `SearchClient`, `LLMClient`, `ContentFetcher` enable mocking (`internal/agent/interfaces.go`)

---

## Critical Patterns & Constraints

### Security (NEVER Remove)
```go
// internal/mcp/handler.go:340-368 - Input validation gates
if err := validateQueryLength(query); err != nil { ... }  // ReDoS prevention
if err := validateURL(url); err != nil { ... }             // SSRF prevention (private IPs, localhost)
```

### Error Handling
**Fail-fast validation:** Security boundary at handler entry
**Graceful degradation:** `preprocessResult = &PreprocessorResult{Queries: []string{query}}` on failure
**Error wrapping:** `fmt.Errorf("failed: %w", err)`
**JSON-RPC codes (NEVER CHANGE):** -32700 (parse), -32600 (invalid request), -32601 (method not found), -32602 (invalid params), -32603 (internal error)
**Goroutine safety:** Panic recovery with mutex-protected results

### Concurrency
**Context cancellation (ALWAYS):**
```go
select {
case result := <-workChan: return result
case <-ctx.Done(): return ctx.Err()
}
```
**Channels:** Buffered size 1 prevents blocking
**All goroutines/sleeps:** Must check `ctx.Done()` to prevent leaks

### Logging
**Stdio mode:** `os.Stderr` only (stdout reserved for MCP)
**HTTP mode:** stdout
**Request correlation:** `requestID := request.GetRequestID(ctx)`
**Debug mode:** `if os.Getenv("DEBUG") == "true" { log.Printf("[DEBUG] ...") }`
**NEVER log full API keys**

### Naming Conventions
- **Constants:** `MaxResultsPerQuery`, `TargetSourceCount`
- **Functions:** `ResearchIteratively()` (public), `validateURL()` (private)
- **Interfaces:** `SearchClient`, `ContentFetcher` (noun + -er)
- **Structs:** `SearchAgent` (private fields lowercase first)

### Non-Negotiable Constraints
1. **Environment Variables** - Fatal if `GOOGLE_API_KEY`/`GOOGLE_CSE_ID` missing
2. **Transport Abstraction** - No HTTP headers/status codes in `ProcessRequest()`, transport code only in wrappers
3. **Graceful Degradation** - User input always valid, fallback to original query on preprocessing failure
4. **API Key Management** - Load from env vars or secrets manager, NEVER hardcode
5. **JSON-RPC Compliance** - Clients rely on error codes for retry logic
6. **Context Cancellation** - All blocking operations must respect `ctx.Done()`

---

## Common Pitfalls

### 1. Stdio Logging
**Problem:** Logs to stdout break MCP protocol
**Symptom:** `Error: invalid character 'L'...`
**Fix:** Logs to `os.Stderr` (configured in `cmd/server/main.go`)

### 2. Docker Mode
**Problem:** Stdio incompatible with containers
**Fix:** Dockerfile enforces `-http` via `CMD ["-http"]`

### 3. Context Cancellation
**Bad:** `time.Sleep(5 * time.Second)`
**Fix:** `select { case <-time.After(5*time.Second): ... case <-ctx.Done(): return }`

### 4. Timeout Tuning
**Current:** 10s request, 5s dial/TLS
**Symptom:** Frequent "context deadline exceeded"
**Fix:** Increase in `internal/agent/fetcher.go` (trade-off: slower failure vs compatibility)

### 5. API Rate Limits
**Google CSE Free:** 100 queries/day, `research_iteratively` = 2-3 queries/request → 30-50 requests exhaust quota
**Symptom:** `MCP error -32603: quota exceeded`
**Fix:** Monitor GCP Console, upgrade to paid tier (10k/day), implement caching

### 6. Deployment Traffic Routing
**Problem:** Cloud Run deployment succeeds but requests hit old revision
**Cause:** Traffic routing takes 30-60s
**Fix:** Orchestrator polls with exponential backoff (1s→30s, max 10 attempts)

### 7. Environment Variable Fallback
**Issue:** `export GEMINI_API_KEY=""` (explicit empty) prevents fallback to `GOOGLE_API_KEY`
**Fix:** `unset GEMINI_API_KEY` or don't export. Debug with `DEBUG=true ./bin/server`

---

## Environment Variables

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `GOOGLE_API_KEY` | Yes | - | Fatal if missing |
| `GOOGLE_CSE_ID` | Yes | - | Fatal if missing |
| `GEMINI_API_KEY` | No | `GOOGLE_API_KEY` | Falls back if not set |
| `PORT` | No | `8080` | HTTP mode only |
| `DEBUG` | No | `false` | JSON structured logging |
| `OAUTH_ISSUER` | No | `http://localhost:8080` | OAuth discovery |

**QA Harness:** `ARGUSEEK_DEV_URL`, `ARGUSEEK_PROD_URL`

---

## Testing

### QA Harness
```bash
go run tools/qa-harness/main.go local  # Auto-starts server
go run tools/qa-harness/main.go dev    # Uses ARGUSEEK_DEV_URL
go run tools/qa-harness/main.go prod   # Uses ARGUSEEK_PROD_URL
```

**Phases:** Env validation → Sequential (20 tests, fail-fast) → Concurrent (10 parallel) → Report
**Test Types:** Functional, tool tests, integration (PDF), negative (error codes), load
**Critical (Fail-Fast):** API keys, MCP protocol, input validation

### Unit Tests
```bash
go test -v ./...
go test -v -cover ./...
```

**Mocking:** `NewSearchAgentWithDependencies(&MockSearchClient{...}, ...)` with interface implementations

### Version Injection Testing

The `internal/version` package uses `sync.OnceValue` which evaluates at init time, making it difficult to test all resolution paths in unit tests. Verify version injection manually:

```bash
# Test 1: ldflags injection (release builds)
go build -ldflags "-X arguseek/internal/version.injectedVersion=v9.9.9" -o /tmp/test-version ./cmd/server
/tmp/test-version -version
# Expected: v9.9.9

# Test 2: VCS metadata (normal builds in git repo)
go build -o /tmp/test-version ./cmd/server
/tmp/test-version -version
# Expected: 7-char git hash (e.g., a1b2c3d) or "a1b2c3d-dirty" if uncommitted changes

# Test 3: Development fallback (build outside git repo)
cd /tmp && go build -o test-version arguseek/cmd/server
./test-version -version
# Expected: development

# Test 4: Release build verification
make release
strings dist/arguseek-*-linux-amd64 | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+"
# Expected: Tag version string found in binary
```

**Why manual testing:** The version resolution happens at package init (via `sync.OnceValue`). Unit tests can only verify one code path per test run. Integration testing or manual builds are required to verify all three resolution tiers (ldflags → VCS → fallback).

---

## Deployment

### Docker
```bash
docker build -t arguseek .
docker run -p 8080:8080 -e GOOGLE_API_KEY="..." -e GOOGLE_CSE_ID="..." arguseek
curl http://localhost:8080/health  # {"status":"ok"}
```

**Security:** No built-in auth. Production options: reverse proxy (nginx, Caddy), Cloud Run IAM, API Gateway, private VPC. See [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md).

### Cloud Run
```yaml
# config/dev.yaml
project_id: "your-gcp-project"
region: "us-central1"
service_name: "arguseek-dev"
runtime:
  memory: "512Mi"
  cpu: 1
  min_instances: 0
  max_instances: 3
secrets:
  - name: "GOOGLE_API_KEY"
    secret: "arguseek-google-api-key"
```

```bash
make deploy-dev-dry  # Preview
make deploy-dev      # Execute
gcloud run services describe arguseek-dev --region=us-central1 --format='value(status.url)'
```

See [DEPLOYMENT.md](DEPLOYMENT.md) for full workflow, traffic verification, rollback procedures.

---

## Key File References

- `cmd/server/main.go` - Entry, env vars, mode selection
- `internal/mcp/handler.go` - Transport-agnostic logic, JSON-RPC, validation (lines 340-368: security gates)
- `internal/agent/agent.go` - Research orchestration, ResearchIteratively, FetchURL
- `internal/agent/fetcher.go` - Two-phase fallback, retry logic, panic recovery
- `internal/agent/interfaces.go` - Dependency contracts
- `tools/qa-harness/main.go` - Test suite
- `Makefile` - Build/deploy automation
- `Dockerfile` - Multi-stage build, HTTP mode enforcement
- `config/*.yaml` - Deployment configs

## Additional Resources

- [README.md](README.md) - User docs, installation, client setup
- [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment workflow, traffic verification
- [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md) - Production security strategies
- Makefile - Complete automation reference
