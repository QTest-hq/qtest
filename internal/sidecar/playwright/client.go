// Package playwright provides a gRPC client for the Playwright sidecar service.
package playwright

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client provides methods to interact with the Playwright sidecar service.
type Client struct {
	conn   *grpc.ClientConn
	client PlaywrightServiceClient
}

// NewClient creates a new Playwright sidecar client.
func NewClient(address string) (*Client, error) {
	conn, err := grpc.Dial(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(100*1024*1024)), // 100MB for screenshots
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to playwright sidecar: %w", err)
	}

	return &Client{
		conn:   conn,
		client: NewPlaywrightServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// SessionConfig contains configuration for creating a browser session.
type SessionConfig struct {
	BrowserType       string            // chromium, firefox, webkit
	Headless          bool
	ViewportWidth     int32
	ViewportHeight    int32
	UserAgent         string
	ExtraHTTPHeaders  map[string]string
	IgnoreHTTPSErrors bool
	ProxyServer       string
	TimeoutMs         int32
}

// CreateSession creates a new browser session.
func (c *Client) CreateSession(ctx context.Context, config SessionConfig) (string, error) {
	resp, err := c.client.CreateSession(ctx, &CreateSessionRequest{
		BrowserType:       config.BrowserType,
		Headless:          config.Headless,
		ViewportWidth:     config.ViewportWidth,
		ViewportHeight:    config.ViewportHeight,
		UserAgent:         config.UserAgent,
		ExtraHttpHeaders:  config.ExtraHTTPHeaders,
		IgnoreHttpsErrors: config.IgnoreHTTPSErrors,
		ProxyServer:       config.ProxyServer,
		TimeoutMs:         config.TimeoutMs,
	})
	if err != nil {
		return "", fmt.Errorf("CreateSession RPC failed: %w", err)
	}
	if !resp.Success {
		return "", fmt.Errorf("CreateSession failed: %s", resp.Error)
	}
	return resp.SessionId, nil
}

// DestroySession destroys a browser session.
func (c *Client) DestroySession(ctx context.Context, sessionID string) error {
	resp, err := c.client.DestroySession(ctx, &DestroySessionRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return fmt.Errorf("DestroySession RPC failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("DestroySession failed: %s", resp.Error)
	}
	return nil
}

// SessionStatus contains status information about a session.
type SessionStatus struct {
	Exists       bool
	CurrentURL   string
	PageTitle    string
	CreatedAt    time.Time
	LastActivity time.Time
}

// GetSessionStatus returns the status of a session.
func (c *Client) GetSessionStatus(ctx context.Context, sessionID string) (*SessionStatus, error) {
	resp, err := c.client.GetSessionStatus(ctx, &GetSessionStatusRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetSessionStatus RPC failed: %w", err)
	}
	return &SessionStatus{
		Exists:       resp.Exists,
		CurrentURL:   resp.CurrentUrl,
		PageTitle:    resp.PageTitle,
		CreatedAt:    time.UnixMilli(resp.CreatedAt),
		LastActivity: time.UnixMilli(resp.LastActivity),
	}, nil
}

// NavigateOptions contains options for navigation.
type NavigateOptions struct {
	TimeoutMs int32
	WaitUntil string // load, domcontentloaded, networkidle
}

// NavigateResult contains the result of navigation.
type NavigateResult struct {
	FinalURL   string
	StatusCode int32
}

// Navigate navigates to a URL.
func (c *Client) Navigate(ctx context.Context, sessionID, url string, opts *NavigateOptions) (*NavigateResult, error) {
	req := &NavigateRequest{
		SessionId: sessionID,
		Url:       url,
	}
	if opts != nil {
		req.TimeoutMs = opts.TimeoutMs
		req.WaitUntil = opts.WaitUntil
	}

	resp, err := c.client.Navigate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Navigate RPC failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("Navigate failed: %s", resp.Error)
	}
	return &NavigateResult{
		FinalURL:   resp.FinalUrl,
		StatusCode: resp.StatusCode,
	}, nil
}

// Click clicks on an element.
func (c *Client) Click(ctx context.Context, sessionID, selector string, timeoutMs int32) error {
	resp, err := c.client.Click(ctx, &ClickRequest{
		SessionId: sessionID,
		Selector:  selector,
		TimeoutMs: timeoutMs,
	})
	if err != nil {
		return fmt.Errorf("Click RPC failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("Click failed: %s", resp.Error)
	}
	return nil
}

// Fill fills an input field.
func (c *Client) Fill(ctx context.Context, sessionID, selector, value string, timeoutMs int32) error {
	resp, err := c.client.Fill(ctx, &FillRequest{
		SessionId: sessionID,
		Selector:  selector,
		Value:     value,
		TimeoutMs: timeoutMs,
	})
	if err != nil {
		return fmt.Errorf("Fill RPC failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("Fill failed: %s", resp.Error)
	}
	return nil
}

// WaitForSelector waits for a selector to appear.
func (c *Client) WaitForSelector(ctx context.Context, sessionID, selector string, timeoutMs int32, state string) error {
	resp, err := c.client.WaitForSelector(ctx, &WaitForSelectorRequest{
		SessionId: sessionID,
		Selector:  selector,
		TimeoutMs: timeoutMs,
		State:     state,
	})
	if err != nil {
		return fmt.Errorf("WaitForSelector RPC failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("WaitForSelector failed: %s", resp.Error)
	}
	return nil
}

// TakeScreenshot takes a screenshot of the page.
func (c *Client) TakeScreenshot(ctx context.Context, sessionID string, fullPage bool, format string) ([]byte, error) {
	resp, err := c.client.TakeScreenshot(ctx, &TakeScreenshotRequest{
		SessionId: sessionID,
		FullPage:  fullPage,
		Format:    format,
	})
	if err != nil {
		return nil, fmt.Errorf("TakeScreenshot RPC failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("TakeScreenshot failed: %s", resp.Error)
	}
	return resp.ImageData, nil
}

// NetworkCaptureConfig contains options for network capture.
type NetworkCaptureConfig struct {
	URLPatterns         []string
	CaptureRequestBody  bool
	CaptureResponseBody bool
}

// StartNetworkCapture starts capturing network requests.
func (c *Client) StartNetworkCapture(ctx context.Context, sessionID string, config NetworkCaptureConfig) error {
	resp, err := c.client.StartNetworkCapture(ctx, &StartNetworkCaptureRequest{
		SessionId:           sessionID,
		UrlPatterns:         config.URLPatterns,
		CaptureRequestBody:  config.CaptureRequestBody,
		CaptureResponseBody: config.CaptureResponseBody,
	})
	if err != nil {
		return fmt.Errorf("StartNetworkCapture RPC failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("StartNetworkCapture failed: %s", resp.Error)
	}
	return nil
}

// StopNetworkCapture stops capturing network requests.
func (c *Client) StopNetworkCapture(ctx context.Context, sessionID string) error {
	resp, err := c.client.StopNetworkCapture(ctx, &StopNetworkCaptureRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return fmt.Errorf("StopNetworkCapture RPC failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("StopNetworkCapture failed: %s", resp.Error)
	}
	return nil
}

// CapturedNetworkRequest represents a captured network request.
type CapturedNetworkRequest struct {
	RequestID    string
	Method       string
	URL          string
	Headers      map[string]string
	Body         string
	ResourceType string
	Timestamp    int64
	Response     *CapturedNetworkResponse
}

// CapturedNetworkResponse represents a captured network response.
type CapturedNetworkResponse struct {
	StatusCode int32
	StatusText string
	Headers    map[string]string
	Body       string
	Timestamp  int64
}

// GetCapturedRequests returns captured network requests.
func (c *Client) GetCapturedRequests(ctx context.Context, sessionID string, clearAfter bool) ([]CapturedNetworkRequest, error) {
	resp, err := c.client.GetCapturedRequests(ctx, &GetCapturedRequestsRequest{
		SessionId:  sessionID,
		ClearAfter: clearAfter,
	})
	if err != nil {
		return nil, fmt.Errorf("GetCapturedRequests RPC failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("GetCapturedRequests failed: %s", resp.Error)
	}

	result := make([]CapturedNetworkRequest, len(resp.Requests))
	for i, req := range resp.Requests {
		captured := CapturedNetworkRequest{
			RequestID:    req.Request.RequestId,
			Method:       req.Request.Method,
			URL:          req.Request.Url,
			Headers:      req.Request.Headers,
			Body:         req.Request.Body,
			ResourceType: req.Request.ResourceType,
			Timestamp:    req.Request.Timestamp,
		}
		if req.Response != nil {
			captured.Response = &CapturedNetworkResponse{
				StatusCode: req.Response.StatusCode,
				StatusText: req.Response.StatusText,
				Headers:    req.Response.Headers,
				Body:       req.Response.Body,
				Timestamp:  req.Response.Timestamp,
			}
		}
		result[i] = captured
	}
	return result, nil
}

// GetPageLinks returns all links on the page.
func (c *Client) GetPageLinks(ctx context.Context, sessionID string, sameOriginOnly bool) ([]*PageLink, error) {
	resp, err := c.client.GetPageLinks(ctx, &GetPageLinksRequest{
		SessionId:      sessionID,
		SameOriginOnly: sameOriginOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("GetPageLinks RPC failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("GetPageLinks failed: %s", resp.Error)
	}
	return resp.Links, nil
}

// GetPageForms returns all forms on the page.
func (c *Client) GetPageForms(ctx context.Context, sessionID string) ([]*PageForm, error) {
	resp, err := c.client.GetPageForms(ctx, &GetPageFormsRequest{
		SessionId: sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetPageForms RPC failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("GetPageForms failed: %s", resp.Error)
	}
	return resp.Forms, nil
}

// GetDOMSnapshot returns the DOM tree of the page.
func (c *Client) GetDOMSnapshot(ctx context.Context, sessionID string, selector string, maxDepth int32) (*DOMNode, string, error) {
	resp, err := c.client.GetDOMSnapshot(ctx, &GetDOMSnapshotRequest{
		SessionId:         sessionID,
		Selector:          selector,
		IncludeAttributes: true,
		MaxDepth:          maxDepth,
	})
	if err != nil {
		return nil, "", fmt.Errorf("GetDOMSnapshot RPC failed: %w", err)
	}
	if !resp.Success {
		return nil, "", fmt.Errorf("GetDOMSnapshot failed: %s", resp.Error)
	}
	return resp.Root, resp.Html, nil
}
