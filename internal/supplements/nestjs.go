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

// NestJSSupplement detects NestJS routes (TypeScript)
type NestJSSupplement struct{}

func (s *NestJSSupplement) Name() string {
	return "nestjs"
}

// Detect checks if the project uses NestJS
func (s *NestJSSupplement) Detect(files []string) bool {
	for _, f := range files {
		// Check for package.json with @nestjs
		if strings.HasSuffix(f, "package.json") {
			content, err := os.ReadFile(f)
			if err == nil && strings.Contains(string(content), "@nestjs/") {
				return true
			}
		}
		// Check for NestJS decorators in TS files
		if strings.HasSuffix(f, ".ts") {
			content, err := os.ReadFile(f)
			if err == nil {
				str := string(content)
				if strings.Contains(str, "@Controller") ||
					strings.Contains(str, "@nestjs/common") {
					return true
				}
			}
		}
	}
	return false
}

// Analyze finds NestJS routes and adds them to the model
func (s *NestJSSupplement) Analyze(m *model.SystemModel) error {
	// Collect all TypeScript files
	var tsFiles []string
	for _, mod := range m.Modules {
		for _, f := range mod.Files {
			if strings.HasSuffix(f, ".ts") && !strings.HasSuffix(f, ".spec.ts") && !strings.HasSuffix(f, ".test.ts") {
				tsFiles = append(tsFiles, f)
			}
		}
	}

	// Patterns for NestJS decorators
	// @Controller('users')
	// @Get(':id')
	// @Post()
	// @Put(':id')
	// @Delete(':id')
	// @Patch(':id')
	controllerPattern := regexp.MustCompile(`@Controller\s*\(\s*['"]?([^'")\s]*)['"]?\s*\)`)
	routeDecorators := regexp.MustCompile(`@(Get|Post|Put|Patch|Delete|Head|Options|All)\s*\(\s*['"]?([^'")\s]*)['"]?\s*\)`)

	// Middleware/Guard/Interceptor patterns
	// @UseGuards(AuthGuard)
	// @UsePipes(ValidationPipe)
	// @UseInterceptors(LoggingInterceptor)
	guardPattern := regexp.MustCompile(`@UseGuards\s*\(([^)]+)\)`)
	pipePattern := regexp.MustCompile(`@UsePipes\s*\(([^)]+)\)`)
	interceptorPattern := regexp.MustCompile(`@UseInterceptors\s*\(([^)]+)\)`)

	// Handler method pattern
	handlerPattern := regexp.MustCompile(`(?:async\s+)?(\w+)\s*\(`)

	for _, filePath := range tsFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		var basePath string

		// First pass: find @Controller decorator
		for _, line := range lines {
			if matches := controllerPattern.FindStringSubmatch(line); len(matches) >= 2 {
				basePath = matches[1]
				if basePath != "" && !strings.HasPrefix(basePath, "/") {
					basePath = "/" + basePath
				}
				break
			}
		}

		// First pass also: extract controller-level middleware
		var controllerMiddleware []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Check for controller-level guards
			if matches := guardPattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				controllerMiddleware = append(controllerMiddleware, s.extractMiddlewareNames(matches[1])...)
			}
			if matches := pipePattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				controllerMiddleware = append(controllerMiddleware, s.extractMiddlewareNames(matches[1])...)
			}
			if matches := interceptorPattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				controllerMiddleware = append(controllerMiddleware, s.extractMiddlewareNames(matches[1])...)
			}
		}

		// Second pass: find route decorators
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		var pendingRoute *struct {
			method     string
			path       string
			line       int
			middleware []string
		}
		var pendingMiddleware []string // method-level middleware

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// Check for method-level middleware (before route decorator)
			if matches := guardPattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				pendingMiddleware = append(pendingMiddleware, s.extractMiddlewareNames(matches[1])...)
			}
			if matches := pipePattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				pendingMiddleware = append(pendingMiddleware, s.extractMiddlewareNames(matches[1])...)
			}
			if matches := interceptorPattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				pendingMiddleware = append(pendingMiddleware, s.extractMiddlewareNames(matches[1])...)
			}

			// Check for route decorator
			if matches := routeDecorators.FindStringSubmatch(trimmed); len(matches) >= 3 {
				method := strings.ToUpper(matches[1])
				if method == "ALL" {
					method = "GET" // Default to GET for @All
				}
				routePath := matches[2]

				// Combine controller-level and method-level middleware
				allMiddleware := append([]string{}, controllerMiddleware...)
				allMiddleware = append(allMiddleware, pendingMiddleware...)

				pendingRoute = &struct {
					method     string
					path       string
					line       int
					middleware []string
				}{method, routePath, lineNum, allMiddleware}
				pendingMiddleware = nil // Reset method-level middleware
			}

			// Check for handler method after decorator
			if pendingRoute != nil && !strings.HasPrefix(trimmed, "@") {
				if handlerMatch := handlerPattern.FindStringSubmatch(trimmed); len(handlerMatch) >= 2 {
					handler := handlerMatch[1]

					// Skip if it's a decorator parameter
					if handler == "Controller" || handler == "Get" || handler == "Post" ||
						handler == "Put" || handler == "Patch" || handler == "Delete" {
						continue
					}

					// Combine base path and route path
					fullPath := basePath
					if pendingRoute.path != "" {
						if strings.HasPrefix(pendingRoute.path, "/") {
							fullPath = pendingRoute.path
						} else {
							fullPath = basePath + "/" + pendingRoute.path
						}
					}
					if fullPath == "" {
						fullPath = "/"
					}

					// Clean up double slashes
					fullPath = regexp.MustCompile(`//+`).ReplaceAllString(fullPath, "/")

					endpoint := model.Endpoint{
						ID:         fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), pendingRoute.method, pendingRoute.line),
						Method:     pendingRoute.method,
						Path:       fullPath,
						Handler:    handler,
						File:       filePath,
						Line:       pendingRoute.line,
						Framework:  "nestjs",
						Middleware: pendingRoute.middleware,
					}

					// Extract path parameters (e.g., :id, :userId)
					paramPattern := regexp.MustCompile(`:(\w+)`)
					if paramMatches := paramPattern.FindAllStringSubmatch(fullPath, -1); len(paramMatches) > 0 {
						for _, pm := range paramMatches {
							endpoint.PathParams = append(endpoint.PathParams, pm[1])
						}
					}

					m.Endpoints = append(m.Endpoints, endpoint)
					pendingRoute = nil
				}
			}
		}
		file.Close()
	}

	return nil
}

// extractMiddlewareNames extracts class/function names from NestJS decorator arguments
// e.g., "AuthGuard, RolesGuard" -> ["AuthGuard", "RolesGuard"]
// e.g., "AuthGuard('jwt')" -> ["AuthGuard"]
func (s *NestJSSupplement) extractMiddlewareNames(str string) []string {
	var names []string
	// Match class/function names (capitalized identifiers or function calls)
	namePattern := regexp.MustCompile(`\b([A-Z]\w*)`)
	matches := namePattern.FindAllStringSubmatch(str, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			names = append(names, m[1])
		}
	}
	return names
}
