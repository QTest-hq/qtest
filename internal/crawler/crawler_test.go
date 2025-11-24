package crawler

import (
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes fragment",
			input:    "https://example.com/page#section",
			expected: "https://example.com/page",
		},
		{
			name:     "removes trailing slash",
			input:    "https://example.com/page/",
			expected: "https://example.com/page",
		},
		{
			name:     "preserves root path",
			input:    "https://example.com/",
			expected: "https://example.com/",
		},
		{
			name:     "adds root path",
			input:    "https://example.com",
			expected: "https://example.com/",
		},
		{
			name:     "preserves query params",
			input:    "https://example.com/page?foo=bar",
			expected: "https://example.com/page?foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d, want 3", config.MaxDepth)
	}
	if config.MaxPages != 100 {
		t.Errorf("MaxPages = %d, want 100", config.MaxPages)
	}
	if !config.RespectRobotsTxt {
		t.Error("RespectRobotsTxt should be true")
	}
	if !config.SameOriginOnly {
		t.Error("SameOriginOnly should be true")
	}
	if !config.CaptureNetwork {
		t.Error("CaptureNetwork should be true")
	}
	if !config.Headless {
		t.Error("Headless should be true")
	}
}

func TestRobotsTxtParse(t *testing.T) {
	robotsContent := `
User-agent: *
Disallow: /private/
Disallow: /admin/
Allow: /public/

User-agent: Googlebot
Disallow: /no-google/

Sitemap: https://example.com/sitemap.xml
`

	robots, err := ParseRobotsTxt(strings.NewReader(robotsContent))
	if err != nil {
		t.Fatalf("ParseRobotsTxt error: %v", err)
	}

	if len(robots.rules) != 2 {
		t.Errorf("got %d rules, want 2", len(robots.rules))
	}

	if len(robots.sitemaps) != 1 {
		t.Errorf("got %d sitemaps, want 1", len(robots.sitemaps))
	}

	if robots.sitemaps[0] != "https://example.com/sitemap.xml" {
		t.Errorf("sitemap = %q, want https://example.com/sitemap.xml", robots.sitemaps[0])
	}
}

func TestRobotsTxtIsAllowed(t *testing.T) {
	robotsContent := `
User-agent: *
Disallow: /private/
Disallow: /admin/
Allow: /admin/public/
`

	robots, err := ParseRobotsTxt(strings.NewReader(robotsContent))
	if err != nil {
		t.Fatalf("ParseRobotsTxt error: %v", err)
	}

	tests := []struct {
		url      string
		expected bool
	}{
		{"https://example.com/", true},
		{"https://example.com/public/page", true},
		{"https://example.com/private/secret", false},
		{"https://example.com/admin/dashboard", false},
		{"https://example.com/admin/public/page", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := robots.IsAllowed(tt.url)
			if result != tt.expected {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestRobotsTxtNil(t *testing.T) {
	var robots *RobotsTxt

	// Should allow everything when nil
	if !robots.IsAllowed("https://example.com/anything") {
		t.Error("nil RobotsTxt should allow all URLs")
	}
}

func TestRobotsTxtWildcard(t *testing.T) {
	robotsContent := `
User-agent: *
Disallow: /api/*
Disallow: /temp/
`

	robots, err := ParseRobotsTxt(strings.NewReader(robotsContent))
	if err != nil {
		t.Fatalf("ParseRobotsTxt error: %v", err)
	}

	tests := []struct {
		url      string
		expected bool
	}{
		{"https://example.com/data.json", true},
		{"https://example.com/data.txt", true},
		{"https://example.com/api/users", false},
		{"https://example.com/api/", false},
		{"https://example.com/public/api", true},
		{"https://example.com/temp/file", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := robots.IsAllowed(tt.url)
			if result != tt.expected {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		path     string
		pattern  string
		expected bool
	}{
		{"/admin/", "/admin/", true},
		{"/admin/page", "/admin/", true},
		{"/public/admin", "/admin/", false},
		{"/api/v1/users", "/api/*", true},
		{"/api/", "/api/*", true},
		{"/page.html", "/page.html$", true},
		{"/page.html/extra", "/page.html$", false},
		{"", "", false},
	}

	for _, tt := range tests {
		name := tt.path + " ~ " + tt.pattern
		t.Run(name, func(t *testing.T) {
			result := matchesPattern(tt.path, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestCrawlerNew(t *testing.T) {
	// Test with nil config
	crawler := New(nil, nil)
	if crawler.config == nil {
		t.Error("config should not be nil")
	}
	if crawler.config.MaxDepth != 3 {
		t.Errorf("MaxDepth = %d, want 3", crawler.config.MaxDepth)
	}

	// Test with custom config
	customConfig := &Config{
		MaxDepth: 5,
		MaxPages: 50,
	}
	crawler2 := New(nil, customConfig)
	if crawler2.config.MaxDepth != 5 {
		t.Errorf("MaxDepth = %d, want 5", crawler2.config.MaxDepth)
	}
}
