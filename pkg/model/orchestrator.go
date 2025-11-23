// Package model provides the orchestrator for building SystemModels
package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QTest-hq/qtest/internal/depgraph"
	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/rs/zerolog/log"
)

// OrchestratorConfig configures the model building process
type OrchestratorConfig struct {
	// Repository metadata
	Repository string
	Branch     string
	CommitSHA  string

	// Parsing options
	ParallelParsing bool
	ParallelWorkers int // 0 means auto-detect

	// Filtering
	ExcludeDirs    []string
	ExcludeFiles   []string
	IncludeOnly    []string // if non-empty, only include these patterns
	SkipTestFiles  bool

	// Analysis options
	BuildDependencyGraph bool
	ComputeRiskScores    bool
	GenerateTestTargets  bool

	// Risk weight configuration
	RiskWeights RiskWeights
}

// RiskWeights configures how risk scores are computed
type RiskWeights struct {
	Complexity float64 // weight for complexity (LOC-based)
	Centrality float64 // weight for how many things call this
	Churn      float64 // weight for code churn
	Depth      float64 // weight for dependency depth
}

// DefaultRiskWeights returns the default risk weights
func DefaultRiskWeights() RiskWeights {
	return RiskWeights{
		Complexity: 0.4,
		Centrality: 0.3,
		Churn:      0.2,
		Depth:      0.1,
	}
}

// BuildStats tracks statistics about the model building process
type BuildStats struct {
	// Timing
	StartTime    time.Time
	EndTime      time.Time
	ParseTime    time.Duration
	AnalysisTime time.Duration

	// File counts
	FilesFound   int
	FilesParsed  int
	FilesSkipped int
	ParseErrors  int

	// Model counts
	FunctionsFound int
	ClassesFound   int
	EndpointsFound int

	// Dependency graph stats
	GraphNodes     int
	GraphEdges     int
	CyclesDetected int

	// Supplement stats
	SupplementsRun    int
	SupplementResults map[string]SupplementResult
}

// SupplementResult tracks results from a supplement
type SupplementResult struct {
	Name          string
	Detected      bool
	EndpointsAdded int
	EventsAdded   int
	Duration      time.Duration
	Error         error
}

// Orchestrator coordinates the building of a SystemModel
type Orchestrator struct {
	config      *OrchestratorConfig
	parser      *parser.Parser
	builder     *Builder
	supplements []Supplement

	// State
	stats  BuildStats
	errors []error
	mu     sync.Mutex
}

// NewOrchestrator creates a new orchestrator with the given config
func NewOrchestrator(cfg *OrchestratorConfig) *Orchestrator {
	if cfg.RiskWeights.Complexity == 0 && cfg.RiskWeights.Centrality == 0 {
		cfg.RiskWeights = DefaultRiskWeights()
	}
	if cfg.ParallelWorkers == 0 {
		cfg.ParallelWorkers = 4 // default workers
	}
	if cfg.SkipTestFiles {
		cfg.ExcludeFiles = append(cfg.ExcludeFiles,
			"_test.go", ".test.js", ".test.ts", ".spec.js", ".spec.ts",
			"test_", "_test.py")
	}
	if len(cfg.ExcludeDirs) == 0 {
		cfg.ExcludeDirs = []string{
			"node_modules", "vendor", ".git", "__pycache__",
			".venv", "venv", "dist", "build", "target", ".next",
		}
	}

	return &Orchestrator{
		config:      cfg,
		parser:      parser.NewParser(),
		supplements: make([]Supplement, 0),
		stats: BuildStats{
			SupplementResults: make(map[string]SupplementResult),
		},
	}
}

// RegisterSupplement adds a framework supplement to the orchestrator
func (o *Orchestrator) RegisterSupplement(s Supplement) {
	o.supplements = append(o.supplements, s)
}

// Build constructs a SystemModel from the given root path
func (o *Orchestrator) Build(ctx context.Context, rootPath string) (*SystemModel, error) {
	o.stats.StartTime = time.Now()
	defer func() {
		o.stats.EndTime = time.Now()
	}()

	// Initialize builder
	o.builder = NewBuilder(o.config.Repository, o.config.Branch, o.config.CommitSHA)

	// Register supplements with builder
	for _, s := range o.supplements {
		o.builder.RegisterSupplement(s)
	}

	// Phase 1: Parse all files
	parseStart := time.Now()
	parsedFiles, err := o.parseDirectory(ctx, rootPath)
	if err != nil {
		return nil, fmt.Errorf("parsing failed: %w", err)
	}
	o.stats.ParseTime = time.Since(parseStart)

	// Phase 2: Add parsed files to builder
	for _, pf := range parsedFiles {
		functions := convertParserFunctions(pf.Functions)
		classes := convertParserClasses(pf.Classes)
		o.builder.AddParsedFile(pf.Path, string(pf.Language), functions, classes)
	}

	// Phase 3: Build dependency graph if enabled
	analysisStart := time.Now()
	if o.config.BuildDependencyGraph {
		if err := o.buildDependencyGraph(parsedFiles); err != nil {
			log.Warn().Err(err).Msg("failed to build dependency graph")
		}
	}

	// Phase 4: Build the final model (runs supplements, risk scores, test targets)
	model, err := o.builder.Build()
	if err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	o.stats.AnalysisTime = time.Since(analysisStart)

	// Phase 5: Validate the model
	if err := o.validateModel(model); err != nil {
		log.Warn().Err(err).Msg("model validation warnings")
	}

	// Update final stats
	o.stats.FunctionsFound = len(model.Functions)
	o.stats.ClassesFound = len(model.Types)
	o.stats.EndpointsFound = len(model.Endpoints)

	return model, nil
}

// parseDirectory parses all source files in a directory
func (o *Orchestrator) parseDirectory(ctx context.Context, rootPath string) ([]*parser.ParsedFile, error) {
	var files []string

	// Walk directory to find source files
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Skip directories
		if info.IsDir() {
			name := info.Name()
			for _, exclude := range o.config.ExcludeDirs {
				if name == exclude || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check file exclusions
		if o.shouldSkipFile(path) {
			o.stats.FilesSkipped++
			return nil
		}

		// Check if it's a supported source file
		lang := parser.DetectLanguage(path)
		if lang == parser.LanguageUnknown {
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	o.stats.FilesFound = len(files)

	// Parse files
	if o.config.ParallelParsing {
		return o.parseFilesParallel(ctx, files)
	}
	return o.parseFilesSequential(ctx, files)
}

// shouldSkipFile checks if a file should be skipped
func (o *Orchestrator) shouldSkipFile(path string) bool {
	base := filepath.Base(path)

	// Check exclusion patterns
	for _, pattern := range o.config.ExcludeFiles {
		if strings.Contains(base, pattern) || strings.Contains(path, pattern) {
			return true
		}
	}

	// Check inclusion patterns (if specified)
	if len(o.config.IncludeOnly) > 0 {
		included := false
		for _, pattern := range o.config.IncludeOnly {
			if strings.Contains(path, pattern) {
				included = true
				break
			}
		}
		if !included {
			return true
		}
	}

	return false
}

// parseFilesSequential parses files one at a time
func (o *Orchestrator) parseFilesSequential(ctx context.Context, files []string) ([]*parser.ParsedFile, error) {
	var result []*parser.ParsedFile

	for _, path := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		pf, err := o.parser.ParseFile(ctx, path)
		if err != nil {
			log.Debug().Err(err).Str("path", path).Msg("failed to parse file")
			o.stats.ParseErrors++
			continue
		}

		result = append(result, pf)
		o.stats.FilesParsed++
	}

	return result, nil
}

// parseFilesParallel parses files concurrently
func (o *Orchestrator) parseFilesParallel(ctx context.Context, files []string) ([]*parser.ParsedFile, error) {
	type parseResult struct {
		file *parser.ParsedFile
		err  error
	}

	jobs := make(chan string, len(files))
	results := make(chan parseResult, len(files))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < o.config.ParallelWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := parser.NewParser() // each worker needs its own parser
			for path := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				pf, err := p.ParseFile(ctx, path)
				results <- parseResult{file: pf, err: err}
			}
		}()
	}

	// Send jobs
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var parsed []*parser.ParsedFile
	for r := range results {
		if r.err != nil {
			o.mu.Lock()
			o.stats.ParseErrors++
			o.mu.Unlock()
			continue
		}
		o.mu.Lock()
		parsed = append(parsed, r.file)
		o.stats.FilesParsed++
		o.mu.Unlock()
	}

	return parsed, nil
}

// buildDependencyGraph constructs and integrates the dependency graph
func (o *Orchestrator) buildDependencyGraph(parsedFiles []*parser.ParsedFile) error {
	// Convert []*ParsedFile to []ParsedFile for the builder
	files := make([]parser.ParsedFile, len(parsedFiles))
	for i, pf := range parsedFiles {
		files[i] = *pf
	}

	graphBuilder := depgraph.NewBuilder()
	graph := graphBuilder.Build(files)

	// Update stats
	stats := graph.GetStats()
	o.stats.GraphNodes = stats.TotalNodes
	o.stats.GraphEdges = stats.TotalEdges

	// Detect cycles
	cycles := graph.FindCycles()
	o.stats.CyclesDetected = len(cycles)
	if len(cycles) > 0 {
		log.Warn().Int("count", len(cycles)).Msg("dependency cycles detected")
	}

	// Populate CallGraph from dependency edges
	for _, edge := range graph.Edges {
		if edge.Type == depgraph.EdgeTypeCalls || edge.Type == depgraph.EdgeTypeUses {
			o.builder.model.CallGraph = append(o.builder.model.CallGraph, CallEdge{
				Caller: edge.From,
				Callee: edge.To,
			})
		}
	}

	// Compute dependency depths and add to risk calculation
	o.addDependencyDepthRisk(graph)

	return nil
}

// addDependencyDepthRisk adds risk based on dependency depth
func (o *Orchestrator) addDependencyDepthRisk(graph *depgraph.Graph) {
	// For each function node, compute its transitive dependency count
	for _, node := range graph.Nodes {
		if node.Type == depgraph.NodeTypeFunction {
			deps := graph.GetTransitiveDependencies(node.ID, 0) // unlimited depth
			depth := len(deps)

			// Higher depth = more risk (more things can break)
			var depthScore float64
			if depth > 20 {
				depthScore = 0.9
			} else if depth > 10 {
				depthScore = 0.6
			} else if depth > 5 {
				depthScore = 0.3
			} else {
				depthScore = 0.1
			}

			// Store in metadata for later use by risk scorer
			if node.Metadata == nil {
				node.Metadata = make(map[string]string)
			}
			node.Metadata["dependency_depth"] = fmt.Sprintf("%d", depth)
			node.Metadata["depth_risk"] = fmt.Sprintf("%.2f", depthScore)
		}
	}
}

// validateModel performs validation checks on the built model
func (o *Orchestrator) validateModel(model *SystemModel) error {
	var warnings []string

	// Check for duplicate function IDs
	funcIDs := make(map[string]bool)
	for _, fn := range model.Functions {
		if funcIDs[fn.ID] {
			warnings = append(warnings, fmt.Sprintf("duplicate function ID: %s", fn.ID))
		}
		funcIDs[fn.ID] = true
	}

	// Check for duplicate type IDs
	typeIDs := make(map[string]bool)
	for _, t := range model.Types {
		if typeIDs[t.ID] {
			warnings = append(warnings, fmt.Sprintf("duplicate type ID: %s", t.ID))
		}
		typeIDs[t.ID] = true
	}

	// Check CallGraph references
	for _, edge := range model.CallGraph {
		if !funcIDs[edge.Caller] && !strings.HasPrefix(edge.Caller, "file:") {
			warnings = append(warnings, fmt.Sprintf("CallGraph references unknown caller: %s", edge.Caller))
		}
	}

	// Check endpoint handlers exist
	for _, ep := range model.Endpoints {
		if ep.Handler != "" && !funcIDs[ep.Handler] {
			// Handler might be a method or external - just log it
			log.Debug().Str("handler", ep.Handler).Str("endpoint", ep.Path).
				Msg("endpoint handler not found in functions")
		}
	}

	if len(warnings) > 0 {
		return fmt.Errorf("validation warnings: %s", strings.Join(warnings, "; "))
	}

	return nil
}

// Stats returns the build statistics
func (o *Orchestrator) Stats() BuildStats {
	return o.stats
}

// Errors returns any errors encountered during building
func (o *Orchestrator) Errors() []error {
	return o.errors
}

// Helper functions to convert parser types to builder types

func convertParserFunctions(funcs []parser.Function) []ParsedFunction {
	result := make([]ParsedFunction, len(funcs))
	for i, fn := range funcs {
		result[i] = ParsedFunction{
			Name:       fn.Name,
			StartLine:  fn.StartLine,
			EndLine:    fn.EndLine,
			Parameters: convertParserParams(fn.Parameters),
			Class:      fn.Class,
			Exported:   fn.Exported,
			Async:      fn.Async,
			Body:       fn.Body,
			DocComment: fn.Comments,
		}
	}
	return result
}

func convertParserClasses(classes []parser.Class) []ParsedClass {
	result := make([]ParsedClass, len(classes))
	for i, cls := range classes {
		result[i] = ParsedClass{
			Name:       cls.Name,
			StartLine:  cls.StartLine,
			EndLine:    cls.EndLine,
			Methods:    convertParserFunctions(cls.Methods),
			Properties: convertParserProperties(cls.Properties),
			Extends:    cls.Extends,
			Implements: cls.Implements,
			Exported:   cls.Exported,
		}
	}
	return result
}

func convertParserParams(params []parser.Parameter) []ParsedParam {
	result := make([]ParsedParam, len(params))
	for i, p := range params {
		result[i] = ParsedParam{
			Name:     p.Name,
			Type:     p.Type,
			Optional: p.Optional,
			Default:  p.Default,
		}
	}
	return result
}

func convertParserProperties(props []parser.Property) []ParsedProperty {
	result := make([]ParsedProperty, len(props))
	for i, p := range props {
		result[i] = ParsedProperty{
			Name:     p.Name,
			Type:     p.Type,
			Exported: p.Exported,
		}
	}
	return result
}
