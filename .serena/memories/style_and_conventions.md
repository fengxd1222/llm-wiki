# Style and Conventions

- Go module: `github.com/fengxd1222/llm-wiki`, Go `1.26.x`.
- Existing Go files use short package comments in Chinese and simple standard-library-first code.
- Keep code boring and project-local: prefer `internal/<domain>` packages for business logic and thin command entrypoints under `cmd/`.
- Errors should be propagated with clear context using `%w`; user-facing CLI errors should be concise and not expose sensitive details.
- Tests should cover public behavior and edge cases. For W1 D1, test init/status core paths, non-empty directory rejection, and template copy behavior.
- Follow Trellis: read `.trellis/spec/backend/` and shared guides before coding; load `trellis-check` before reporting completion.