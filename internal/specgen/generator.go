// Package specgen generates test specifications from test intents using LLM
package specgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/pkg/model"
)

// Generator generates test specifications from intents
type Generator struct {
	router *llm.Router
	tier   llm.Tier
}

// NewGenerator creates a new spec generator
func NewGenerator(router *llm.Router, tier llm.Tier) *Generator {
	return &Generator{
		router: router,
		tier:   tier,
	}
}

// GenerateSpec generates a test spec for a single intent
func (g *Generator) GenerateSpec(ctx context.Context, intent model.TestIntent, sysModel *model.SystemModel) (*model.TestSpec, error) {
	// Build the context for this intent
	fragment := g.buildModelFragment(intent, sysModel)

	// Create prompt
	prompt := g.buildPrompt(intent, fragment)

	// Call LLM
	resp, err := g.router.Complete(ctx, &llm.Request{
		Tier:   g.tier,
		System: systemPromptSpecGen,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2, // Low temperature for structured output
		MaxTokens:   2000,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse response
	spec, err := g.parseSpecResponse(resp.Content, intent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}

	return spec, nil
}

// GenerateSpecs generates specs for multiple intents
func (g *Generator) GenerateSpecs(ctx context.Context, plan *model.TestPlan, sysModel *model.SystemModel) (*model.TestSpecSet, error) {
	specSet := &model.TestSpecSet{
		ModelID:    plan.ModelID,
		Repository: plan.Repository,
		Specs:      make([]model.TestSpec, 0),
	}

	for _, intent := range plan.Intents {
		spec, err := g.GenerateSpec(ctx, intent, sysModel)
		if err != nil {
			// Log error but continue with other intents
			continue
		}
		specSet.Specs = append(specSet.Specs, *spec)
	}

	return specSet, nil
}

// buildModelFragment extracts relevant parts of the model for an intent
func (g *Generator) buildModelFragment(intent model.TestIntent, sysModel *model.SystemModel) map[string]interface{} {
	fragment := make(map[string]interface{})

	switch intent.TargetKind {
	case "endpoint":
		// Find the endpoint
		for _, ep := range sysModel.Endpoints {
			if ep.ID == intent.TargetID {
				fragment["endpoint"] = ep

				// Find the handler function if available
				for _, fn := range sysModel.Functions {
					if fn.Name == ep.Handler {
						fragment["handler"] = fn
						break
					}
				}

				// Find related types (request/response bodies)
				if ep.RequestBody != "" {
					for _, t := range sysModel.Types {
						if t.Name == ep.RequestBody {
							fragment["request_type"] = t
							break
						}
					}
				}
				if ep.ResponseBody != "" {
					for _, t := range sysModel.Types {
						if t.Name == ep.ResponseBody {
							fragment["response_type"] = t
							break
						}
					}
				}
				break
			}
		}

	case "function":
		// Find the function
		for _, fn := range sysModel.Functions {
			if fn.ID == intent.TargetID {
				fragment["function"] = fn

				// Find related types in parameters/returns
				for _, param := range fn.Parameters {
					if param.Type != "" {
						for _, t := range sysModel.Types {
							if t.Name == param.Type {
								if fragment["types"] == nil {
									fragment["types"] = []model.TypeDef{}
								}
								fragment["types"] = append(fragment["types"].([]model.TypeDef), t)
							}
						}
					}
				}
				break
			}
		}
	}

	return fragment
}

// buildPrompt creates the LLM prompt for spec generation
func (g *Generator) buildPrompt(intent model.TestIntent, fragment map[string]interface{}) string {
	fragmentJSON, _ := json.MarshalIndent(fragment, "", "  ")
	intentJSON, _ := json.MarshalIndent(intent, "", "  ")

	var sb strings.Builder

	sb.WriteString("Generate a test specification for the following target.\n\n")

	sb.WriteString("## System Model Fragment\n```json\n")
	sb.WriteString(string(fragmentJSON))
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Test Intent\n```json\n")
	sb.WriteString(string(intentJSON))
	sb.WriteString("\n```\n\n")

	if intent.Level == model.LevelAPI {
		sb.WriteString(apiTestGuidance)
	} else {
		sb.WriteString(unitTestGuidance)
	}

	sb.WriteString("\n\nOutput ONLY valid JSON matching the TestSpec schema. No explanation.")

	return sb.String()
}

// parseSpecResponse parses LLM response into TestSpec
func (g *Generator) parseSpecResponse(response string, intent model.TestIntent) (*model.TestSpec, error) {
	// Clean up response - extract JSON if wrapped in markdown
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}
	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}
	response = strings.TrimSpace(response)

	var spec model.TestSpec
	if err := json.Unmarshal([]byte(response), &spec); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nResponse: %s", err, response)
	}

	// Ensure required fields are set
	if spec.ID == "" {
		spec.ID = intent.ID
	}
	if spec.Level == "" {
		spec.Level = intent.Level
	}
	if spec.TargetKind == "" {
		spec.TargetKind = intent.TargetKind
	}
	if spec.TargetID == "" {
		spec.TargetID = intent.TargetID
	}
	if spec.Priority == "" {
		spec.Priority = intent.Priority
	}

	return &spec, nil
}

const systemPromptSpecGen = `You are an expert test engineer. Your task is to generate test specifications in JSON format.

IMPORTANT:
- Output ONLY valid JSON, no explanation or markdown
- Use realistic test values (not edge cases like MAX_INT)
- Include meaningful assertions that verify behavior
- For API tests, always check status code and response structure
- For unit tests, verify return values and side effects

JSON Schema for TestSpec:
{
  "id": "string",
  "level": "unit" | "api" | "e2e",
  "target_kind": "function" | "endpoint",
  "target_id": "string",
  "description": "string - what this test verifies",

  // For function tests:
  "function_name": "string",
  "inputs": { "arg1": value, "arg2": value },

  // For API tests:
  "method": "GET" | "POST" | "PUT" | "DELETE",
  "path": "/path/to/resource",
  "path_params": { "id": "123" },
  "query_params": { "limit": 10 },
  "headers": { "Authorization": "Bearer token" },
  "body": { "field": "value" },

  // Expected outcomes:
  "expected": {
    "status": 200,
    "body": { ... }
  },
  "assertions": [
    {
      "kind": "equality" | "contains" | "not_null" | "status_code",
      "actual": "result" | "status" | "body.field",
      "expected": value
    }
  ],

  "priority": "high" | "medium" | "low",
  "tags": ["tag1", "tag2"]
}`

const apiTestGuidance = `## API Test Guidelines
- Test the happy path first (valid request → successful response)
- Check status code matches expected (200, 201, 404, etc.)
- Verify response body structure
- For endpoints with path params, use realistic values
- For POST/PUT, include a valid request body
- Include assertions for:
  - Status code
  - Response body key fields
  - Error handling (if testing error cases)`

const unitTestGuidance = `## Unit Test Guidelines
- Test with typical inputs first
- Include edge cases (empty, zero, negative if applicable)
- Use realistic values within normal ranges
- Include assertions for:
  - Return value
  - Expected behavior based on function signature`
