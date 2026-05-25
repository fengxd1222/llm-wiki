# Installation Guide

## From Source (recommended)

```bash
git clone https://github.com/fengxd1222/llm-wiki.git
cd llm-wiki
go build -o bin/wikimind ./cmd/wikimind
go build -o bin/wikimindd ./cmd/wikimindd

# Add to PATH
export PATH="$PWD/bin:$PATH"
```

## macOS (Homebrew)

```bash
# Coming in v0.1.0 release
brew tap fengxd1222/wikimind
brew install wikimind
```

## Dependencies

### Required
- **Go 1.26+**: Build the binaries
- **git ≥ 2.30**: Version control backend

### Optional
- **Python 3**: PDF ingest support
- **pypdf**: `pip install pypdf` (PDF text extraction)

## Verify Installation

```bash
wikimind doctor
# ✓ git: git version 2.x.x
# ✓ python3: Python 3.x.x
# ✓ pypdf: x.x.x (or ✗ if not installed)
```

## Uninstall

```bash
# Remove binaries
rm $(which wikimind) $(which wikimindd)

# Remove Homebrew (if installed via brew)
brew uninstall wikimind
```
