<div align="center">
  <img src="assets/logos/logo-full.svg" alt="ArguSeek" width="350">

  <h3>Wide research, not deep reports</h3>

  <p>Ground your AI agents in current reality.</p>

  <p>
    <a href="https://github.com/ArguSeek/arguseek/actions/workflows/ci.yml"><img src="https://github.com/ArguSeek/arguseek/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
    <img src="https://img.shields.io/badge/100%25%20AI-Generated-ff69b4.svg" alt="100% AI Generated">
  </p>
</div>

---

## Wide Research

**12 sources in 40 seconds. Not one deep dive.**

Daily coding is hundreds of micro-decisions: debug this error, check if that library is maintained, find the right API pattern. Deep research tools give you thorough reports—but you don't need a 10-minute analysis when you're mid-task.

Wide research synthesizes many sources fast. Think: asking 12 developers for quick takes vs. hiring one consultant for a report.

| Task | Deep Research | Wide Research |
|------|---------------|---------------|
| "Best auth library for Next.js 15?" | 10-min report from 3 articles | 40-sec synthesis from 12+ sources |
| "Why is my React hydration failing?" | Detailed explanation you'll skim | Actual solutions from real discussions |
| "Is passport.js still maintained?" | History and full analysis | Current status + alternatives if not |

**When to use each:**
- **Wide** (95% of coding): Debugging, API lookups, library choices, error messages, compatibility checks
- **Deep** (5%): Learning new frameworks, architecture decisions, strategic planning

---

## The Problem

LLMs guess from outdated training data. When your agent needs current info—recent CVEs, breaking changes, deprecation notices—it's working from stale patterns instead of facts.

## The Solution

ArguSeek is an MCP server that gives AI agents real-time web research:

- **12+ sources in ~40 seconds** — not one deep dive
- **Bias detection** — promotional content flagged with counter-queries
- **Context chaining** — build on previous queries without repeating work

## The Tools

### `research_iteratively` — Wide Research

Answers questions by researching multiple sources in parallel.

```
"Research passport.js JWT CVEs since January 2025"
"What changed in React Server Components v14 to v15? Focus on breaking changes"
```

**How it works:** Your query → 2-3 optimized Google searches → 30 URLs deduplicated → 12+ sources fetched → bias detection + synthesis → answer with citations.

### `fetch_url` — Targeted Extraction

Extracts specific information from a known URL (HTML or PDF).

```
fetch_url(url="https://docs.stripe.com/api/authentication", looking_for="authentication methods")
fetch_url(url="https://github.com/vercel/next.js/releases/tag/v15.0.0", looking_for="breaking changes")
```

## Quick Start

```bash
# Install
brew tap ArguSeek/arguseek && brew install arguseek
# Or: git clone ... && make install

# Configure
export GOOGLE_API_KEY="your-key"
export GOOGLE_CSE_ID="your-cse-id"

# Run
arguseek  # stdio mode (for Claude Code, Claude Desktop)
```

## Connect to Claude Code

```bash
claude mcp add arguseek arguseek
```

Done. Both tools are now available to your agent.

## Other Clients

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "arguseek": {
      "command": "arguseek",
      "env": {
        "GOOGLE_API_KEY": "your-key",
        "GOOGLE_CSE_ID": "your-cse-id"
      }
    }
  }
}
```

**HTTP mode** (containers, remote):
```bash
arguseek -http  # Runs on :8080
claude mcp add --transport http arguseek http://localhost:8080/mcp
```

## Architecture

```
Query → Query Optimization (LLM)
      → 2-3 Parallel Google Searches
      → 30 URLs (deduplicated, ranked)
      → Fetch 12+ (two-phase fallback)
      → Bias Detection ‖ Synthesis (parallel)
      → Answer with citations
```

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_API_KEY` | Yes | [Google Custom Search API key](https://developers.google.com/custom-search/v1/introduction) |
| `GOOGLE_CSE_ID` | Yes | [Custom Search Engine ID](https://programmablesearchengine.google.com/) |
| `GEMINI_API_KEY` | No | Defaults to `GOOGLE_API_KEY` |

## Advanced

- **Deployment:** [DEPLOYMENT.md](DEPLOYMENT.md) — Cloud Run, Docker, traffic verification
- **Security:** [PRODUCTION_SECURITY.md](PRODUCTION_SECURITY.md) — Auth strategies for production
- **Development:** [CLAUDE.md](CLAUDE.md) — Full API reference, architecture details

## License

[MIT](LICENSE)
