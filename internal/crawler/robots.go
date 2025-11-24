package crawler

import (
	"bufio"
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RobotsTxt represents a parsed robots.txt file.
type RobotsTxt struct {
	rules     []robotRule
	sitemaps  []string
	crawlDelay time.Duration
}

type robotRule struct {
	userAgent string
	allow     []string
	disallow  []string
}

// FetchRobotsTxt fetches and parses a robots.txt file.
func FetchRobotsTxt(ctx context.Context, robotsURL, userAgent string) (*RobotsTxt, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// No robots.txt or error - allow all
		return &RobotsTxt{}, nil
	}

	return ParseRobotsTxt(resp.Body)
}

// ParseRobotsTxt parses robots.txt content.
func ParseRobotsTxt(r interface{ Read([]byte) (int, error) }) (*RobotsTxt, error) {
	robots := &RobotsTxt{
		rules:    make([]robotRule, 0),
		sitemaps: make([]string, 0),
	}

	scanner := bufio.NewScanner(r)
	var currentRule *robotRule

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse directive
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		directive := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch directive {
		case "user-agent":
			// Start new rule group
			currentRule = &robotRule{
				userAgent: strings.ToLower(value),
				allow:     make([]string, 0),
				disallow:  make([]string, 0),
			}
			robots.rules = append(robots.rules, *currentRule)
			currentRule = &robots.rules[len(robots.rules)-1]

		case "allow":
			if currentRule != nil {
				currentRule.allow = append(currentRule.allow, value)
			}

		case "disallow":
			if currentRule != nil {
				currentRule.disallow = append(currentRule.disallow, value)
			}

		case "sitemap":
			robots.sitemaps = append(robots.sitemaps, value)

		case "crawl-delay":
			// Parse crawl delay (optional)
			// Not implemented for simplicity
		}
	}

	return robots, scanner.Err()
}

// IsAllowed checks if crawling the given URL is allowed.
func (r *RobotsTxt) IsAllowed(rawURL string) bool {
	if r == nil || len(r.rules) == 0 {
		return true
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}

	path := parsed.Path
	if path == "" {
		path = "/"
	}

	// Find matching rule (prefer specific user-agent, fallback to *)
	var applicableRule *robotRule
	for i := range r.rules {
		rule := &r.rules[i]
		if rule.userAgent == "*" || strings.Contains(strings.ToLower("qtest"), rule.userAgent) {
			applicableRule = rule
			// Don't break - later rules may be more specific
		}
	}

	if applicableRule == nil {
		return true
	}

	// Check allow rules first (they take precedence for matching paths)
	for _, pattern := range applicableRule.allow {
		if matchesPattern(path, pattern) {
			return true
		}
	}

	// Check disallow rules
	for _, pattern := range applicableRule.disallow {
		if pattern == "" {
			continue
		}
		if matchesPattern(path, pattern) {
			return false
		}
	}

	return true
}

// matchesPattern checks if a path matches a robots.txt pattern.
func matchesPattern(path, pattern string) bool {
	if pattern == "" {
		return false
	}

	// Handle end-of-string anchor
	hasEndAnchor := strings.HasSuffix(pattern, "$")
	if hasEndAnchor {
		pattern = strings.TrimSuffix(pattern, "$")
	}

	// Handle wildcards
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		idx := 0
		for i, part := range parts {
			if part == "" {
				continue
			}
			foundIdx := strings.Index(path[idx:], part)
			if foundIdx == -1 {
				return false
			}
			// First part must match at the beginning
			if i == 0 && foundIdx != 0 {
				return false
			}
			idx += foundIdx + len(part)
		}
		// If there's an end anchor, ensure we're at the end
		if hasEndAnchor && idx != len(path) {
			return false
		}
		return true
	}

	// With end anchor, must be exact match
	if hasEndAnchor {
		return path == pattern
	}

	// Simple prefix match
	return strings.HasPrefix(path, pattern)
}

// Sitemaps returns the sitemap URLs found in robots.txt.
func (r *RobotsTxt) Sitemaps() []string {
	if r == nil {
		return nil
	}
	return r.sitemaps
}
