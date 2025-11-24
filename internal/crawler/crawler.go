// Package crawler provides website crawling capabilities using the Playwright sidecar.
package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QTest-hq/qtest/internal/sidecar/playwright"
)

// Config contains crawler configuration options.
type Config struct {
	// MaxDepth is the maximum depth to crawl (0 = unlimited)
	MaxDepth int
	// MaxPages is the maximum number of pages to crawl (0 = unlimited)
	MaxPages int
	// Timeout is the timeout for each page load
	Timeout time.Duration
	// RespectRobotsTxt determines whether to respect robots.txt
	RespectRobotsTxt bool
	// UserAgent is the user agent to use
	UserAgent string
	// SameOriginOnly limits crawling to the same origin
	SameOriginOnly bool
	// CaptureNetwork enables network request capture
	CaptureNetwork bool
	// CaptureScreenshots enables screenshot capture
	CaptureScreenshots bool
	// Headless runs browser in headless mode
	Headless bool
}

// DefaultConfig returns a default crawler configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxDepth:           3,
		MaxPages:           100,
		Timeout:            30 * time.Second,
		RespectRobotsTxt:   true,
		UserAgent:          "QTest-Crawler/1.0",
		SameOriginOnly:     true,
		CaptureNetwork:     true,
		CaptureScreenshots: false,
		Headless:           true,
	}
}

// PageResult contains the result of crawling a single page.
type PageResult struct {
	URL            string
	Title          string
	StatusCode     int32
	Links          []*playwright.PageLink
	Forms          []*playwright.PageForm
	NetworkLog     []playwright.CapturedNetworkRequest
	Screenshot     []byte
	CrawledAt      time.Time
	Depth          int
	Error          string
	LoadTimeMs     int64
}

// CrawlResult contains the complete result of a crawl session.
type CrawlResult struct {
	StartURL      string
	Pages         []*PageResult
	StartedAt     time.Time
	FinishedAt    time.Time
	TotalPages    int
	SuccessPages  int
	FailedPages   int
	RobotsTxtURL  string
	DisallowedURLs []string
}

// Crawler orchestrates website crawling using Playwright.
type Crawler struct {
	client   *playwright.Client
	config   *Config
	visited  map[string]bool
	queue    []queueItem
	mu       sync.Mutex
	robotsTxt *RobotsTxt
}

type queueItem struct {
	url   string
	depth int
}

// New creates a new Crawler instance.
func New(client *playwright.Client, config *Config) *Crawler {
	if config == nil {
		config = DefaultConfig()
	}
	return &Crawler{
		client:  client,
		config:  config,
		visited: make(map[string]bool),
		queue:   make([]queueItem, 0),
	}
}

// Crawl starts crawling from the given URL.
func (c *Crawler) Crawl(ctx context.Context, startURL string) (*CrawlResult, error) {
	result := &CrawlResult{
		StartURL:  startURL,
		StartedAt: time.Now(),
		Pages:     make([]*PageResult, 0),
	}

	// Parse start URL
	parsedURL, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("invalid start URL: %w", err)
	}

	// Fetch robots.txt if configured
	if c.config.RespectRobotsTxt {
		robotsURL := fmt.Sprintf("%s://%s/robots.txt", parsedURL.Scheme, parsedURL.Host)
		result.RobotsTxtURL = robotsURL
		c.robotsTxt, _ = FetchRobotsTxt(ctx, robotsURL, c.config.UserAgent)
	}

	// Create browser session
	sessionID, err := c.client.CreateSession(ctx, playwright.SessionConfig{
		BrowserType:    "chromium",
		Headless:       c.config.Headless,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		UserAgent:      c.config.UserAgent,
		TimeoutMs:      int32(c.config.Timeout.Milliseconds()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create browser session: %w", err)
	}
	defer c.client.DestroySession(ctx, sessionID)

	// Start network capture if configured
	if c.config.CaptureNetwork {
		c.client.StartNetworkCapture(ctx, sessionID, playwright.NetworkCaptureConfig{
			CaptureRequestBody:  true,
			CaptureResponseBody: true,
		})
	}

	// Initialize queue with start URL
	c.queue = append(c.queue, queueItem{url: startURL, depth: 0})
	c.visited[normalizeURL(startURL)] = true

	// Process queue
	for len(c.queue) > 0 && (c.config.MaxPages == 0 || len(result.Pages) < c.config.MaxPages) {
		select {
		case <-ctx.Done():
			result.FinishedAt = time.Now()
			return result, ctx.Err()
		default:
		}

		// Dequeue
		c.mu.Lock()
		item := c.queue[0]
		c.queue = c.queue[1:]
		c.mu.Unlock()

		// Check depth limit
		if c.config.MaxDepth > 0 && item.depth > c.config.MaxDepth {
			continue
		}

		// Check robots.txt
		if c.robotsTxt != nil && !c.robotsTxt.IsAllowed(item.url) {
			result.DisallowedURLs = append(result.DisallowedURLs, item.url)
			continue
		}

		// Crawl page
		pageResult := c.crawlPage(ctx, sessionID, item.url, item.depth, parsedURL)
		result.Pages = append(result.Pages, pageResult)

		if pageResult.Error != "" {
			result.FailedPages++
		} else {
			result.SuccessPages++

			// Add discovered links to queue
			for _, link := range pageResult.Links {
				c.maybeEnqueue(link.Href, item.depth+1, parsedURL)
			}
		}
	}

	// Get captured network requests
	if c.config.CaptureNetwork {
		c.client.StopNetworkCapture(ctx, sessionID)
	}

	result.FinishedAt = time.Now()
	result.TotalPages = len(result.Pages)
	return result, nil
}

func (c *Crawler) crawlPage(ctx context.Context, sessionID, pageURL string, depth int, baseURL *url.URL) *PageResult {
	result := &PageResult{
		URL:       pageURL,
		CrawledAt: time.Now(),
		Depth:     depth,
	}

	startTime := time.Now()

	// Navigate to page
	navResult, err := c.client.Navigate(ctx, sessionID, pageURL, &playwright.NavigateOptions{
		TimeoutMs: int32(c.config.Timeout.Milliseconds()),
		WaitUntil: "networkidle",
	})
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.LoadTimeMs = time.Since(startTime).Milliseconds()
	result.StatusCode = navResult.StatusCode

	// Get page status
	status, err := c.client.GetSessionStatus(ctx, sessionID)
	if err == nil && status != nil {
		result.Title = status.PageTitle
	}

	// Get links
	links, err := c.client.GetPageLinks(ctx, sessionID, c.config.SameOriginOnly)
	if err == nil {
		result.Links = links
	}

	// Get forms
	forms, err := c.client.GetPageForms(ctx, sessionID)
	if err == nil {
		result.Forms = forms
	}

	// Get network log
	if c.config.CaptureNetwork {
		requests, err := c.client.GetCapturedRequests(ctx, sessionID, true)
		if err == nil {
			result.NetworkLog = requests
		}
	}

	// Take screenshot
	if c.config.CaptureScreenshots {
		screenshot, err := c.client.TakeScreenshot(ctx, sessionID, false, "png")
		if err == nil {
			result.Screenshot = screenshot
		}
	}

	return result
}

func (c *Crawler) maybeEnqueue(linkURL string, depth int, baseURL *url.URL) {
	// Parse link URL
	parsedLink, err := url.Parse(linkURL)
	if err != nil {
		return
	}

	// Resolve relative URLs
	if !parsedLink.IsAbs() {
		parsedLink = baseURL.ResolveReference(parsedLink)
	}

	// Check same origin
	if c.config.SameOriginOnly && parsedLink.Host != baseURL.Host {
		return
	}

	// Skip non-HTTP URLs
	if parsedLink.Scheme != "http" && parsedLink.Scheme != "https" {
		return
	}

	// Skip already visited
	normalized := normalizeURL(parsedLink.String())
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.visited[normalized] {
		return
	}

	// Skip common non-page resources
	path := strings.ToLower(parsedLink.Path)
	skipExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".css", ".js", ".ico", ".pdf", ".zip", ".tar", ".gz"}
	for _, ext := range skipExtensions {
		if strings.HasSuffix(path, ext) {
			return
		}
	}

	c.visited[normalized] = true
	c.queue = append(c.queue, queueItem{url: parsedLink.String(), depth: depth})
}

func normalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Remove fragment
	parsed.Fragment = ""

	// Remove trailing slash for consistency
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = "/"
	}

	return parsed.String()
}
