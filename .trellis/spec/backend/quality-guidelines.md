# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

(To be filled by the team)

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)

<spec-entry category="quality" keywords="cobra-cli,vault-init,status-contract,go-embed,schema-templates" date="2026-05-22" source="cmd/wikimind/command.go:12;internal/vault/vault.go:46;spec-v2/templates/templates.go:9">

## Scenario: W1 D1 CLI init/status contract

### 1. Scope / Trigger
- Trigger: adding or changing public WikiMind CLI commands, vault bootstrap behavior, or embedded schema template wiring.
- Applies to `cmd/wikimind`, `internal/vault`, `internal/schema`, and the embedded default templates package.

### 2. Signatures
- `wikimind init <vault>` initializes a vault and prints `initialized: <abs-root>` plus `schema_version: <version>`.
- `wikimind status [vault]` accepts zero or one path; with no path, it starts from the current working directory and walks upward to find `.wikimind/config.toml`.
- D1 stubs are `wikimind ingest`, `query`, `review`, `lint`, and `revert`; they must run and print `wikimind <cmd>: D1 未实现`.

### 3. Contracts
- `init` accepts a missing path or an existing empty directory. Existing non-empty directories are rejected.
- `init` creates `raw/{inbox,imported,attachments,manifests}`, `wiki/{claims,entities,concepts,sources,topics,_review,_reports}`, `schema/`, and `.wikimind/{audit,locks}`.
- `init` writes `.wikimind/config.toml` with `vault_root`, `schema_version`, and `created_at`.
- `init` writes `wiki/index.md`, `wiki/log.md`, and all seven default schema templates.
- Default templates must be embedded from the `spec-v2/templates` tree. Because Go embed patterns cannot reach parent directories, the embed package lives next to those template files.
- `status` reports vault path, schema version, raw file count, wiki Markdown page count, claim Markdown count, git branch, git clean/dirty state, and health.

### 4. Validation & Error Matrix
- Empty `init` path -> return `vault path is required`.
- Existing file at target path -> return `vault path exists and is not a directory`.
- Existing non-empty target directory -> return `vault directory already exists and is not empty`.
- Missing vault marker for `status` -> return `no WikiMind vault found`.
- Malformed or missing `schema_version` in config -> return a config parse/read error.
- Git unavailable or non-repository during `status` -> keep status readable and mark git unavailable.

### 5. Good/Base/Bad Cases
- Good: `wikimind init /tmp/vault && wikimind status /tmp/vault` creates a git-backed vault and reports `schema_version: 1.0`.
- Base: `wikimind status` from `vault/wiki/topics` resolves the parent vault root.
- Bad: `wikimind init` into a directory containing any file refuses to modify that directory.

### 6. Tests Required
- Command tests for `init`, `status`, and every D1 stub command.
- Vault tests for directory structure, config file, initial wiki files, git repository presence, and non-empty directory rejection.
- Template tests comparing written schema files with `spec-v2/templates` source content.
- Public helper tests for template filename immutability and unknown template rejection.

### 7. Wrong vs Correct

#### Wrong
```go
//go:embed ../../spec-v2/templates/*.md
```

#### Correct
```go
// In spec-v2/templates/templates.go, next to the source template files.
//go:embed *.md
var files embed.FS
```

</spec-entry>
