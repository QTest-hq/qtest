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

// SpringBootSupplement detects Spring Boot REST endpoints (Java)
type SpringBootSupplement struct{}

func (s *SpringBootSupplement) Name() string {
	return "springboot"
}

// Detect checks if the project uses Spring Boot
func (s *SpringBootSupplement) Detect(files []string) bool {
	for _, f := range files {
		// Check for pom.xml with spring-boot
		if strings.HasSuffix(f, "pom.xml") {
			content, err := os.ReadFile(f)
			if err == nil && strings.Contains(string(content), "spring-boot") {
				return true
			}
		}
		// Check for build.gradle with spring-boot
		if strings.HasSuffix(f, "build.gradle") || strings.HasSuffix(f, "build.gradle.kts") {
			content, err := os.ReadFile(f)
			if err == nil && strings.Contains(string(content), "spring-boot") {
				return true
			}
		}
		// Check for Spring annotations in Java files
		if strings.HasSuffix(f, ".java") {
			content, err := os.ReadFile(f)
			if err == nil {
				if strings.Contains(string(content), "@RestController") ||
					strings.Contains(string(content), "@Controller") ||
					strings.Contains(string(content), "@RequestMapping") {
					return true
				}
			}
		}
	}
	return false
}

// Analyze finds Spring Boot REST endpoints and adds them to the model
func (s *SpringBootSupplement) Analyze(m *model.SystemModel) error {
	// Collect all Java files
	var javaFiles []string
	for _, mod := range m.Modules {
		for _, f := range mod.Files {
			if strings.HasSuffix(f, ".java") {
				javaFiles = append(javaFiles, f)
			}
		}
	}

	// Patterns for Spring annotations
	// @GetMapping("/path")
	// @PostMapping("/path")
	// @RequestMapping(value = "/path", method = RequestMethod.GET)
	mappingAnnotations := regexp.MustCompile(`@(Get|Post|Put|Patch|Delete|Request)Mapping\s*\(`)
	pathPattern := regexp.MustCompile(`(?:value\s*=\s*)?"([^"]+)"`)
	methodPattern := regexp.MustCompile(`method\s*=\s*RequestMethod\.(\w+)`)

	// Handler method pattern (public void/ResponseEntity/Object methodName)
	handlerPattern := regexp.MustCompile(`public\s+\w+(?:<[^>]+>)?\s+(\w+)\s*\(`)

	// Class-level RequestMapping for base path
	classPathPattern := regexp.MustCompile(`@RequestMapping\s*\(\s*(?:value\s*=\s*)?"([^"]+)"`)

	for _, filePath := range javaFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		var basePath string

		// First pass: find class-level RequestMapping
		for _, line := range lines {
			if matches := classPathPattern.FindStringSubmatch(line); len(matches) >= 2 {
				// Check if this is before a class definition
				basePath = matches[1]
				break
			}
		}

		// Second pass: find endpoint annotations
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		var pendingAnnotation *struct {
			method string
			path   string
			line   int
		}

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// Check for mapping annotation
			if matches := mappingAnnotations.FindStringSubmatch(trimmed); len(matches) >= 2 {
				annotationType := strings.ToUpper(matches[1])

				// Determine HTTP method
				method := "GET"
				switch annotationType {
				case "GET":
					method = "GET"
				case "POST":
					method = "POST"
				case "PUT":
					method = "PUT"
				case "PATCH":
					method = "PATCH"
				case "DELETE":
					method = "DELETE"
				case "REQUEST":
					// Look for method in the annotation
					if methodMatch := methodPattern.FindStringSubmatch(trimmed); len(methodMatch) >= 2 {
						method = strings.ToUpper(methodMatch[1])
					}
				}

				// Extract path
				path := ""
				if pathMatch := pathPattern.FindStringSubmatch(trimmed); len(pathMatch) >= 2 {
					path = pathMatch[1]
				}

				pendingAnnotation = &struct {
					method string
					path   string
					line   int
				}{method, path, lineNum}
			}

			// Check for handler method after annotation
			if pendingAnnotation != nil {
				if handlerMatch := handlerPattern.FindStringSubmatch(trimmed); len(handlerMatch) >= 2 {
					handler := handlerMatch[1]

					// Combine base path and endpoint path
					fullPath := basePath + pendingAnnotation.path
					if !strings.HasPrefix(fullPath, "/") {
						fullPath = "/" + fullPath
					}

					endpoint := model.Endpoint{
						ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), pendingAnnotation.method, pendingAnnotation.line),
						Method:    pendingAnnotation.method,
						Path:      fullPath,
						Handler:   handler,
						File:      filePath,
						Line:      pendingAnnotation.line,
						Framework: "springboot",
					}

					// Extract path variables (e.g., {id}, {userId})
					paramPattern := regexp.MustCompile(`\{(\w+)\}`)
					if paramMatches := paramPattern.FindAllStringSubmatch(fullPath, -1); len(paramMatches) > 0 {
						for _, pm := range paramMatches {
							endpoint.PathParams = append(endpoint.PathParams, pm[1])
						}
					}

					m.Endpoints = append(m.Endpoints, endpoint)
					pendingAnnotation = nil
				}
			}
		}
		file.Close()
	}

	return nil
}
