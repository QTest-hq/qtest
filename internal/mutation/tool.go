package mutation

import (
	"context"
	"fmt"
)

// Tool defines the interface for mutation testing tools
type Tool interface {
	// Name returns the name of the mutation tool
	Name() string

	// Run executes mutation testing for the given source and test files
	Run(ctx context.Context, sourceFile, testFile string, cfg MutationConfig) (*Result, error)

	// IsAvailable checks if the tool is installed and available
	IsAvailable(ctx context.Context) bool
}

// Runner orchestrates mutation testing across different tools
type Runner struct {
	tools []Tool
}

// NewRunner creates a new mutation testing runner
func NewRunner(tools ...Tool) *Runner {
	return &Runner{tools: tools}
}

// AddTool adds a mutation testing tool
func (r *Runner) AddTool(tool Tool) {
	r.tools = append(r.tools, tool)
}

// Run executes mutation testing using the appropriate tool for the file type
func (r *Runner) Run(ctx context.Context, sourceFile, testFile string, cfg MutationConfig) (*Result, error) {
	if len(r.tools) == 0 {
		return nil, fmt.Errorf("no mutation testing tools configured")
	}

	// Find an available tool
	var tool Tool
	for _, t := range r.tools {
		if t.IsAvailable(ctx) {
			tool = t
			break
		}
	}

	if tool == nil {
		return nil, fmt.Errorf("no mutation testing tool available")
	}

	return tool.Run(ctx, sourceFile, testFile, cfg)
}

// GetAvailableTools returns all available tools
func (r *Runner) GetAvailableTools(ctx context.Context) []Tool {
	var available []Tool
	for _, t := range r.tools {
		if t.IsAvailable(ctx) {
			available = append(available, t)
		}
	}
	return available
}
