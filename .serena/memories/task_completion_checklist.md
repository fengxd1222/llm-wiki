# Task Completion Checklist

- Run `gofmt` on changed Go files.
- Run `go build ./...`.
- Run `go vet ./...`.
- Run `go test ./...`.
- If Python worker changes, run `ruff check worker/`.
- Load and follow `trellis-check` for final verification.
- Load `trellis-update-spec` to decide whether new project knowledge should be recorded.
- Prepare a specific-file git commit plan; do not include unrelated dirty files.