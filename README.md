# ArguSeek

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

"What changed in React Server Components between v14 and v15? Focus on breaking changes."

"Find production gotchas with Prisma connection pooling in serverless environments—include recent GitHub issues"
```

### fetch_url: Targeted Documentation Extraction

Precise extraction when your agent needs specific information from a known source:

**What it does:**
- Context-aware extraction from web pages
- PDF processing via Gemini Vision API
- Preserves structure for migration guides and API references

**Grounding scenarios:**
```
"Extract authentication methods from https://docs.stripe.com/api/authentication"

"Parse breaking changes from https://github.com/vercel/next.js/releases/tag/v15.0.0"

"Get migration steps from https://www.prisma.io/docs/guides/upgrade-guides/upgrading-versions"
```

## Why Grounding Matters for Production

Without grounding tools, agents generate solutions from:
- ❌ Training data frozen 6-12 months ago
- ❌ Generic patterns that may not match your architecture
- ❌ Outdated API signatures from deprecated library versions
- ❌ No awareness of recent CVEs or security advisories

**With ArguSeek**, agents work from:
- ✓ Current security advisories and CVE databases
- ✓ Latest library documentation and migration guides
- ✓ Production patterns discovered by the community
- ✓ Recent GitHub issues and real-world solutions

Grounding transforms "sounds reasonable" into "verifiably correct."

## Quick Start

> **SECURITY NOTE:** ArguSeek has **NO built-in authentication**. Anyone who can reach your server can use it and consume your API quota.
>
> **For local development:** This is fine—use `localhost:8080`
>
> **For production:** DO NOT expose port 8080 to the internet without authentication. Use reverse proxy auth, Cloud Run IAM, or API gateway. See [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md).

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

**For detailed installation and deployment options**, see [SELF_HOSTING.md](SELF_HOSTING.md)

## Integration with AI Agents

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "arguseek": {
      "command": "npx",
      "args": ["arguseek-client"],
      "env": {
        "ARGUSEEK_URL": "http://localhost:8080/mcp"
      }
    }
  }
}
```

### The Grounding Stack

Production AI coding requires grounding in **two dimensions**:

| Tool | Grounds Agent In | Use When |
|------|------------------|----------|
| **ChunkHound** | Your actual codebase | "How does auth work in *this* project?" |
| **ArguSeek** | Current ecosystem knowledge | "What are current JWT best practices in 2025?" |

**Agentic RAG in practice:**
The agent decides when to ground itself dynamically:

```
1. Task: "Fix the auth bug"

2. Agent reasons: "I don't know this codebase's implementation"
   → Calls: code_research("How does JWT authentication work in this codebase?")
   → ChunkHound returns: "Uses Passport.js at src/auth.ts:45-67, jsonwebtoken@8.5.1"

3. Agent reasons: "Need current security best practices for JWT"
   → Calls: research_iteratively("passport.js JWT security best practices 2025")
   → ArguSeek returns: "CVE-2025-23529 affects <9.0.0, upgrade required, v9 API changes..."

4. Agent synthesizes: Grounded solution from YOUR architecture + CURRENT knowledge
```

No pre-configured workflows. The agent orchestrates retrieval based on what it learns.

## Effective Usage Examples

### Implementation Tasks
```
"Before implementing JWT refresh tokens, research current best practices and security considerations for 2025"

"Find production-ready examples of WebSocket authentication with JWT—include recent patterns"
```

### Debugging with Current Context
```
"Research 'CUDA out of memory' errors in PyTorch 2.x training. Focus on solutions from 2024-2025."

"Search for GitHub issues about NextJS middleware breaking after v14→v15 upgrade"
```

### Architecture Decisions
```
"Compare Prisma vs Drizzle ORM for TypeScript projects—focus on 2025 performance benchmarks and production experiences"

"Research serverless vs containerized deployment trade-offs for real-time applications in 2025"
```

### Learning with Current Documentation
```
"Research FastAPI beyond official docs—find advanced patterns and common mistakes from community experience"

"Use fetch_url to extract WebSocket connection lifecycle from https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-websocket-api.html"
```

### Combining Both Tools
```
"First research the current state management patterns for React in 2025, then fetch_url from the Redux Toolkit docs to compare the latest API"
```

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

See [SELF_HOSTING.md](SELF_HOSTING.md) for detailed configuration options including GCP Secret Manager integration.

## Deployment Options

- **Docker Compose**: Easiest for most users (recommended)
- **Binary**: Standalone executable
- **From Source**: Build and run with Go
- **Google Cloud Platform**: Optional GCP deployment with Secret Manager

See [SELF_HOSTING.md](SELF_HOSTING.md) for detailed deployment guides.

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

For detailed development setup, see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)

## Contributing

We welcome contributions! Please see:

- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) - Community standards
- [SECURITY.md](SECURITY.md) - Security policy

## Documentation

- [Self-Hosting Guide](SELF_HOSTING.md) - Complete deployment instructions
- [Development Guide](docs/DEVELOPMENT.md) - Local development setup
- [Production Security](PRODUCTION_SECURITY.md) - Security configuration options

## License

ArguSeek is released under the [MIT License](LICENSE).

## Support

- **Issues**: https://github.com/ArguSeek/arguseek/issues
- **Discussions**: https://github.com/ArguSeek/arguseek/discussions
- **Security**: See [SECURITY.md](SECURITY.md)

## Acknowledgments

Built with:
- [Model Context Protocol](https://modelcontextprotocol.io/) by Anthropic
- Google Custom Search API
- Google Gemini API
- Go and various open source libraries

---

**Self-host your grounding infrastructure. Transform agents from guessing to knowing.**
