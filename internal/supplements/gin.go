package supplements

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/QTest-hq/qtest/pkg/model"
)

// GinSupplement detects Gin routes (Go)
type GinSupplement struct{}

func (s *GinSupplement) Name() string {
	return "gin"
}

// Detect checks if the project uses Gin
func (s *GinSupplement) Detect(files []string) bool {
	for _, f := range files {
		// Check for go.mod with gin-gonic
		if strings.HasSuffix(f, "go.mod") {
			content, err := os.ReadFile(f)
			if err == nil && strings.Contains(string(content), "github.com/gin-gonic/gin") {
				return true
			}
		}
		// Check for gin imports in Go files
		if strings.HasSuffix(f, ".go") {
			content, err := os.ReadFile(f)
			if err == nil && strings.Contains(string(content), "\"github.com/gin-gonic/gin\"") {
				return true
			}
		}
	}
	return false
}

// Analyze finds Gin routes and adds them to the model
func (s *GinSupplement) Analyze(m *model.SystemModel) error {
	// Collect all Go files
	var goFiles []string
	for _, mod := range m.Modules {
		for _, f := range mod.Files {
			if strings.HasSuffix(f, ".go") {
				goFiles = append(goFiles, f)
			}
		}
	}

	// Patterns for Gin route definitions
	// r.GET("/path", handler)
	// router.POST("/path", middleware, handler)
	// g.PUT("/path", handler)  // from group
	routePattern := regexp.MustCompile(`(\w+)\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s*\(\s*"([^"]+)"`)

	// Handler pattern - last argument in the route definition
	handlerPattern := regexp.MustCompile(`(\w+)\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s*\(\s*"[^"]+"\s*(?:,\s*[\w.]+)*,\s*([\w.]+)\s*\)`)

	for _, filePath := range goFiles {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Find route definitions
			if matches := routePattern.FindStringSubmatch(line); len(matches) >= 4 {
				method := matches[2] // Already uppercase
				path := matches[3]

				// Find handler name
				handler := "anonymous"
				if handlerMatches := handlerPattern.FindStringSubmatch(line); len(handlerMatches) >= 4 {
					handler = handlerMatches[3]
				}

				endpoint := model.Endpoint{
					ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), method, lineNum),
					Method:    method,
					Path:      path,
					Handler:   handler,
					File:      filePath,
					Line:      lineNum,
					Framework: "gin",
				}

				// Extract path parameters (e.g., :id, :userId)
				paramPattern := regexp.MustCompile(`:(\w+)`)
				if paramMatches := paramPattern.FindAllStringSubmatch(path, -1); len(paramMatches) > 0 {
					for _, pm := range paramMatches {
						endpoint.PathParams = append(endpoint.PathParams, pm[1])
					}
				}

				m.Endpoints = append(m.Endpoints, endpoint)
			}
		}
		file.Close()
	}

	return nil
}
