package mutation

import (
	"context"
	"encoding/xml"
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

// PITTool implements mutation testing using PIT (Pitest) for Java
type PITTool struct {
	// MavenPath is the path to maven binary (default: mvn)
	MavenPath string
	// GradlePath is the path to gradle binary (default: gradle)
	GradlePath string
	// UseMaven uses Maven to run PIT (default: true)
	UseMaven bool
	// UseGradle uses Gradle to run PIT
	UseGradle bool
}

// NewPITTool creates a new PIT mutation testing tool
func NewPITTool() *PITTool {
	return &PITTool{
		MavenPath:  "mvn",
		GradlePath: "gradle",
		UseMaven:   true,
		UseGradle:  false,
	}
}

// Name returns the tool name
func (t *PITTool) Name() string {
	return "pit"
}

// IsAvailable checks if PIT is available
func (t *PITTool) IsAvailable(ctx context.Context) bool {
	// Check if Java is available
	cmd := exec.CommandContext(ctx, "java", "-version")
	if err := cmd.Run(); err != nil {
		return false
	}

	// Check if build tool is available
	if t.UseMaven {
		cmd = exec.CommandContext(ctx, t.MavenPath, "--version")
		return cmd.Run() == nil
	}

	if t.UseGradle {
		cmd = exec.CommandContext(ctx, t.GradlePath, "--version")
		return cmd.Run() == nil
	}

	return false
}

// Run executes PIT mutation testing on the source file
func (t *PITTool) Run(ctx context.Context, sourceFile, testFile string, cfg MutationConfig) (*Result, error) {
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

	// Get the project directory (where pom.xml or build.gradle should be)
	projectDir := findJavaProjectRoot(filepath.Dir(sourceFile))

	// Extract the target class and test class from file paths
	targetClass := extractJavaClassName(sourceFile)
	testClass := extractJavaClassName(testFile)

	if targetClass == "" || testClass == "" {
		result.Error = "could not determine Java class names from file paths"
		return result, nil
	}

	// Build and run PIT command
	var cmd *exec.Cmd
	var reportDir string

	if t.UseMaven {
		reportDir = filepath.Join(projectDir, "target", "pit-reports")
		cmd = t.buildMavenCommand(ctx, projectDir, targetClass, testClass, cfg)
	} else if t.UseGradle {
		reportDir = filepath.Join(projectDir, "build", "reports", "pitest")
		cmd = t.buildGradleCommand(ctx, projectDir, targetClass, testClass, cfg)
	} else {
		result.Error = "no build tool configured for PIT"
		return result, nil
	}

	cmd.Dir = projectDir

	log.Debug().
		Str("source", sourceFile).
		Str("test", testFile).
		Str("targetClass", targetClass).
		Str("testClass", testClass).
		Str("dir", projectDir).
		Msg("running PIT mutation testing")

	// Run PIT
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	result.Duration = time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = "mutation testing timed out"
		return result, nil
	}

	// Try to parse XML report
	xmlReportPath := filepath.Join(reportDir, "mutations.xml")
	if _, statErr := os.Stat(xmlReportPath); statErr == nil {
		if parseErr := t.parsePITXMLReport(xmlReportPath, result); parseErr != nil {
			log.Warn().Err(parseErr).Msg("failed to parse PIT XML report, falling back to output parsing")
			t.parsePITOutput(outputStr, result)
		}
	} else {
		// Try HTML report parsing as fallback
		htmlReportPath := filepath.Join(reportDir, "index.html")
		if _, statErr := os.Stat(htmlReportPath); statErr == nil {
			t.parsePITHTMLReport(htmlReportPath, result)
		} else {
			t.parsePITOutput(outputStr, result)
		}
	}

	// If we couldn't parse any results and there was an error, report it
	if result.Total == 0 && err != nil {
		result.Error = fmt.Sprintf("PIT failed: %v\nOutput: %s", err, truncateOutput(outputStr, 500))
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
		Msg("PIT mutation testing complete")

	return result, nil
}

// buildMavenCommand builds the Maven command to run PIT
func (t *PITTool) buildMavenCommand(ctx context.Context, projectDir, targetClass, testClass string, cfg MutationConfig) *exec.Cmd {
	args := []string{
		"org.pitest:pitest-maven:mutationCoverage",
		fmt.Sprintf("-DtargetClasses=%s", targetClass),
		fmt.Sprintf("-DtargetTests=%s", testClass),
		"-DoutputFormats=XML,HTML",
		"-DtimestampedReports=false",
	}

	// Add timeout configuration
	if cfg.TimeoutPerMutant > 0 {
		args = append(args, fmt.Sprintf("-DtimeoutConstant=%d", cfg.TimeoutPerMutant.Milliseconds()))
	}

	// Add max mutants if configured
	if cfg.MaxMutantsPerFunction > 0 {
		// PIT doesn't have per-function limit, use threads to limit parallelism
		args = append(args, fmt.Sprintf("-Dthreads=%d", 2))
	}

	// Suppress Maven output
	args = append(args, "-q")

	return exec.CommandContext(ctx, t.MavenPath, args...)
}

// buildGradleCommand builds the Gradle command to run PIT
func (t *PITTool) buildGradleCommand(ctx context.Context, projectDir, targetClass, testClass string, cfg MutationConfig) *exec.Cmd {
	args := []string{
		"pitest",
		fmt.Sprintf("-PtargetClasses=%s", targetClass),
		fmt.Sprintf("-PtargetTests=%s", testClass),
	}

	// Add timeout configuration
	if cfg.TimeoutPerMutant > 0 {
		args = append(args, fmt.Sprintf("-PtimeoutConstInMillis=%d", cfg.TimeoutPerMutant.Milliseconds()))
	}

	// Suppress Gradle output
	args = append(args, "-q")

	return exec.CommandContext(ctx, t.GradlePath, args...)
}

// PITMutations represents the XML structure of PIT mutations report
type PITMutations struct {
	XMLName   xml.Name     `xml:"mutations"`
	Mutations []PITMutant `xml:"mutation"`
}

// PITMutant represents a single mutant in PIT XML output
type PITMutant struct {
	Detected      bool   `xml:"detected,attr"`
	Status        string `xml:"status,attr"`
	MutatedClass  string `xml:"mutatedClass"`
	MutatedMethod string `xml:"mutatedMethod"`
	Mutator       string `xml:"mutator"`
	LineNumber    int    `xml:"lineNumber"`
	Description   string `xml:"description"`
	KillingTest   string `xml:"killingTest,omitempty"`
}

// parsePITXMLReport parses the PIT XML mutations report
func (t *PITTool) parsePITXMLReport(reportPath string, result *Result) error {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}

	var mutations PITMutations
	if err := xml.Unmarshal(data, &mutations); err != nil {
		return err
	}

	for _, m := range mutations.Mutations {
		mutant := Mutant{
			ID:          fmt.Sprintf("%s:%d:%s", m.MutatedClass, m.LineNumber, m.Mutator),
			Type:        t.mapMutatorName(m.Mutator),
			Description: m.Description,
			Line:        m.LineNumber,
		}

		switch m.Status {
		case "KILLED":
			mutant.Status = StatusKilled
			result.Killed++
		case "SURVIVED", "NO_COVERAGE":
			mutant.Status = StatusSurvived
			result.Survived++
		case "TIMED_OUT":
			mutant.Status = StatusTimeout
			result.Timeout++
		default:
			mutant.Status = StatusError
		}

		result.Mutants = append(result.Mutants, mutant)
		result.Total++
	}

	return nil
}

// parsePITHTMLReport parses the PIT HTML index report for summary
func (t *PITTool) parsePITHTMLReport(reportPath string, result *Result) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return
	}

	content := string(data)

	// Look for mutation score patterns in HTML
	// PIT HTML format: "Mutation Score: 75%"
	scoreRe := regexp.MustCompile(`Mutation\s+Score[:\s]+(\d+)%`)
	if matches := scoreRe.FindStringSubmatch(content); len(matches) >= 2 {
		score, _ := strconv.Atoi(matches[1])
		result.Score = float64(score) / 100.0
	}

	// Look for killed/survived counts
	// Format: "Killed: 10 (75%)"
	killedRe := regexp.MustCompile(`Killed[:\s]+(\d+)`)
	if matches := killedRe.FindStringSubmatch(content); len(matches) >= 2 {
		result.Killed, _ = strconv.Atoi(matches[1])
	}

	survivedRe := regexp.MustCompile(`Survived[:\s]+(\d+)`)
	if matches := survivedRe.FindStringSubmatch(content); len(matches) >= 2 {
		result.Survived, _ = strconv.Atoi(matches[1])
	}

	result.Total = result.Killed + result.Survived + result.Timeout
}

// parsePITOutput parses PIT console output when reports aren't available
func (t *PITTool) parsePITOutput(output string, result *Result) {
	/*
		PIT output format:
		================================================================================
		- Statistics
		================================================================================
		>> Line Coverage: 100/120 (83%)
		>> Generated 50 mutations Killed 40 (80%)
		>> Mutations with no coverage 5. Test strength 89%
		>> Ran 120 tests (2.4 tests per mutation)
	*/

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse mutation summary line
		// Format: ">> Generated X mutations Killed Y (Z%)"
		if strings.Contains(line, "Generated") && strings.Contains(line, "mutations") && strings.Contains(line, "Killed") {
			re := regexp.MustCompile(`Generated\s+(\d+)\s+mutations\s+Killed\s+(\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) >= 3 {
				result.Total, _ = strconv.Atoi(matches[1])
				result.Killed, _ = strconv.Atoi(matches[2])
				result.Survived = result.Total - result.Killed
			}
		}

		// Parse "Mutations with no coverage" line
		if strings.Contains(line, "Mutations with no coverage") {
			re := regexp.MustCompile(`no coverage\s+(\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) >= 2 {
				noCoverage, _ := strconv.Atoi(matches[1])
				// No coverage mutants count as survived
				if result.Survived < noCoverage {
					result.Survived = noCoverage
				}
			}
		}

		// Parse timed out mutants if present
		if strings.Contains(line, "timed out") || strings.Contains(line, "TIMED_OUT") {
			re := regexp.MustCompile(`(\d+)\s+timed\s+out`)
			if matches := re.FindStringSubmatch(line); len(matches) >= 2 {
				result.Timeout, _ = strconv.Atoi(matches[1])
			}
		}
	}

	// Recalculate total if needed
	if result.Total == 0 {
		result.Total = result.Killed + result.Survived + result.Timeout
	}
}

// mapMutatorName maps PIT mutator names to our generic types
func (t *PITTool) mapMutatorName(mutator string) string {
	mutator = strings.ToLower(mutator)

	// Check more specific patterns first
	switch {
	// Conditionals - check before "negate" since NegateConditionalsMutator contains "negate"
	case strings.Contains(mutator, "conditionalsboundary"):
		return "comparison"
	case strings.Contains(mutator, "negateconditionals"):
		return "comparison"
	case strings.Contains(mutator, "removeconditionals"):
		return "comparison"
	// Literals - check "null" before "return" since NullReturnsMutator contains "return"
	case strings.Contains(mutator, "null"):
		return "literal"
	case strings.Contains(mutator, "constant"):
		return "literal"
	// Arithmetic
	case strings.Contains(mutator, "math"):
		return "arithmetic"
	case strings.Contains(mutator, "negate"):
		return "arithmetic"
	case strings.Contains(mutator, "increment"):
		return "arithmetic"
	case strings.Contains(mutator, "invert"):
		return "arithmetic"
	// Boolean
	case strings.Contains(mutator, "true") || strings.Contains(mutator, "false"):
		return "boolean"
	// Statements
	case strings.Contains(mutator, "return"):
		return "statement"
	case strings.Contains(mutator, "void"):
		return "statement"
	case strings.Contains(mutator, "empty"):
		return "statement"
	default:
		return "unknown"
	}
}

// extractJavaClassName extracts the fully qualified class name from a Java file path
func extractJavaClassName(filePath string) string {
	// Remove .java extension
	if !strings.HasSuffix(filePath, ".java") {
		return ""
	}

	// Try to find src/main/java or src/test/java to determine package
	srcMainIdx := strings.Index(filePath, "src/main/java/")
	srcTestIdx := strings.Index(filePath, "src/test/java/")

	var relativePath string
	if srcMainIdx >= 0 {
		relativePath = filePath[srcMainIdx+len("src/main/java/"):]
	} else if srcTestIdx >= 0 {
		relativePath = filePath[srcTestIdx+len("src/test/java/"):]
	} else {
		// Fall back to just the file name without extension
		base := filepath.Base(filePath)
		return strings.TrimSuffix(base, ".java")
	}

	// Remove .java extension and convert path separators to dots
	relativePath = strings.TrimSuffix(relativePath, ".java")
	className := strings.ReplaceAll(relativePath, "/", ".")
	className = strings.ReplaceAll(className, "\\", ".")

	return className
}

// findJavaProjectRoot walks up directories to find pom.xml or build.gradle
func findJavaProjectRoot(start string) string {
	dir := start
	for {
		// Check for Java project markers
		if _, err := os.Stat(filepath.Join(dir, "pom.xml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "build.gradle")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "build.gradle.kts")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, return original directory
			return start
		}
		dir = parent
	}
}
