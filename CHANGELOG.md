# Changelog

All notable changes to ArguSeek will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Native MCP protocol support**: Connect AI agents directly without external bridges or proxies. Works out-of-the-box with Claude Code, Cursor, and other MCP clients via stdio transport.
- **Dual transport modes**: Run as a local subprocess (stdio mode, default) for direct MCP integration, or as an HTTP server (`-http` flag) for container deployments and web-based clients.
- **Version tracking**: Query the server version with `-version` flag or through the MCP initialize handshake. Supports automated release builds with version injection.
- **One-command installation**: Install globally with `make install` or to your user directory with `make install-user` for instant access across your system.
- **Automated release workflow**: Create versioned releases with a single `make release` command—handles version bumping, Docker builds, and binary packaging automatically.
- **Deployment automation**: Deploy to Google Cloud Run with confidence using built-in dry-run previews, traffic verification, and integrated quality assurance testing.
- **Comprehensive security guide**: Production-ready authentication strategies, rate limiting patterns, OAuth integration, and pre-deployment security checklists in `PRODUCTION_SECURITY.md`.
- **AI agent integration guide**: Complete operational guide (`CLAUDE.md`) covering architecture patterns, security constraints, common pitfalls, testing strategies, and deployment workflows.
- **Step-by-step deployment documentation**: Full walkthrough in `DEPLOYMENT.md` with configuration templates, traffic routing verification, rollback procedures, and post-deployment validation.
- **Enhanced QA harness**: Automatically manages server lifecycle in local mode, validates MCP protocol compliance, and tests concurrent load scenarios. Provides actionable reports with citation validation.
- **Visual branding**: Official ArguSeek logo and brand guidelines for integration into documentation and tools.

### Changed

- **MCP protocol compliance**: Server responses now wrap content in proper MCP format, ensuring compatibility with all MCP-compliant clients. Includes OAuth discovery endpoints for future authentication extensions.
- **Improved documentation**: Installation instructions now include system requirements, dependency checks, and troubleshooting guidance. README features quick-start sections and clear usage examples.

### Removed

- **Node.js stdio bridge**: Eliminated external dependency by implementing native Go stdio transport. Reduces installation complexity and improves reliability.

### Fixed

- **Container networking**: Docker deployments now correctly default to HTTP mode, eliminating connection issues in containerized environments.
- **Project structure**: Build artifacts and binaries are properly ignored with root-anchored patterns, preventing accidental commits.

---

## Release History

No releases yet. This changelog captures all development leading to the first release.

When the first release is tagged, this section will document the version history in reverse chronological order.
