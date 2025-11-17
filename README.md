# ArguSeek

<p align="center">
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/100%25%20AI-Generated-ff69b4.svg" alt="100% AI Generated">
</p>

**Ground your AI agents in current reality**

LLMs operate from training data frozen at a point in time. Claude Sonnet 4.5's knowledge ended January 2025. When your agent needs to:
- Check for CVEs published last month
- Understand breaking changes in library v9 released last week
- Find current best practices that evolved since training

...it's guessing from outdated patterns, not working from facts.

ArguSeek is an open-source MCP server that anchors agents in current ecosystem knowledge through intelligent web research. Self-host your own instance and provide two specialized tools via the Model Context Protocol:

- **Deep Research** (`research_iteratively`): Multi-source parallel research with synthesis and bias detection
- **Targeted Extraction** (`fetch_url`): Context-aware content extraction from specific URLs

Transform your agent from creative fiction writer to grounded assistant working from verifiable current knowledge.

## The Grounding Problem

**Without current knowledge, agents hallucinate plausible solutions.**

Real scenario: Your agent debugs a JWT authentication bug.

**Without ArguSeek (working from training data):**
```javascript
// Agent generates from training data patterns (frozen Jan 2025)
app.use(async (req, res, next) => {
  const decoded = jwt.verify(token, SECRET);  // v8 API pattern
  // ... looks reasonable, but:
  // ❌ jsonwebtoken v9 requires algorithms parameter
  // ❌ CVE-2025-23529 affects <9.0.0 (algorithm confusion)
  // ❌ Your prod runs v8.5.1 (vulnerable)
});
```

**With ArguSeek (grounded in current knowledge):**
```javascript
// Agent researches: "jsonwebtoken security best practices 2025"
// ArguSeek finds: CVE-2025-23529, v9 migration docs, current patterns
app.use(async (req, res, next) => {
  const decoded = jwt.verify(token, SECRET, {
    algorithms: ['HS256']  // v9 required parameter
  });
  // ✓ Uses current v9 API
  // ✓ Aware of CVE-2025-23529 mitigation
  // ✓ Follows 2025 security guidance
});
```

**The difference:** Guesswork from outdated patterns vs. knowledge from current sources.

## How Grounding Works

### research_iteratively: Deep Multi-Source Research

Comprehensive research when your agent needs current knowledge on a topic:

**What it does:**
- Optimizes your query into 2-3 complementary searches using LLM reasoning
- Executes parallel searches across the web
- Fetches content from 12+ high-quality sources
- Detects bias/promotional content with counter-query suggestions
- Synthesizes findings with source citations

**Grounding scenarios:**
```
"Research passport.js JWT token expiration validation—check for CVEs and breaking changes since January 2025"
```
```
"What changed in React Server Components between v14 and v15? Focus on breaking changes."
```
```
"Find production gotchas with Prisma connection pooling in serverless environments—include recent GitHub issues"
```

### fetch_url: Targeted Documentation Extraction

Precise extraction when your agent needs specific information from a known source:

**What it does:**
- Context-aware extraction from web pages
- PDF processing via Gemini Vision API
- Preserves structure for migration guides and API references

**The `looking_for` parameter (optional):**
When provided, instructs extraction to focus only on specific information. Without it, the tool extracts all content generically.

- **Without `looking_for`**: Returns full page content as markdown
- **With `looking_for="authentication methods"`**: Returns ONLY auth-related sections; explicitly states "This page doesn't contain information about authentication methods" if absent

This reduces hallucination—the agent reports missing information instead of guessing.

**Grounding scenarios:**
```
"Extract authentication methods from https://docs.stripe.com/api/authentication"
```
```
"Parse breaking changes from https://github.com/vercel/next.js/releases/tag/v15.0.0"
```
```
"Get migration steps from https://www.prisma.io/docs/guides/upgrade-guides/upgrading-versions"
```

## Quick Start

> **SECURITY NOTE:** ArguSeek has **NO built-in authentication**. Anyone who can reach your server can use it and consume your API quota.
>
> **For local development:** This is fine—use `localhost:8080`
>
> **For production:** DO NOT expose port 8080 to the internet without authentication. Use reverse proxy auth, Cloud Run IAM, or API gateway.

### Self-Hosting with Docker Compose (Recommended)

1. **Clone the repository**
   ```bash
   git clone https://github.com/ArguSeek/arguseek.git
   cd arguseek
   ```

2. **Get your service API keys**
   - Google Custom Search API: https://developers.google.com/custom-search/v1/introduction
   - Google CSE ID: https://programmablesearchengine.google.com/
   - Gemini API: https://ai.google.dev/

3. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env with your API keys
   ```

4. **Start the service**
   ```bash
   docker-compose up -d
   ```

5. **Verify it's running**
   ```bash
   curl http://localhost:8080/health
   ```

Your ArguSeek instance is now running locally.

## Connecting AI Clients

ArguSeek supports two transport modes to connect with your AI clients:

1. **Stdio mode (default)**: Direct subprocess integration via stdin/stdout
2. **HTTP mode**: HTTP server for remote deployments and Docker containers

### Native Stdio Integration (Recommended for Local Use)

The simplest way to connect MCP clients like Claude Code CLI. ArguSeek runs as a subprocess and communicates via stdin/stdout.

**Configuration for Claude Code CLI:**
```bash
claude mcp add arguseek ./server
```

**Configuration for Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "arguseek": {
      "command": "/path/to/arguseek/server",
      "env": {
        "GOOGLE_API_KEY": "your-key",
        "GOOGLE_CSE_ID": "your-cse-id",
        "GEMINI_API_KEY": "your-gemini-key"
      }
    }
  }
}
```

**Benefits of stdio mode:**
- No network configuration required
- No port conflicts
- Automatic process lifecycle management by the client
- Simplest setup for local development

### HTTP Mode (For Remote Deployments)

Use the `-http` flag to run ArguSeek as an HTTP server. This is required for Docker containers and remote deployments.

**Start in HTTP mode:**
```bash
./server -http
```

Modern AI CLIs support direct HTTP connections to MCP servers.

#### Claude Code CLI

**For local development:**
```bash
claude mcp add --transport http arguseek http://localhost:8080/mcp
```

**For remote deployments with authentication:**
```bash
claude mcp add --transport http arguseek https://your-domain.com/mcp
```

#### GitHub Copilot / VS Code

Add to `.vscode/mcp.json` (workspace) or user settings:

```json
{
  "mcpServers": {
    "arguseek": {
      "type": "http",
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

For remote servers with authentication, add `headers`:
```json
{
  "mcpServers": {
    "arguseek": {
      "type": "http",
      "url": "https://your-domain.com/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_TOKEN"
      }
    }
  }
}
```

#### OpenAI Codex CLI

Add to `~/.codex/config.toml`:

```toml
[mcp.servers.arguseek]
url = "http://localhost:8080/mcp"
```

For authenticated remote servers:
```toml
[mcp.servers.arguseek]
url = "https://your-domain.com/mcp"
bearer_token = "YOUR_TOKEN"

# For OAuth (requires rmcp_client feature)
[features]
rmcp_client = true
```

#### OAuth Discovery Endpoint

ArguSeek exposes `/.well-known/oauth-authorization-server` for compatibility with OAuth-aware MCP clients. This endpoint returns minimal metadata to signal that **no authentication is required** for local development.

**For production deployments:**
- Set the `OAUTH_ISSUER` environment variable to your service's public URL
- Implement authentication externally (see [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md))
- Example: `export OAUTH_ISSUER="https://arguseek.example.com"`

**Note:** This endpoint is informational only. ArguSeek does not implement OAuth flows. Secure production deployments using reverse proxy authentication, Cloud Run IAM, or API Gateway.

#### Transport Details

ArguSeek implements **basic HTTP request-response transport** for MCP protocol version 2024-11-05:
- Single endpoint: `POST /mcp` for all JSON-RPC messages
- Synchronous request-response pattern (no Server-Sent Events or streaming)
- Compatible with modern MCP clients that support HTTP transport

## Architecture

ArguSeek implements a layered MCP-based architecture optimized for concurrent execution and bias-aware synthesis:

```
┌─────────────────────────────────────────┐
│      AI CLIs (HTTP or Stdio)            │
│  Claude Code, Copilot, Claude Desktop   │
└───────────────┬─────────────────────────┘
                │ HTTP/JSON-RPC or Stdio
                │
                ▼
┌─────────────────────────────────────────┐
│       ArguSeek MCP Server               │
│       (Open-by-Default)                 │
│       :8080/mcp (HTTP) or stdio         │
│                                         │
│  ┌───────────────────────────────────┐ │
│  │       Research Agent              │ │
│  │  - Query optimization (LLM)       │ │
│  │  - Parallel search execution      │ │
│  │  - Bias detection & analysis      │ │
│  │  - Intelligent synthesis          │ │
│  └───────────────────────────────────┘ │
└──────────────────┬──────────────────────┘
                   │
     ┌─────────────┼─────────────┐
     ▼             ▼             ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│  Google  │  │  Gemini  │  │   Web    │
│  Search  │  │   API    │  │  Fetch   │
└──────────┘  └──────────┘  └──────────┘
```

**Connection Paths:**
- **HTTP mode**: Modern CLIs connect directly to `:8080/mcp` endpoint
- **Stdio mode**: Clients spawn ArguSeek as subprocess, communicate via stdin/stdout

### Key Design Principles

- **LLM-Driven Query Optimization**: Transforms single query into 2-3 complementary searches that explore different angles
- **Parallel Execution**: Concurrent search execution and content fetching reduce latency
- **Bias Detection**: Pattern analysis identifies coordinated messaging, promotional content, or one-sided coverage—provides counter-query suggestions
- **Fallback Mechanisms**: Two-phase URL fetching (primary + backup) guarantees synthesis quality even with partial failures
- **Open-by-Default**: No built-in authentication; security delegated to infrastructure layer (reverse proxy, Cloud Run IAM, API gateway)

### Configuration

**Environment Variables:**

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_API_KEY` | Yes | Google Custom Search API key |
| `GOOGLE_CSE_ID` | Yes | Google Custom Search Engine ID |
| `GEMINI_API_KEY` | No | Gemini API key (defaults to GOOGLE_API_KEY) |
| `PORT` | No | Server port (default: 8080) |

## Deployment Options

- **Docker Compose**: Easiest for most users (recommended)
- **Binary**: Standalone executable
- **From Source**: Build and run with Go
- **Google Cloud Platform**: Optional GCP deployment with Secret Manager

## Development

### Prerequisites

- Go 1.21 or higher
- Docker (for containerized development)
- Google API keys

### Running Locally

```bash
# Install dependencies
go mod download

# Copy environment template
cp .env.example .env
# Edit .env with your keys

# Run the server
go run cmd/server/main.go

# Run tests
go test ./...
```

## License

ArguSeek is released under the [MIT License](LICENSE).

---

**Self-host your grounding infrastructure. Transform agents from guessing to knowing.**
