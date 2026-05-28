# Suggested Commands

- Build all Go packages: `go build ./...`
- Vet all Go packages: `go vet ./...`
- Run Go tests: `go test ./...`
- Run Python worker lint: `ruff check worker/`
- Run WikiMind CLI locally after implementation: `go run ./cmd/wikimind --help`
- Run daemon stub locally: `go run ./cmd/wikimindd`
- Check git state: `git status --porcelain`
- Trellis context: `python3 ./.trellis/scripts/get_context.py`
- Trellis phase detail: `python3 ./.trellis/scripts/get_context.py --mode phase --step <step> --platform codex`