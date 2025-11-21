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

// FastAPISupplement detects FastAPI routes (Python)
type FastAPISupplement struct{}

func (s *FastAPISupplement) Name() string {
	return "fastapi"
}

// Detect checks if the project uses FastAPI
func (s *FastAPISupplement) Detect(files []string) bool {
	for _, f := range files {
		// Check for requirements.txt or pyproject.toml with fastapi
		if strings.HasSuffix(f, "requirements.txt") || strings.HasSuffix(f, "pyproject.toml") {
			content, err := os.ReadFile(f)
			if err == nil && strings.Contains(strings.ToLower(string(content)), "fastapi") {
				return true
			}
		}
		// Check for fastapi imports in Python files
		if strings.HasSuffix(f, ".py") {
			content, err := os.ReadFile(f)
			if err == nil {
				contentStr := string(content)
				if strings.Contains(contentStr, "from fastapi import") ||
					strings.Contains(contentStr, "import fastapi") {
					return true
				}
			}
		}
	}
	return false
}

// Analyze finds FastAPI routes and adds them to the model
func (s *FastAPISupplement) Analyze(m *model.SystemModel) error {
	// Collect all Python files
	var pyFiles []string
	for _, mod := range m.Modules {
		for _, f := range mod.Files {
			if strings.HasSuffix(f, ".py") {
				pyFiles = append(pyFiles, f)
			}
		}
	}

	// Patterns for FastAPI route definitions
	// @app.get("/path")
	// @router.post("/path")
	decoratorPattern := regexp.MustCompile(`@(\w+)\.(get|post|put|patch|delete|options|head)\s*\(\s*["']([^"']+)["']`)

	for _, filePath := range pyFiles {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		var pendingRoute *model.Endpoint

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Find decorator-based route definitions
			if matches := decoratorPattern.FindStringSubmatch(line); len(matches) >= 4 {
				method := strings.ToUpper(matches[2])
				path := matches[3]

				pendingRoute = &model.Endpoint{
					ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), method, lineNum),
					Method:    method,
					Path:      path,
					File:      filePath,
					Line:      lineNum,
					Framework: "fastapi",
				}

				// Extract path parameters (e.g., {id}, {user_id})
				paramPattern := regexp.MustCompile(`\{(\w+)\}`)
				if paramMatches := paramPattern.FindAllStringSubmatch(path, -1); len(paramMatches) > 0 {
					for _, pm := range paramMatches {
						pendingRoute.PathParams = append(pendingRoute.PathParams, pm[1])
					}
				}
			}

			// Look for handler function after decorator
			if pendingRoute != nil && strings.Contains(line, "def ") || strings.Contains(line, "async def ") {
				funcPattern := regexp.MustCompile(`(?:async\s+)?def\s+(\w+)`)
				if funcMatches := funcPattern.FindStringSubmatch(line); len(funcMatches) >= 2 {
					pendingRoute.Handler = funcMatches[1]
					m.Endpoints = append(m.Endpoints, *pendingRoute)
					pendingRoute = nil
				}
			}
		}
		file.Close()
	}

	return nil
}
