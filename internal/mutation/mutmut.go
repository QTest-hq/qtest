package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// MutmutTool implements mutation testing using mutmut for Python
type MutmutTool struct {
	// BinaryPath is the path to mutmut binary (default: mutmut)
	BinaryPath string
	// PythonPath is the path to Python interpreter
	PythonPath string
	// UsePipx uses pipx to run mutmut
	UsePipx bool
}

// NewMutmutTool creates a new mutmut mutation testing tool
func NewMutmutTool() *MutmutTool {
	return &MutmutTool{
		BinaryPath: "mutmut",
		PythonPath: "python3",
		UsePipx:    false,
	}
}

// Name returns the tool name
func (t *MutmutTool) Name() string {
	return "mutmut"
}

// IsAvailable checks if mutmut is installed and available
func (t *MutmutTool) IsAvailable(ctx context.Context) bool {
	// Check if Python is available
	cmd := exec.CommandContext(ctx, t.PythonPath, "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	// Check if mutmut is available
	var mutmutCmd *exec.Cmd
	if t.UsePipx {
		mutmutCmd = exec.CommandContext(ctx, "pipx", "run", "mutmut", "--version")
	} else {
		mutmutCmd = exec.CommandContext(ctx, t.BinaryPath, "--version")
	}
	err := mutmutCmd.Run()
	return err == nil
}

// Run executes mutmut mutation testing on the source file
func (t *MutmutTool) Run(ctx context.Context, sourceFile, testFile string, cfg MutationConfig) (*Result, error) {
	start := time.Now()

	result := &Result{
		SourceFile: sourceFile,
		TestFile:   testFile,
	}

	// Create context with timeout
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	// Get the project directory (where setup.py/pyproject.toml should be)
	projectDir := findPythonProjectRoot(filepath.Dir(sourceFile))

	// Create a temporary directory for mutmut cache
	cacheDir := filepath.Join(projectDir, ".mutmut-cache-tmp")
	os.MkdirAll(cacheDir, 0755)
	defer os.RemoveAll(cacheDir)

	// Determine paths relative to project
	relSourceFile, err := filepath.Rel(projectDir, sourceFile)
	if err != nil {
		relSourceFile = sourceFile
	}

	// Get test directory from test file path
	testDir := filepath.Dir(testFile)
	relTestDir, err := filepath.Rel(projectDir, testDir)
	if err != nil {
		relTestDir = testDir
	}

	// Build mutmut command args
	args := []string{
		"run",
		"--paths-to-mutate", relSourceFile,
		"--tests-dir", relTestDir,
		"--no-progress",
	}

	// Add timeout per mutant
	if cfg.TimeoutPerMutant > 0 {
		args = append(args, "--runner", fmt.Sprintf("python -m pytest --timeout=%d", int(cfg.TimeoutPerMutant.Seconds())))
	}

	// Build the command
	var cmd *exec.Cmd
	if t.UsePipx {
		cmdArgs := append([]string{"run", "mutmut"}, args...)
		cmd = exec.CommandContext(ctx, "pipx", cmdArgs...)
	} else {
		cmd = exec.CommandContext(ctx, t.BinaryPath, args...)
	}
	cmd.Dir = projectDir

	// Set environment to use the cache directory
	cmd.Env = append(os.Environ(), fmt.Sprintf("MUTMUT_CACHE_DIR=%s", cacheDir))

	log.Debug().
		Str("source", sourceFile).
		Str("test", testFile).
		Str("dir", projectDir).
		Strs("args", args).
		Msg("running mutmut mutation testing")

	// Run mutmut
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result.Duration = time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "mutation testing timed out"
		return result, nil
	}

	// Try to get results using mutmut results command
	resultsCmd := t.buildCommand(ctx, "results")
	resultsCmd.Dir = projectDir
	resultsCmd.Env = append(os.Environ(), fmt.Sprintf("MUTMUT_CACHE_DIR=%s", cacheDir))
	resultsOutput, resultsErr := resultsCmd.CombinedOutput()

	if resultsErr == nil {
		t.parseMutmutResults(string(resultsOutput), result)
	} else {
		// Fall back to parsing run output
		t.parseMutmutOutput(outputStr, result)
	}

	// Try to get detailed mutant information
	t.collectMutantDetails(ctx, projectDir, cacheDir, result)

	// If we couldn't parse any results and there was an error, report it
	if result.Total == 0 && err != nil {
		result.Error = fmt.Sprintf("mutmut failed: %v\nOutput: %s", err, truncateOutput(outputStr, 500))
	}

	// Calculate score
	if result.Total > 0 {
		result.Score = float64(result.Killed) / float64(result.Total)
	}

	log.Info().
		Str("source", sourceFile).
		Int("total", result.Total).
		Int("killed", result.Killed).
		Int("survived", result.Survived).
		Float64("score", result.Score).
		Dur("duration", result.Duration).
		Msg("mutmut mutation testing complete")

	return result, nil
}

// buildCommand builds a mutmut command with the given subcommand
func (t *MutmutTool) buildCommand(ctx context.Context, subCmd string, args ...string) *exec.Cmd {
	allArgs := append([]string{subCmd}, args...)
	if t.UsePipx {
		cmdArgs := append([]string{"run", "mutmut"}, allArgs...)
		return exec.CommandContext(ctx, "pipx", cmdArgs...)
	}
	return exec.CommandContext(ctx, t.BinaryPath, allArgs...)
}

// MutmutJSONResult represents the JSON output structure from mutmut
type MutmutJSONResult struct {
	Survived     []int `json:"survived"`
	Killed       []int `json:"killed"`
	Timeout      []int `json:"timeout"`
	Suspicious   []int `json:"suspicious"`
	Skipped      []int `json:"skipped"`
	Untested     []int `json:"untested"`
	TotalMutants int   `json:"total_mutants"`
}

// parseMutmutResults parses the output of `mutmut results`
func (t *MutmutTool) parseMutmutResults(output string, result *Result) {
	/*
		mutmut results output format:
		Legend for output:
		🎉 Killed mutants.   Count: 10
		⏰ Timeout.          Count: 1
		🤔 Suspicious.       Count: 0
		🙁 Survived.         Count: 3
		🔇 Skipped.          Count: 0

		Or older format:
		Killed mutants: 10
		Survived mutants: 3
		Timeout: 1
	*/

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// New format with emojis
		if strings.Contains(line, "Killed") && strings.Contains(line, "Count:") {
			result.Killed = extractCount(line)
		}
		if strings.Contains(line, "Survived") && strings.Contains(line, "Count:") {
			result.Survived = extractCount(line)
		}
		if strings.Contains(line, "Timeout") && strings.Contains(line, "Count:") {
			result.Timeout = extractCount(line)
		}

		// Old format
		if strings.HasPrefix(line, "Killed mutants:") {
			fmt.Sscanf(line, "Killed mutants: %d", &result.Killed)
		}
		if strings.HasPrefix(line, "Survived mutants:") {
			fmt.Sscanf(line, "Survived mutants: %d", &result.Survived)
		}
		if strings.HasPrefix(line, "Timeout:") {
			fmt.Sscanf(line, "Timeout: %d", &result.Timeout)
		}
	}

	result.Total = result.Killed + result.Survived + result.Timeout
}

// parseMutmutOutput parses mutmut run output when results command isn't available
func (t *MutmutTool) parseMutmutOutput(output string, result *Result) {
	/*
		mutmut run output format:
		Running tests without mutations... Done in 0.5 seconds
		Running mutation testing... Done in 10.5 seconds
		⠋ 15/15  🎉 10  ⏰ 1  🤔 0  🙁 3  🔇 0
	*/

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for progress line with results
		// Format: X/Y 🎉 killed ⏰ timeout 🤔 suspicious 🙁 survived
		if strings.Contains(line, "🎉") || strings.Contains(line, "🙁") {
			// Parse the emoji-based format
			parts := strings.Fields(line)
			for i := 0; i < len(parts)-1; i++ {
				switch parts[i] {
				case "🎉":
					result.Killed, _ = strconv.Atoi(parts[i+1])
				case "🙁":
					result.Survived, _ = strconv.Atoi(parts[i+1])
				case "⏰":
					result.Timeout, _ = strconv.Atoi(parts[i+1])
				}
			}

			// Try to get total from X/Y format
			if len(parts) > 0 {
				if idx := strings.Index(parts[0], "/"); idx > 0 {
					total, _ := strconv.Atoi(parts[0][idx+1:])
					if total > 0 {
						result.Total = total
					}
				}
			}
		}

		// Also try simple number format: "10 killed, 3 survived"
		if strings.Contains(line, "killed") && strings.Contains(line, "survived") {
			fmt.Sscanf(line, "%d killed, %d survived", &result.Killed, &result.Survived)
		}
	}

	// Calculate total if not found
	if result.Total == 0 {
		result.Total = result.Killed + result.Survived + result.Timeout
	}
}

// extractCount extracts a count value from a line like "🎉 Killed mutants.   Count: 10"
func extractCount(line string) int {
	re := regexp.MustCompile(`Count:\s*(\d+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		count, _ := strconv.Atoi(matches[1])
		return count
	}
	return 0
}

// collectMutantDetails collects detailed information about each mutant
func (t *MutmutTool) collectMutantDetails(ctx context.Context, projectDir, cacheDir string, result *Result) {
	// Try to get list of surviving mutants
	survivedCmd := t.buildCommand(ctx, "results", "--json-output")
	survivedCmd.Dir = projectDir
	survivedCmd.Env = append(os.Environ(), fmt.Sprintf("MUTMUT_CACHE_DIR=%s", cacheDir))
	jsonOutput, err := survivedCmd.Output()

	if err == nil && len(jsonOutput) > 0 {
		// Try to parse JSON output
		var jsonResult map[string]interface{}
		if err := json.Unmarshal(jsonOutput, &jsonResult); err == nil {
			t.parseJSONMutants(jsonResult, result)
			return
		}
	}

	// Fall back to getting individual mutant details
	for i := 1; i <= result.Total && i <= 100; i++ { // Limit to 100 mutants
		showCmd := t.buildCommand(ctx, "show", strconv.Itoa(i))
		showCmd.Dir = projectDir
		showCmd.Env = append(os.Environ(), fmt.Sprintf("MUTMUT_CACHE_DIR=%s", cacheDir))
		showOutput, err := showCmd.Output()

		if err != nil {
			continue
		}

		mutant := t.parseMutantShow(string(showOutput), i)
		if mutant != nil {
			result.Mutants = append(result.Mutants, *mutant)
		}
	}
}

// parseJSONMutants parses mutants from JSON output
func (t *MutmutTool) parseJSONMutants(data map[string]interface{}, result *Result) {
	// Structure varies by mutmut version
	// Try to extract mutant details from common structures

	if mutants, ok := data["mutants"].([]interface{}); ok {
		for _, m := range mutants {
			if mutant, ok := m.(map[string]interface{}); ok {
				result.Mutants = append(result.Mutants, Mutant{
					ID:          fmt.Sprintf("%v", mutant["id"]),
					Type:        t.mapMutationType(fmt.Sprintf("%v", mutant["type"])),
					Description: fmt.Sprintf("%v", mutant["description"]),
					Line:        int(mutant["line"].(float64)),
					Status:      t.mapMutantStatus(fmt.Sprintf("%v", mutant["status"])),
				})
			}
		}
	}
}

// parseMutantShow parses the output of `mutmut show <id>`
func (t *MutmutTool) parseMutantShow(output string, id int) *Mutant {
	/*
		mutmut show output format:

		--- source.py
		+++ source.py
		@@ -10,1 +10,1 @@
		- return a + b
		+ return a - b

		Or:
		Mutant 1: (killed)
		--- a/source.py
		+++ b/source.py
		...
	*/

	mutant := &Mutant{
		ID: strconv.Itoa(id),
	}

	lines := strings.Split(output, "\n")
	var original, mutated string
	var lineNum int

	for i, line := range lines {
		// Parse status from header
		line = strings.TrimSpace(line)

		if strings.Contains(line, "(killed)") {
			mutant.Status = StatusKilled
		} else if strings.Contains(line, "(survived)") {
			mutant.Status = StatusSurvived
		} else if strings.Contains(line, "(timeout)") {
			mutant.Status = StatusTimeout
		}

		// Parse line number from @@ header
		if strings.HasPrefix(line, "@@") {
			// Format: @@ -10,1 +10,1 @@
			re := regexp.MustCompile(`@@ -(\d+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				lineNum, _ = strconv.Atoi(matches[1])
			}
		}

		// Parse diff lines
		if strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "--- ") {
			original = strings.TrimPrefix(line, "- ")
		}
		if strings.HasPrefix(line, "+ ") && !strings.HasPrefix(line, "+++ ") {
			mutated = strings.TrimPrefix(line, "+ ")
		}

		// Detect mutation type from the change
		if original != "" && mutated != "" && i > 0 {
			mutant.Type = t.detectMutationType(original, mutated)
			mutant.Description = fmt.Sprintf("Changed '%s' to '%s'", strings.TrimSpace(original), strings.TrimSpace(mutated))
			mutant.Original = original
			mutant.Mutated = mutated
			break
		}
	}

	mutant.Line = lineNum

	// Set default status if not found
	if mutant.Status == "" {
		mutant.Status = StatusSurvived
	}

	// Generate description if not set
	if mutant.Description == "" && original != "" {
		mutant.Description = fmt.Sprintf("Mutation at line %d", lineNum)
	}

	return mutant
}

// detectMutationType detects the type of mutation from original and mutated code
func (t *MutmutTool) detectMutationType(original, mutated string) string {
	original = strings.TrimSpace(original)
	mutated = strings.TrimSpace(mutated)

	// Boundary mutations (off-by-one) - check before arithmetic since "+1" to "-1" is more specific
	if (strings.Contains(original, "+1") && strings.Contains(mutated, "-1")) ||
		(strings.Contains(original, "-1") && strings.Contains(mutated, "+1")) {
		return "boundary"
	}

	// Arithmetic operators
	arithmeticOps := [][]string{{"+", "-"}, {"-", "+"}, {"*", "/"}, {"/", "*"}, {"//", "/"}, {"%", "/"}, {"**", "*"}}
	for _, ops := range arithmeticOps {
		if strings.Contains(original, ops[0]) && strings.Contains(mutated, ops[1]) {
			return "arithmetic"
		}
	}

	// Comparison operators
	comparisonOps := [][]string{{"==", "!="}, {"!=", "=="}, {"<", ">"}, {">", "<"}, {"<=", ">="}, {">=", "<="}, {"<", "<="}, {">", ">="}}
	for _, ops := range comparisonOps {
		if strings.Contains(original, ops[0]) && strings.Contains(mutated, ops[1]) {
			return "comparison"
		}
	}

	// Boolean operators
	boolOps := [][]string{{"and", "or"}, {"or", "and"}, {"True", "False"}, {"False", "True"}, {"not ", ""}}
	for _, ops := range boolOps {
		if strings.Contains(original, ops[0]) && strings.Contains(mutated, ops[1]) {
			return "boolean"
		}
	}

	// String/literal mutations
	if (strings.Contains(original, `"`) || strings.Contains(original, `'`)) &&
		(strings.Contains(mutated, `"`) || strings.Contains(mutated, `'`)) {
		return "literal"
	}

	// Return statement mutations
	if strings.Contains(original, "return") || strings.Contains(mutated, "return") {
		return "statement"
	}

	return "unknown"
}

// mapMutationType maps mutmut mutation names to our generic types
func (t *MutmutTool) mapMutationType(name string) string {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "arithmetic"):
		return "arithmetic"
	case strings.Contains(name, "comparison") || strings.Contains(name, "relational"):
		return "comparison"
	case strings.Contains(name, "boolean") || strings.Contains(name, "logical"):
		return "boolean"
	case strings.Contains(name, "string") || strings.Contains(name, "literal"):
		return "literal"
	case strings.Contains(name, "statement") || strings.Contains(name, "return"):
		return "statement"
	default:
		return "unknown"
	}
}

// mapMutantStatus maps mutmut status to our status constants
func (t *MutmutTool) mapMutantStatus(status string) string {
	status = strings.ToLower(status)
	switch {
	case strings.Contains(status, "killed"):
		return StatusKilled
	case strings.Contains(status, "survived"):
		return StatusSurvived
	case strings.Contains(status, "timeout"):
		return StatusTimeout
	default:
		return StatusError
	}
}

// findPythonProjectRoot walks up directories to find setup.py or pyproject.toml
func findPythonProjectRoot(start string) string {
	dir := start
	for {
		// Check for Python project markers
		markers := []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt"}
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, return original directory
			return start
		}
		dir = parent
	}
}
