# QTest - AI-Powered Test Generation Platform

## Project Overview

QTest generates comprehensive test suites using AI (Ollama LLMs). It parses source code, sends functions to LLMs for test specification generation, and converts the output to language-specific test code.

## Quick Start

```bash
# Build
go build -o ./bin/qtest ./cmd/cli/

# Parse a file
./bin/qtest parse -f <source-file>

# Generate tests (requires: ollama serve)
./bin/qtest generate-file -f <source-file> -t 1 -m 5 --write

# Workspace management
./bin/qtest workspace init <repo-url>
./bin/qtest workspace list
./bin/qtest workspace status <name>
```

## Architecture

```
cmd/cli/main.go              # CLI entry (cobra)
internal/
  adapters/                  # Framework adapters
    go_adapter.go            # Go test generation
    python_adapter.go        # pytest generation
    jest_adapter.go          # Jest generation
  generator/
    generator.go             # LLM orchestration
    converter.go             # YAML to DSL conversion
  llm/
    router.go                # Tier-based LLM routing
    ollama.go                # Ollama client
    prompts.go               # System prompts
  parser/parser.go           # Tree-sitter parsing
  workspace/                 # Workspace management
pkg/dsl/types.go             # DSL type definitions
```

## LLM Tiers

- **Tier 1** (fast): qwen2.5-coder:7b
- **Tier 2** (balanced): deepseek-coder-v2:16b
- **Tier 3** (thorough): deepseek-coder-v2:16b

## Key Commands

| Command | Description |
|---------|-------------|
| `qtest parse -f FILE` | Parse source, show functions |
| `qtest generate-file -f FILE -t TIER -m MAX --write` | Generate tests |
| `qtest workspace init URL` | Initialize workspace from repo |
| `qtest workspace list` | List workspaces |
| `qtest analyze -p PATH` | Analyze repository |

## Testing

```bash
# Test with example file
./bin/qtest generate-file -f examples/math.go -t 1 -m 2 --write
cd examples && go test -v
```

## Development Notes

### DSL Format
LLM returns simple YAML that `converter.go` transforms to full DSL:
```yaml
- name: "Test case name"
  setup: {a: 1, b: 2}
  action: "FunctionName(a, b)"
  assertions: {result: 3}
```

### Go Adapter Template
Uses `var result interface{}` with `result =` assignments to avoid redeclaration errors.

### Variable Handling
`formatGoArg()` in go_adapter.go handles unresolved `${var}` references by defaulting to `0`.

## Current Status

See `CONTEXT.md` for detailed development state and pending tasks.
