# ArguSeek

**Open-source MCP server for intelligent web research**

ArguSeek transforms your AI coding agent from a simple assistant into a research powerhouse. Self-host your own instance and provide two specialized tools through the Model Context Protocol (MCP):

- **Deep Research** (`research_iteratively`): Performs multi-query, parallel web research with intelligent synthesis
- **Targeted Content Extraction** (`fetch_url`): Fetches and analyzes specific web pages with context-aware extraction

Instead of basic web searches, ArguSeek finds exactly what you need through comprehensive, iterative research.

## Why ArguSeek?

**Finds What Others Can't**
- Uncovers solutions in obscure GitHub issues, forgotten blog posts, and buried documentation
- Automatically searches multiple variations of your query to catch edge cases
- Connects related information across sources for comprehensive answers

**Saves Hours of Research**
- One query spawns multiple parallel searches across the entire web
- Intelligent filtering focuses on truly relevant, high-quality sources
- Progressive deep dives from broad topics to specific solutions

**Perfect for Real Coding Challenges**
- Debug cryptic errors by finding others who solved them
- Discover undocumented API features and workarounds
- Compare tools, services, and pricing across vendors
- Learn best practices that actually work in production
- Extract specific information from documentation and API guides

## Quick Start

> **⚠️ SECURITY NOTE:** ArguSeek has **NO built-in authentication**. Anyone who can reach your server can use it and consume your Google/Gemini API quota.
>
> **For local development:** This is fine - use `localhost:8080`
>
> **For production:** DO NOT expose port 8080 to the internet without adding authentication. Use reverse proxy auth, Cloud Run IAM, or API gateway. See [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md) for options.

### Self-Hosting with Docker Compose (Recommended)

1. **Clone the repository**
   ```bash
   git clone https://github.com/ArguSeek/arguseek.git
   cd arguseek
   ```

2. **Get your service API keys**
   - Google Custom Search API: https://developers.google.com/custom-search/v1/introduction
   - Google CSE ID: https://programmablesearchengine.google.com/
   - Gemini API (optional): https://ai.google.dev/

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

That's it! Your ArguSeek instance is now running locally.

**For detailed installation instructions**, see [SELF_HOSTING.md](SELF_HOSTING.md)

## Using ArguSeek with Claude Desktop

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

## Available Tools

### research_iteratively

Performs comprehensive web research using Google Search and Gemini AI:

```json
{
  "name": "research_iteratively",
  "arguments": {
    "query": "React Server Components best practices and common pitfalls",
    "previous_query": "optional context from previous search"
  }
}
```

Features:
- Multiple parallel search queries
- Automatic query optimization
- Intelligent synthesis with citations
- Contextual follow-up support

### fetch_url

Fetches and extracts content from specific web pages:

```json
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://docs.example.com/api-guide",
    "looking_for": "authentication methods and API endpoints"
  }
}
```

Features:
- Intelligent content extraction
- Context-aware summarization
- Documentation parsing
- API guide analysis

## Effective Usage Examples

**For Implementation Tasks**
```
"Research React Server Components best practices before implementing"
"Find production-ready examples of WebSocket authentication"
```

**For Debugging**
```
"Research 'CUDA out of memory' errors in PyTorch training. Find solutions and workarounds."
"Search for GitHub issues related to NextJS middleware breaking after upgrade"
```

**For Architecture Decisions**
```
"Compare Prisma vs Drizzle ORM for TypeScript projects in 2024"
"Research pros and cons of serverless vs containerized deployments"
```

**For Learning New Tools**
```
"Research FastAPI beyond official docs - find tutorials and common mistakes"
"Use fetch_url to extract authentication steps from https://docs.stripe.com"
```

**Combining Both Tools**
```
"First research the best state management libraries for React, then fetch_url from their docs to compare APIs"
```

With ArguSeek, your AI agent doesn't just search—it investigates, analyzes, and delivers actionable intelligence.

## Deployment Options

- **Docker Compose**: Easiest for most users (recommended)
- **Binary**: Standalone executable
- **From Source**: Build and run with Go
- **Google Cloud Platform**: Optional GCP deployment with Secret Manager

See [SELF_HOSTING.md](SELF_HOSTING.md) for detailed deployment guides.

## Architecture

```
┌─────────────────────────────────────────┐
│         Your AI Agent (Claude)          │
│          via MCP Protocol               │
└───────────────┬─────────────────────────┘
                │
                │ HTTP/JSON-RPC
                ▼
┌─────────────────────────────────────────┐
│          ArguSeek MCP Server            │
│         (Open-by-Default)               │
│                                         │
│  ┌─────────────────────────────────┐   │
│  │      Research Agent             │   │
│  │   - Query optimization          │   │
│  │   - Parallel execution          │   │
│  │   - Content synthesis           │   │
│  └─────────────────────────────────┘   │
└───────────┬─────────────────────────────┘
            │
            ├──────────────┬──────────────┐
            ▼              ▼              ▼
     ┌──────────┐   ┌──────────┐   ┌──────────┐
     │  Google  │   │  Gemini  │   │   Web    │
     │  Search  │   │   API    │   │  Fetch   │
     └──────────┘   └──────────┘   └──────────┘
```

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_API_KEY` | Yes | Google Custom Search API key |
| `GOOGLE_CSE_ID` | Yes | Google Custom Search Engine ID |
| `GEMINI_API_KEY` | No | Gemini API key (defaults to GOOGLE_API_KEY) |
| `PORT` | No | Server port (default: 8080) |

### Secret Management

- **Environment Variables** (default): Best for self-hosting
- **GCP Secret Manager** (optional): For cloud deployments

See [SELF_HOSTING.md](SELF_HOSTING.md) for configuration details.

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

**Self-host your own intelligent research assistant today!**
