# WikiMind

> Local-first multi-agent wiki that compounds knowledge over time.

WikiMind is a git-backed knowledge base where AI agents (Claude, Codex, etc.)
can propose changes through a review queue, while humans maintain full control.

## Quick Start

```bash
# Build
go build -o bin/wikimind ./cmd/wikimind

# Initialize a vault
wikimind init ~/my-wiki

# Ingest a document
wikimind ingest ~/papers/interesting-paper.md

# Search
wikimind query "knowledge compounding"

# Run health checks
wikimind lint

# Start MCP server for AI agents
wikimind mcp serve --vault ~/my-wiki

# 5-minute guided demo
wikimind demo
```

## Architecture

- **Vault**: Git repository with structured directories (`raw/`, `wiki/`, `.wikimind/`)
- **MCP Server**: 17 tools for AI agent interaction (read, propose, review)
- **Review Queue**: Agents propose changes → user accepts/rejects → git commit
- **FTS5 Search**: Full-text search with CJK trigram tokenizer
- **Daemon**: Background process for watcher, lock reaper, bundle merger

## CLI Commands

| Command | Description |
|---------|-------------|
| `wikimind init <path>` | Initialize a new vault |
| `wikimind status` | Show vault status |
| `wikimind ingest <file>` | Ingest a raw file (md/pdf/image) |
| `wikimind query <text>` | Full-text search |
| `wikimind revert <seq>` | Revert a change by sequence number |
| `wikimind review list` | List pending reviews |
| `wikimind review accept <id>` | Accept and apply a review |
| `wikimind review reject <id>` | Reject a review |
| `wikimind lint` | Run vault health checks |
| `wikimind doctor` | Check system dependencies |
| `wikimind watch --auto-ingest` | Watch for new files |
| `wikimind demo` | Run guided demo |
| `wikimind mcp serve` | Start MCP server |
| `wikimind log` | Show change-log history |
| `wikimind reindex` | Rebuild page index |

## MCP Tools (17 total)

**Read (9)**: wiki_info, list_index, read_page, read_raw, read_raw_anchor,
read_claim, search, graph_neighbors, get_history

**Write (8)**: agent_handshake, propose_page, propose_edit, propose_claim,
request_review, log_append, acquire_lock, release_lock

## Requirements

- Go 1.26+
- git ≥ 2.30
- Python 3 + pypdf (optional, for PDF ingest)

## License

MIT
