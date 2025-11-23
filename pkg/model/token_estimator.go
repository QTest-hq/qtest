package model

// TokenEstimator estimates token usage for LLM test generation
// This helps with cost prediction and resource planning.
//
// Token estimation uses character-based heuristics:
// - Code typically averages ~4 characters per token
// - System prompts are pre-calculated constants
// - Output tokens scale with input complexity

// Token constants for system prompts (pre-measured)
// These are approximate token counts for the IRSpec system prompt
const (
	// SystemPromptTokens is the base token count for the IRSpec system prompt
	// Calculated: ~2500 characters including schema and example
	SystemPromptTokens = 700

	// UserPromptBaseTokens is the overhead for the user prompt template
	// (excluding function code)
	UserPromptBaseTokens = 150

	// CharsPerToken is the average characters per token for code
	// Code tends to have more punctuation, so slightly lower than prose
	CharsPerToken = 4
)

// Output estimation multipliers by test level
const (
	// UnitTestOutputMultiplier: output tokens relative to input code tokens
	// Unit tests typically produce 2-3x the code volume in test output
	UnitTestOutputMultiplier = 2.5

	// APITestOutputMultiplier: API tests include more setup/teardown
	APITestOutputMultiplier = 3.0

	// E2ETestOutputMultiplier: E2E tests have extensive setup
	E2ETestOutputMultiplier = 4.0

	// MinOutputTokens ensures reasonable minimum for small functions
	MinOutputTokens = 200

	// MaxOutputTokens caps estimates for very large functions
	MaxOutputTokens = 4000
)

// TokenEstimate contains token usage estimates for a test generation request
type TokenEstimate struct {
	// InputTokens is the estimated input token count
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the estimated output token count
	OutputTokens int `json:"output_tokens"`

	// TotalTokens is the sum of input and output tokens
	TotalTokens int `json:"total_tokens"`

	// Components breaks down the input token sources
	Components TokenComponents `json:"components"`
}

// TokenComponents breaks down token sources for visibility
type TokenComponents struct {
	SystemPrompt int `json:"system_prompt"`
	UserPrompt   int `json:"user_prompt"`
	FunctionCode int `json:"function_code"`
	Context      int `json:"context"`
}

// EstimateTokens calculates token estimates for generating tests for a function
func EstimateTokens(fn *Function, level TestLevel, contextSize int) TokenEstimate {
	// Calculate function code tokens
	codeTokens := estimateCodeTokens(fn.Body)
	if codeTokens == 0 && fn.LOC > 0 {
		// Fallback: estimate from LOC if body not available
		// Average ~40 chars per line of code
		codeTokens = (fn.LOC * 40) / CharsPerToken
	}

	// Calculate context tokens
	contextTokens := contextSize / CharsPerToken

	// Input tokens = system prompt + user prompt base + code + context
	inputTokens := SystemPromptTokens + UserPromptBaseTokens + codeTokens + contextTokens

	// Output tokens based on test level and input complexity
	outputTokens := estimateOutputTokens(codeTokens, level, fn.Complexity)

	return TokenEstimate{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		Components: TokenComponents{
			SystemPrompt: SystemPromptTokens,
			UserPrompt:   UserPromptBaseTokens,
			FunctionCode: codeTokens,
			Context:      contextTokens,
		},
	}
}

// EstimateTokensForIntent estimates tokens for a test intent
func EstimateTokensForIntent(intent *TestIntent, model *SystemModel) TokenEstimate {
	switch intent.TargetKind {
	case "function":
		fn := model.GetFunction(intent.TargetID)
		if fn != nil {
			return EstimateTokens(fn, intent.Level, 0)
		}
	case "endpoint":
		ep := model.GetEndpoint(intent.TargetID)
		if ep != nil {
			// For endpoints, estimate based on handler if available
			if ep.Handler != "" {
				fn := model.GetFunction(ep.Handler)
				if fn != nil {
					return EstimateTokens(fn, LevelAPI, 200) // Extra context for endpoint info
				}
			}
			// Fallback for endpoint without handler reference
			return TokenEstimate{
				InputTokens:  SystemPromptTokens + UserPromptBaseTokens + 200,
				OutputTokens: int(MinOutputTokens * APITestOutputMultiplier),
				TotalTokens:  SystemPromptTokens + UserPromptBaseTokens + 200 + int(MinOutputTokens*APITestOutputMultiplier),
				Components: TokenComponents{
					SystemPrompt: SystemPromptTokens,
					UserPrompt:   UserPromptBaseTokens,
					Context:      200,
				},
			}
		}
	}

	// Default estimate for unknown targets
	return TokenEstimate{
		InputTokens:  SystemPromptTokens + UserPromptBaseTokens + 100,
		OutputTokens: MinOutputTokens,
		TotalTokens:  SystemPromptTokens + UserPromptBaseTokens + 100 + MinOutputTokens,
		Components: TokenComponents{
			SystemPrompt: SystemPromptTokens,
			UserPrompt:   UserPromptBaseTokens,
		},
	}
}

// EstimatePlanTokens calculates total token estimates for a test plan
func EstimatePlanTokens(plan *TestPlan, model *SystemModel) PlanTokenEstimate {
	var total PlanTokenEstimate

	for i := range plan.Intents {
		estimate := EstimateTokensForIntent(&plan.Intents[i], model)
		total.TotalInputTokens += estimate.InputTokens
		total.TotalOutputTokens += estimate.OutputTokens
		total.IntentEstimates = append(total.IntentEstimates, IntentTokenEstimate{
			IntentID: plan.Intents[i].ID,
			Estimate: estimate,
		})
	}

	total.TotalTokens = total.TotalInputTokens + total.TotalOutputTokens

	return total
}

// PlanTokenEstimate contains aggregated estimates for a test plan
type PlanTokenEstimate struct {
	TotalInputTokens  int                   `json:"total_input_tokens"`
	TotalOutputTokens int                   `json:"total_output_tokens"`
	TotalTokens       int                   `json:"total_tokens"`
	IntentEstimates   []IntentTokenEstimate `json:"intent_estimates,omitempty"`
}

// IntentTokenEstimate pairs an intent with its token estimate
type IntentTokenEstimate struct {
	IntentID string        `json:"intent_id"`
	Estimate TokenEstimate `json:"estimate"`
}

// estimateCodeTokens estimates tokens for code content
func estimateCodeTokens(code string) int {
	if code == "" {
		return 0
	}
	return len(code) / CharsPerToken
}

// estimateOutputTokens estimates expected output tokens
func estimateOutputTokens(codeTokens int, level TestLevel, complexity int) int {
	// Get multiplier based on test level
	var multiplier float64
	switch level {
	case LevelAPI:
		multiplier = APITestOutputMultiplier
	case LevelE2E:
		multiplier = E2ETestOutputMultiplier
	default:
		multiplier = UnitTestOutputMultiplier
	}

	// Adjust for complexity (higher complexity = more test cases = more output)
	if complexity > 10 {
		multiplier *= 1.2
	} else if complexity > 5 {
		multiplier *= 1.1
	}

	// Calculate base output tokens
	outputTokens := int(float64(codeTokens) * multiplier)

	// Apply bounds
	if outputTokens < MinOutputTokens {
		outputTokens = MinOutputTokens
	}
	if outputTokens > MaxOutputTokens {
		outputTokens = MaxOutputTokens
	}

	return outputTokens
}

// CostEstimate provides cost estimates based on token counts
// Prices are per 1M tokens (typical LLM pricing structure)
type CostEstimate struct {
	InputCost  float64 `json:"input_cost"`
	OutputCost float64 `json:"output_cost"`
	TotalCost  float64 `json:"total_cost"`
	Currency   string  `json:"currency"`
}

// EstimateCost calculates cost for a token estimate
// inputPricePerM and outputPricePerM are prices per 1 million tokens
func EstimateCost(estimate TokenEstimate, inputPricePerM, outputPricePerM float64) CostEstimate {
	inputCost := float64(estimate.InputTokens) / 1_000_000 * inputPricePerM
	outputCost := float64(estimate.OutputTokens) / 1_000_000 * outputPricePerM

	return CostEstimate{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Currency:   "USD",
	}
}

// EstimatePlanCost calculates cost for a full test plan
func EstimatePlanCost(planEstimate PlanTokenEstimate, inputPricePerM, outputPricePerM float64) CostEstimate {
	inputCost := float64(planEstimate.TotalInputTokens) / 1_000_000 * inputPricePerM
	outputCost := float64(planEstimate.TotalOutputTokens) / 1_000_000 * outputPricePerM

	return CostEstimate{
		InputCost:  inputCost,
		OutputCost: outputCost,
		TotalCost:  inputCost + outputCost,
		Currency:   "USD",
	}
}

// Common pricing constants (USD per 1M tokens)
const (
	// Ollama local models (free)
	OllamaInputPrice  = 0.0
	OllamaOutputPrice = 0.0

	// OpenAI GPT-4o (as of 2024)
	GPT4oInputPrice  = 5.0  // $5/1M input tokens
	GPT4oOutputPrice = 15.0 // $15/1M output tokens

	// OpenAI GPT-4o-mini
	GPT4oMiniInputPrice  = 0.15 // $0.15/1M input tokens
	GPT4oMiniOutputPrice = 0.60 // $0.60/1M output tokens

	// Anthropic Claude 3.5 Sonnet
	ClaudeSonnetInputPrice  = 3.0  // $3/1M input tokens
	ClaudeSonnetOutputPrice = 15.0 // $15/1M output tokens
)
