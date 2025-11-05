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

## Architecture

ArguSeek implements a layered MCP-based architecture optimized for concurrent execution and bias-aware synthesis:

```
┌─────────────────────────────────────────┐
│         AI Agent (Claude)                │
│         via MCP Protocol                 │
└───────────────┬─────────────────────────┘
                │ HTTP/JSON-RPC
                ▼
┌─────────────────────────────────────────┐
│       ArguSeek MCP Server                │
│      (Open-by-Default)                   │
│                                          │
│  ┌─────────────────────────────────┐    │
│  │      Research Agent              │    │
│  │   - Query optimization (LLM)     │    │
│  │   - Parallel search execution    │    │
│  │   - Bias detection               │    │
│  │   - Content synthesis            │    │
│  └─────────────────────────────────┘    │
└───────────┬─────────────────────────────┘
            │
            ├──────────────┬──────────────┐
            ▼              ▼              ▼
     ┌──────────┐   ┌──────────┐   ┌──────────┐
     │  Google  │   │  Gemini  │   │   Web    │
     │  Search  │   │   API    │   │  Fetch   │
     └──────────┘   └──────────┘   └──────────┘
```

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
