<div align="center">
  <img src="assets/logos/logo-full.svg" alt="ArguSeek" width="350">

  <p>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <img src="https://img.shields.io/badge/100%25%20AI-Generated-ff69b4.svg" alt="100% AI Generated">
  </p>
</div>

**Ground your AI agents in current reality**

LLMs work from training data frozen at a point in time. When your agent needs to check recent CVEs, understand breaking changes in newly released libraries, or find evolved best practices, it's guessing from outdated patterns instead of working from current facts.

ArguSeek is an open-source MCP server that grounds coding agents in current web knowledge through intelligent research. Run locally as a native subprocess or self-host as an HTTP server, providing two specialized tools:

- **`research_iteratively`**: Multi-source parallel research with synthesis and bias detection
- **`fetch_url`**: Context-aware content extraction from specific URLs

## The Tools

### research_iteratively: Deep Multi-Source Research

Comprehensive research when your agent needs current knowledge on a topic.

**What it does:**
Answers questions by researching multiple sources, synthesizing findings, and detecting bias in results.

**How it works:**
- Optimizes your query into 2-3 complementary searches using LLM reasoning
- Executes searches in parallel across Google Custom Search
- Fetches content from 12+ sources with two-phase fallback (15 primary + 15 backup URLs)
- Runs bias detection and synthesis concurrently for faster results
- Returns synthesized findings with source citations and counter-query suggestions

**Example queries:**
```
"Research passport.js JWT token expiration—check for CVEs since January 2025"
"What changed in React Server Components v14 to v15? Focus on breaking changes"
```

### fetch_url: Targeted Documentation Extraction

Precise extraction when your agent needs specific information from a known URL.

**What it does:**
Extracts relevant content from web pages and PDFs, optionally filtering for specific information.

**How it works:**
- Fetches content with timeout cascade (10s request, 5s TLS/headers)
- Routes to appropriate processor (HTML via goquery, PDF via Gemini Vision)
- Extracts main content, preserving structure and code blocks
- Optional `looking_for` parameter focuses extraction on specific topics
- LLM refinement explicitly reports missing information instead of hallucinating

**Example queries:**
```
fetch_url(url="https://docs.stripe.com/api/authentication", looking_for="authentication methods")
fetch_url(url="https://github.com/vercel/next.js/releases/tag/v15.0.0", looking_for="breaking changes")
```

## Installation

### Prerequisites
- Go 1.23 or higher
- Google API key and Custom Search Engine ID (required)
- Gemini API key (optional, defaults to Google API key if not set)

> See [Configuration](#configuration) section below for detailed API key setup links.

### Quick Start

```bash
# 1. Clone and build
git clone https://github.com/yourusername/arguseek.git
cd arguseek
make install

# 2. Set required environment variables
export GOOGLE_API_KEY="your-google-api-key"
export GOOGLE_CSE_ID="your-custom-search-engine-id"
# Optional: export GEMINI_API_KEY="your-gemini-key"  # Defaults to GOOGLE_API_KEY if not set

# 3. Run the server
arguseek        # Stdio mode (for local MCP clients)
# OR
arguseek -http  # HTTP mode (for remote clients, runs on port 8080)
```

> **Note:** For local build without installation, user-local installation, or platform-specific builds, see [Advanced Installation Options](#advanced-installation-options) below.

## Connecting AI Clients

ArguSeek supports two transport modes:

- **Stdio mode (default)**: Direct subprocess integration for local development
- **HTTP mode (`-http` flag)**: HTTP server for remote deployments

### Stdio Mode (Local Installation)

**Claude Code CLI:**
```bash
# Default (global install)
claude mcp add arguseek arguseek

# Alternative (local build without install)
claude mcp add arguseek ./bin/server
```

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "arguseek": {
      "command": "arguseek",  // default: global install, or use "/path/to/arguseek/bin/server" for local build
      "env": {
        "GOOGLE_API_KEY": "your-key",
        "GOOGLE_CSE_ID": "your-cse-id",
        "GEMINI_API_KEY": "your-gemini-key"  // optional: defaults to GOOGLE_API_KEY if omitted
      }
    }
  }
}
```

### HTTP Mode (Remote Deployments)

Start server: `arguseek -http` (default) or `./bin/server -http` (local build)

**Claude Code CLI:**
```bash
claude mcp add --transport http arguseek http://localhost:8080/mcp
```

**VS Code / GitHub Copilot** (`.vscode/mcp.json`):
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

## Architecture

ArguSeek implements dual-transport MCP architecture with transport-agnostic core logic.

```
┌─────────────────────────────────────────┐
│      AI CLIs (HTTP or Stdio)            │
│  Claude Code, Copilot, Claude Desktop   │
└───────────────┬─────────────────────────┘
                │ HTTP/JSON-RPC or Stdio
                ▼
┌─────────────────────────────────────────┐
│       ArguSeek MCP Server               │
│       :8080/mcp (HTTP) or stdio         │
│                                         │
│  ┌───────────────────────────────────┐  │
│  │       Research Agent              │  │
│  │  - Query optimization (LLM)       │  │
│  │  - Parallel search execution      │  │
│  │  - Bias detection & synthesis     │  │
│  └───────────────────────────────────┘  │
└──────────────────┬──────────────────────┘
                   │
     ┌─────────────┼─────────────┐
     ▼             ▼             ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│  Google  │  │  Gemini  │  │   Web    │
│  Search  │  │   API    │  │  Fetch   │
└──────────┘  └──────────┘  └──────────┘
```

**Key technical details:**
- **Query optimization**: Single query → 2-3 complementary searches via LLM
- **Parallel execution**: Concurrent searches and content fetching (~10s vs 19s serial)
- **Fallback mechanisms**: Two-phase URL fetching guarantees 12+ sources (15 primary + 15 backup)
- **Bias detection**: Pattern analysis identifies promotional content with counter-query suggestions

## Advanced Installation Options

### Local Build (Without Installation)

Build the server binary without installing it globally:

```bash
make build
# Creates ./bin/server in the project directory
# Run with: ./bin/server or ./bin/server -http
```

### User-local Installation

Install to your home directory without requiring sudo:

```bash
make install-user
# Installs to ~/bin/arguseek
# Ensure ~/bin is in your PATH: export PATH="$HOME/bin:$PATH"
# Run from anywhere: arguseek or arguseek -http
```

### Platform-Specific Builds

**Linux:**
```bash
make build
./bin/server        # Stdio mode (default)
./bin/server -http  # HTTP mode
```

**Windows:**
```bash
go build -o server.exe ./cmd/server
server.exe          # Stdio mode
server.exe -http    # HTTP mode
```

**Cross-compilation:**
```bash
# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o server-darwin ./cmd/server

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o server-arm64 ./cmd/server

# Linux
GOOS=linux GOARCH=amd64 go build -o server-linux ./cmd/server

# Windows
GOOS=windows GOARCH=amd64 go build -o server.exe ./cmd/server
```

### Development Setup

```bash
go mod download
cp .env.example .env
# Edit .env with your API keys:
#   GOOGLE_API_KEY (required)
#   GOOGLE_CSE_ID (required)
#   GEMINI_API_KEY (optional, defaults to GOOGLE_API_KEY)
go run cmd/server/main.go        # Stdio mode
go run cmd/server/main.go -http  # HTTP mode
go test ./...                    # Run tests
```

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_API_KEY` | Yes | [Google Custom Search API key](https://developers.google.com/custom-search/v1/introduction) |
| `GOOGLE_CSE_ID` | Yes | [Google Custom Search Engine ID](https://programmablesearchengine.google.com/) |
| `GEMINI_API_KEY` | No | [Gemini API key](https://ai.google.dev/) (defaults to GOOGLE_API_KEY) |
| `PORT` | No | Server port (default: 8080) |

**API Key Setup:**
- Get your `GOOGLE_API_KEY`: [Google Custom Search API](https://developers.google.com/custom-search/v1/introduction)
- Create your `GOOGLE_CSE_ID`: [Programmable Search Engine](https://programmablesearchengine.google.com/)
- Get your `GEMINI_API_KEY` (optional): [Gemini API](https://ai.google.dev/)

> **SECURITY:** ArguSeek has no built-in authentication. For local development, use `localhost:8080`. For production, secure with reverse proxy auth, Cloud Run IAM, or API gateway. See [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md).

## License

ArguSeek is released under the [MIT License](LICENSE).

## Branding

Logo assets are available in [`assets/logos/`](assets/logos/):
- `logo-full.svg` - Primary wordmark with icon (350×64)
- `logo-symbol.svg` - Icon only for favicons and small spaces (100×100)
- `wordmark.svg` - Text only for horizontal layouts (240×50)
- `favicon-*.svg` - Optimized favicons in multiple sizes

**Color Palette:**
- Primary: `#10B981` (Emerald Green) - knowledge, growth, open source
- Accent: `#14B8A6` (Teal) - innovation, discovery, analysis

For detailed usage guidelines and design specs, see [`assets/logos/showcase.jsx`](assets/logos/showcase.jsx).

---

**Run locally or self-host your grounding infrastructure. Transform agents from guessing to knowing.**
