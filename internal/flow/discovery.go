package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/QTest-hq/qtest/internal/llm"
	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
)

// Discovery uses LLM to discover and suggest user flows.
type Discovery struct {
	client    *playwright.Client
	llmRouter *llm.Router
	config    *FlowConfig
}

// NewDiscovery creates a new LLM-based flow discovery.
func NewDiscovery(client *playwright.Client, llmRouter *llm.Router, config *FlowConfig) *Discovery {
	if config == nil {
		config = DefaultFlowConfig()
	}
	return &Discovery{
		client:    client,
		llmRouter: llmRouter,
		config:    config,
	}
}

// DiscoverFlows uses LLM to analyze a page and suggest flows.
func (d *Discovery) DiscoverFlows(ctx context.Context, sessionID string) ([]Flow, error) {
	// Get page content for LLM analysis
	pageContent, err := d.getPageContext(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get page context: %w", err)
	}

	// Build LLM prompt
	prompt := d.buildDiscoveryPrompt(pageContent)

	// Call LLM
	response, err := d.llmRouter.Complete(ctx, &llm.Request{
		System:   flowDiscoverySystemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   2000,
		Tier:        llm.Tier2,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse LLM response
	flows, err := d.parseFlowSuggestions(response.Content, pageContent.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return flows, nil
}

// ExploreAndDiscover autonomously explores a site and discovers flows.
func (d *Discovery) ExploreAndDiscover(ctx context.Context, startURL string, maxPages int) ([]Flow, error) {
	// Create session
	sessionID, err := d.client.CreateSession(ctx, playwright.SessionConfig{
		BrowserType:    "chromium",
		Headless:       true,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		TimeoutMs:      int32(d.config.PageLoadTimeout.Milliseconds()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer d.client.DestroySession(ctx, sessionID)

	// Navigate to start URL
	_, err = d.client.Navigate(ctx, sessionID, startURL, &playwright.NavigateOptions{
		TimeoutMs: int32(d.config.PageLoadTimeout.Milliseconds()),
		WaitUntil: "networkidle",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	allFlows := make([]Flow, 0)
	visited := make(map[string]bool)
	queue := []string{startURL}

	for len(queue) > 0 && len(visited) < maxPages {
		currentURL := queue[0]
		queue = queue[1:]

		if visited[currentURL] {
			continue
		}
		visited[currentURL] = true

		// Navigate if not already there
		status, _ := d.client.GetSessionStatus(ctx, sessionID)
		if status == nil || status.CurrentURL != currentURL {
			d.client.Navigate(ctx, sessionID, currentURL, &playwright.NavigateOptions{
				TimeoutMs: int32(d.config.PageLoadTimeout.Milliseconds()),
				WaitUntil: "networkidle",
			})
		}

		// Discover flows on this page
		flows, err := d.DiscoverFlows(ctx, sessionID)
		if err == nil {
			allFlows = append(allFlows, flows...)
		}

		// Get links for further exploration
		links, _ := d.client.GetPageLinks(ctx, sessionID, true)
		for _, link := range links {
			if !visited[link.Href] {
				queue = append(queue, link.Href)
			}
		}
	}

	// Deduplicate and merge similar flows
	return d.deduplicateFlows(allFlows), nil
}

// SuggestNextActions uses LLM to suggest what actions to take next.
func (d *Discovery) SuggestNextActions(ctx context.Context, sessionID string, currentFlow *Flow) ([]Action, error) {
	pageContent, err := d.getPageContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	prompt := d.buildNextActionsPrompt(pageContent, currentFlow)

	response, err := d.llmRouter.Complete(ctx, &llm.Request{
		System: actionSuggestionSystemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   1000,
		Tier:        llm.Tier2,
	})
	if err != nil {
		return nil, err
	}

	return d.parseActionSuggestions(response.Content)
}

// PageContext holds context about a page for LLM analysis.
type PageContext struct {
	URL         string
	Title       string
	Forms       []FormSummary
	Links       []LinkSummary
	Buttons     []string
	Headings    []string
	DOMSummary  string
}

type FormSummary struct {
	Action string
	Method string
	Fields []string
}

type LinkSummary struct {
	Text string
	Href string
}

func (d *Discovery) getPageContext(ctx context.Context, sessionID string) (*PageContext, error) {
	status, err := d.client.GetSessionStatus(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	context := &PageContext{
		URL:   status.CurrentURL,
		Title: status.PageTitle,
	}

	// Get forms
	forms, _ := d.client.GetPageForms(ctx, sessionID)
	for _, form := range forms {
		summary := FormSummary{
			Action: form.Action,
			Method: form.Method,
		}
		for _, field := range form.Fields {
			summary.Fields = append(summary.Fields, fmt.Sprintf("%s (%s)", field.Name, field.Type))
		}
		context.Forms = append(context.Forms, summary)
	}

	// Get links
	links, _ := d.client.GetPageLinks(ctx, sessionID, true)
	for _, link := range links {
		context.Links = append(context.Links, LinkSummary{
			Text: link.Text,
			Href: link.Href,
		})
	}

	// Get DOM summary
	_, html, _ := d.client.GetDOMSnapshot(ctx, sessionID, "body", 3)
	if html != "" {
		context.DOMSummary = truncateString(html, 2000)
	}

	return context, nil
}

func (d *Discovery) buildDiscoveryPrompt(ctx *PageContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Analyze this web page and identify testable user flows:\n\n"))
	sb.WriteString(fmt.Sprintf("URL: %s\n", ctx.URL))
	sb.WriteString(fmt.Sprintf("Title: %s\n\n", ctx.Title))

	if len(ctx.Forms) > 0 {
		sb.WriteString("Forms found:\n")
		for i, form := range ctx.Forms {
			sb.WriteString(fmt.Sprintf("  Form %d: action=%s, method=%s\n", i+1, form.Action, form.Method))
			sb.WriteString(fmt.Sprintf("    Fields: %s\n", strings.Join(form.Fields, ", ")))
		}
		sb.WriteString("\n")
	}

	if len(ctx.Links) > 0 {
		sb.WriteString("Navigation links (first 20):\n")
		for i, link := range ctx.Links {
			if i >= 20 {
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s -> %s\n", link.Text, link.Href))
		}
		sb.WriteString("\n")
	}

	if ctx.DOMSummary != "" {
		sb.WriteString("Page structure summary:\n")
		sb.WriteString(ctx.DOMSummary)
		sb.WriteString("\n")
	}

	sb.WriteString("\nIdentify user flows that should be tested. For each flow, provide:\n")
	sb.WriteString("1. Flow name and type (login, search, checkout, navigation, etc.)\n")
	sb.WriteString("2. Step-by-step actions with selectors\n")
	sb.WriteString("3. Expected assertions\n\n")
	sb.WriteString("Return as JSON array of flows.")

	return sb.String()
}

func (d *Discovery) buildNextActionsPrompt(ctx *PageContext, flow *Flow) string {
	var sb strings.Builder

	sb.WriteString("Current page context:\n")
	sb.WriteString(fmt.Sprintf("URL: %s\n", ctx.URL))
	sb.WriteString(fmt.Sprintf("Title: %s\n\n", ctx.Title))

	if flow != nil && len(flow.Steps) > 0 {
		sb.WriteString("Actions taken so far:\n")
		for _, step := range flow.Steps {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", step.Action.Type, step.Action.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("What actions should be taken next to complete this flow?\n")
	sb.WriteString("Return as JSON array of actions with type, selector, and value.")

	return sb.String()
}

func (d *Discovery) parseFlowSuggestions(response string, baseURL string) ([]Flow, error) {
	// Try to extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var suggestions []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Steps       []struct {
			Action      string `json:"action"`
			Selector    string `json:"selector"`
			Value       string `json:"value,omitempty"`
			Description string `json:"description"`
		} `json:"steps"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &suggestions); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	flows := make([]Flow, 0, len(suggestions))
	for _, s := range suggestions {
		flow := Flow{
			ID:          uuid.New().String(),
			Name:        s.Name,
			Type:        mapFlowType(s.Type),
			Description: s.Description,
			StartURL:    baseURL,
			Steps:       make([]Step, 0, len(s.Steps)),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		for i, step := range s.Steps {
			action := Action{
				ID:          uuid.New().String(),
				Type:        mapActionType(step.Action),
				Description: step.Description,
				Timestamp:   time.Now(),
			}

			if step.Selector != "" {
				action.Selector = &Selector{
					Primary:    step.Selector,
					Strategy:   SelectorCSS,
					Confidence: 0.6,
				}
			}

			if step.Value != "" {
				action.Value = step.Value
			}

			flow.Steps = append(flow.Steps, Step{
				ID:     uuid.New().String(),
				Name:   step.Description,
				Action: action,
				Order:  i + 1,
			})
		}

		flows = append(flows, flow)
	}

	return flows, nil
}

func (d *Discovery) parseActionSuggestions(response string) ([]Action, error) {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var suggestions []struct {
		Type     string `json:"type"`
		Selector string `json:"selector"`
		Value    string `json:"value,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &suggestions); err != nil {
		return nil, err
	}

	actions := make([]Action, 0, len(suggestions))
	for _, s := range suggestions {
		action := Action{
			ID:        uuid.New().String(),
			Type:      mapActionType(s.Type),
			Value:     s.Value,
			Timestamp: time.Now(),
		}

		if s.Selector != "" {
			action.Selector = &Selector{
				Primary:  s.Selector,
				Strategy: SelectorCSS,
			}
		}

		actions = append(actions, action)
	}

	return actions, nil
}

func (d *Discovery) deduplicateFlows(flows []Flow) []Flow {
	seen := make(map[string]bool)
	unique := make([]Flow, 0)

	for _, flow := range flows {
		key := fmt.Sprintf("%s-%s-%d", flow.Type, flow.StartURL, len(flow.Steps))
		if !seen[key] {
			seen[key] = true
			unique = append(unique, flow)
		}
	}

	return unique
}

// Helper functions

func mapFlowType(s string) FlowType {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "login"):
		return FlowTypeLogin
	case strings.Contains(s, "register"), strings.Contains(s, "registration"), strings.Contains(s, "signup"):
		return FlowTypeRegistration
	case strings.Contains(s, "checkout"), strings.Contains(s, "payment"):
		return FlowTypeCheckout
	case strings.Contains(s, "search"):
		return FlowTypeSearch
	case strings.Contains(s, "form"):
		return FlowTypeFormSubmit
	case strings.Contains(s, "nav"):
		return FlowTypeNavigation
	case strings.Contains(s, "crud"):
		return FlowTypeCRUD
	default:
		return FlowTypeCustom
	}
}

func mapActionType(s string) ActionType {
	s = strings.ToLower(s)
	switch {
	case strings.Contains(s, "click"):
		return ActionTypeClick
	case strings.Contains(s, "fill"), strings.Contains(s, "type"), strings.Contains(s, "input"):
		return ActionTypeFill
	case strings.Contains(s, "select"):
		return ActionTypeSelect
	case strings.Contains(s, "check"):
		return ActionTypeCheck
	case strings.Contains(s, "hover"):
		return ActionTypeHover
	case strings.Contains(s, "scroll"):
		return ActionTypeScroll
	case strings.Contains(s, "key"), strings.Contains(s, "press"):
		return ActionTypeKeypress
	case strings.Contains(s, "nav"), strings.Contains(s, "goto"):
		return ActionTypeNavigate
	case strings.Contains(s, "wait"):
		return ActionTypeWait
	case strings.Contains(s, "assert"):
		return ActionTypeAssert
	default:
		return ActionTypeClick
	}
}

func extractJSON(s string) string {
	// Find JSON array or object in the response
	start := strings.Index(s, "[")
	if start == -1 {
		start = strings.Index(s, "{")
	}
	if start == -1 {
		return ""
	}

	// Find matching bracket
	openBracket := s[start]
	closeBracket := byte(']')
	if openBracket == '{' {
		closeBracket = '}'
	}

	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == openBracket {
			depth++
		} else if s[i] == closeBracket {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// System prompts for LLM

const flowDiscoverySystemPrompt = `You are an expert QA engineer specializing in E2E test automation.
Your task is to analyze web pages and identify testable user flows.

For each flow you identify:
1. Give it a descriptive name
2. Classify the type (login, registration, search, checkout, navigation, crud, form_submit)
3. List the specific steps with CSS selectors
4. Include expected outcomes

Focus on critical user journeys that should be tested.
Return your analysis as a valid JSON array.`

const actionSuggestionSystemPrompt = `You are an expert QA engineer helping to complete E2E test flows.
Given the current page state and actions already taken, suggest the next logical actions.

Provide specific CSS selectors for elements.
Consider form validation, success/error states, and navigation.
Return your suggestions as a valid JSON array of actions.`
