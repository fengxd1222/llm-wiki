# Code Structure

- `cmd/wikimind/main.go`: WikiMind CLI entrypoint. W1 D1 task replaces W0 stub with Cobra CLI.
- `cmd/wikimindd/main.go`: daemon entrypoint, currently W0 stub.
- `internal/*`: Go internal packages, currently mostly `doc.go` placeholders. Relevant W1 D1 packages: `internal/vault` for vault creation/status, `internal/schema` for embedded schema template writing.
- `spec-v2/templates/`: source templates that should be embedded/written to vault `schema/`.
- `migrations/`: SQL migration files.
- `verify/`: small verification programs for MCP, SQLite FTS5, IPC.
- `worker/`: Python worker checked with `ruff` in CI.