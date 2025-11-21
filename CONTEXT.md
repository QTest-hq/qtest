# QTest Development Context Document

**Date:** 2025-11-21
**Purpose:** Resume development from current state

---

## Project Overview

QTest is an AI-powered test generation platform that:
1. Parses source code using tree-sitter (Go, Python, TypeScript, JavaScript)
2. Sends function code to LLM (Ollama) to generate test specifications in YAML
3. Converts YAML DSL to language-specific test code using adapters
4. Supports workspace-based incremental test generation

---

## Architecture

```
cmd/cli/main.go          - CLI entry point (cobra commands)
internal/
  adapters/              - Framework adapters (Go, Python, Jest)
    registry.go          - Adapter registry
    go_adapter.go        - Go test generation
    python_adapter.go    - Python/pytest generation
    jest_adapter.go      - Jest/TypeScript generation
  config/config.go       - Configuration management
  generator/
    generator.go         - Main test generator (LLM orchestration)
    converter.go         - Converts LLM YAML to DSL
  llm/
    router.go            - LLM routing by tier
    ollama.go            - Ollama client
    prompts.go           - System prompts for test generation
  parser/
    parser.go            - Multi-language parser using tree-sitter
    languages.go         - Language detection
  workspace/
    workspace.go         - Workspace management
    targets.go           - Test target discovery
    artifacts.go         - Build artifacts
    validation.go        - Test validation
    coverage.go          - Coverage collection
pkg/dsl/types.go         - DSL type definitions
```

---

## Current State

### What Works
- `qtest parse -f <file>` - Parses source files, extracts functions
- `qtest generate-file -f <file> -t <tier> -m <max>` - Generates test DSL via LLM
- `qtest generate-file -f <file> --write` - Generates and writes test files
- `qtest workspace init <repo-url>` - Clones repo, discovers targets
- `qtest workspace list/status` - Shows workspaces and their status

### LLM Configuration
- Tier 1 (fast): qwen2.5-coder:7b
- Tier 2 (balanced): deepseek-coder-v2:16b
- Tier 3 (thorough): deepseek-coder-v2:16b
- Requires: `ollama serve` running

---

## Recent Fixes Applied

### 1. DSL Format Conversion (converter.go)
**Problem:** LLM returns simple YAML format, not our full DSL structure.

**Solution:** Created `internal/generator/converter.go` that handles multiple formats:
- List of simple tests: `[{name, setup, action, assertions}, ...]`
- Wrapper format: `{tests: [...]}`
- Single test format: `{name, setup, action, assertions}`
- Full DSL format: `{version, name, type, target, steps}`

### 2. Go Test Template (go_adapter.go)
**Problem:** Multiple `result :=` declarations in same function scope caused compile errors.

**Solution:** Updated template to use:
```go
var result interface{}
_ = result
// Then for each step:
result = FunctionCall(args)
```

### 3. Variable Reference Handling (go_adapter.go)
**Problem:** LLM sometimes returns `${a}, ${b}` or `$a, $b` template variables that weren't resolved.

**Solution:** Added `formatGoArg()` function that:
- Detects `${var}`, `$var`, `*var` patterns
- Returns `0` as default value for unresolved variables
- Properly formats strings, numbers, bools

### 4. Package Name Extraction (go_adapter.go)
**Problem:** Test file had wrong package name causing `found packages X and Y` error.

**Solution:** Updated `extractPackageName()` to:
- Read actual source file
- Parse `package X` declaration
- Fall back to directory name

---

## Files Modified (Key Changes)

### /home/satish/QTest/internal/generator/converter.go (NEW)
```go
// Key types and functions:
type SimpleTest struct {
    Name       string                 `yaml:"name"`
    Setup      map[string]interface{} `yaml:"setup,omitempty"`
    Action     interface{}            `yaml:"action"`
    Assertions interface{}            `yaml:"assertions"`
}

func ConvertToDSL(yamlContent string, funcName, filePath, language string) (*dsl.TestDSL, error)
func parseAction(action interface{}, funcName string) []interface{}
func parseActionArgs(action string) []interface{}
func parseAssertions(assertions interface{}) *dsl.Expected
```

### /home/satish/QTest/internal/adapters/go_adapter.go
Key sections:
- Lines 32-57: Updated template with `var result interface{}`
- Lines 201-227: `generateGoStepAction()` returns just function call
- Lines 261-291: `formatGoArg()` handles variable references

### /home/satish/QTest/internal/generator/generator.go
- Line 132: Now uses `ConvertToDSL()` instead of direct YAML unmarshal

### /home/satish/QTest/cmd/cli/main.go
- Lines 77-161: `generate-file` command with `--write` flag
- Lines 233-302: `writeTestFiles()` function

---

## Test Files

### /home/satish/QTest/examples/math.go
```go
package math

func Add(a, b int) int {
    return a + b
}

func Multiply(a, b int) int {
    return a * b
}
```

### /home/satish/QTest/examples/math_test.go (OLD - needs regeneration)
This file was generated BEFORE the fixes and has:
- Multiple `result :=` declarations (compile error)
- `${a}, ${b}` unresolved variables (compile error)

**Must delete and regenerate after rebuild.**

---

## Commands to Resume

```bash
# 1. Ensure Ollama is running
ollama serve

# 2. Build the binary
cd /home/satish/QTest
go build -o ./bin/qtest ./cmd/cli/

# 3. Verify build
./bin/qtest --version

# 4. Delete old broken test file
rm examples/math_test.go

# 5. Generate new tests with fixes
./bin/qtest generate-file -f examples/math.go -t 1 -m 2 --write

# 6. View generated test
cat examples/math_test.go

# 7. Run tests
cd examples && go test -v
```

---

## Expected Output After Fix

The generated `math_test.go` should look like:
```go
package math

import (
    "testing"
)

func TestMathCombined(t *testing.T) {
    var result interface{}
    _ = result

    // Add two positive integers
    result = Add(2, 3)
    if result != 5 {
        t.Errorf("expected %v, got %v", 5, result)
    }

    // Multiply positive integers
    result = Multiply(2, 3)
    if result != 6 {
        t.Errorf("expected %v, got %v", 6, result)
    }
}
```

---

## Pending Tasks

1. **Test the fixes** - Rebuild, regenerate tests, verify they compile and pass
2. **Push to GitHub** - Commit all changes with proper message
3. **Future enhancements:**
   - Add testify assertions support
   - Add table-driven test generation
   - Improve LLM prompts for better test quality
   - Add integration test generation
   - Add coverage threshold enforcement

---

## Known Issues

1. **Bash shell not responding** - In the Claude Code session, bash commands were returning exit code 1 with no output. This appears to be a session issue, not a code issue.

2. **LLM output variability** - Different runs may produce different test formats. The converter handles most cases but edge cases may exist.

---

## Repository Structure

```
/home/satish/QTest/
├── bin/qtest              # Built binary
├── cmd/cli/main.go        # CLI entry
├── examples/
│   ├── math.go            # Test source file
│   └── math_test.go       # Generated tests (delete & regenerate)
├── internal/
│   ├── adapters/          # go_adapter.go, python_adapter.go, etc.
│   ├── config/
│   ├── generator/         # generator.go, converter.go
│   ├── llm/
│   ├── parser/
│   └── workspace/
├── pkg/dsl/types.go
├── go.mod
├── go.sum
└── CONTEXT.md             # This file
```

---

## How to Continue

1. Start new Claude Code session
2. Say: "Continue QTest development. Read /home/satish/QTest/CONTEXT.md for current state."
3. Run the commands in "Commands to Resume" section
4. If tests pass, commit and push
5. If tests fail, debug based on error messages
