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
make build              # → ./bin/server
make install            # Global (sudo) → /usr/local/bin/arguseek
make install-user       # User → ~/bin/arguseek

./bin/server            # Stdio mode (MCP clients)
./bin/server -http      # HTTP mode (containers)
DEBUG=true ./bin/server -http

go run tools/qa-harness/main.go local
go test -v ./...
make lint
```

**CRITICAL:** Stdio mode reserves stdout for MCP. All logs MUST go to stderr.

---

## Critical Patterns & Constraints

### Non-Negotiable Constraints
1. **Environment Variables** - Fatal if `GOOGLE_API_KEY`/`GOOGLE_CSE_ID` missing
2. **Transport Abstraction** - No HTTP headers/status codes in `ProcessRequest()`
3. **Graceful Degradation** - User input always valid, fallback on preprocessing failure
4. **API Key Management** - Load from env vars or secrets manager, NEVER hardcode
5. **JSON-RPC Compliance** - Clients rely on error codes for retry logic
6. **Context Cancellation** - All blocking operations must respect `ctx.Done()`
7. **Clean Lint** - All code must pass `make lint` with no errors or warnings

---

## Common Pitfalls

### Stdio Logging
**Problem:** Logs to stdout break MCP protocol
**Symptom:** `Error: invalid character 'L'...`
**Fix:** Logs to `os.Stderr` (configured in `cmd/server/main.go`)

### Docker Mode
**Problem:** Stdio incompatible with containers
**Fix:** Dockerfile enforces `-http` via `CMD ["-http"]`

### Context Cancellation
**Bad:** `time.Sleep(5 * time.Second)`
**Fix:** `select { case <-time.After(5*time.Second): ... case <-ctx.Done(): return }`

---

## Environment Variables

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `GOOGLE_API_KEY` | Yes | - | Fatal if missing |
| `GOOGLE_CSE_ID` | Yes | - | Fatal if missing |
| `GOOGLE_CLOUD_PROJECT` | No | - | Enables Vertex AI backend (better quotas, IAM auth) |
| `GOOGLE_CLOUD_LOCATION` | No | `us-central1` | Vertex AI region |
| `GEMINI_API_KEY` | No | `GOOGLE_API_KEY` | Only when Vertex AI not configured |
| `PORT` | No | `8080` | HTTP mode only |
| `DEBUG` | No | `false` | Verbose logging |

**QA Harness:** `ARGUSEEK_DEV_URL`, `ARGUSEEK_PROD_URL`

---

## Testing

### QA Harness
```bash
go run tools/qa-harness/main.go local  # Auto-starts server
go run tools/qa-harness/main.go dev    # Uses ARGUSEEK_DEV_URL
go run tools/qa-harness/main.go prod   # Uses ARGUSEEK_PROD_URL
```

### Unit Tests
```bash
go test -v ./...
go test -v -cover ./...
```

### Version Testing
Version resolution uses `sync.OnceValue` (init-time). Test manually:
- ldflags injection: `go build -ldflags "-X arguseek/internal/version.injectedVersion=v9.9.9"`
- VCS metadata: Build in git repo → 7-char hash
- Fallback: Build outside git repo → "development"

---

## Deployment

### Docker
```bash
docker build -t arguseek .
docker run -p 8080:8080 -e GOOGLE_API_KEY="..." -e GOOGLE_CSE_ID="..." arguseek
curl http://localhost:8080/health
```
**Security:** No built-in auth. Use reverse proxy, Cloud Run IAM, or API Gateway. See [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md).

### Cloud Run
**Config files:** `config/dev.yaml`, `config/prod.yaml`
**Workflow:** Always dry-run first → `make deploy-dev-dry && make deploy-dev`
**Traffic delay:** 30-60s propagation; orchestrator uses exponential backoff

See [DEPLOYMENT.md](DEPLOYMENT.md) for full workflow and rollback procedures.

---

## Additional Resources

- [README.md](README.md) - User docs, installation, client setup
- [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment workflow, traffic verification
- [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md) - Production security strategies
