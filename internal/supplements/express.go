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

// ExpressSupplement detects Express.js routes and middleware
type ExpressSupplement struct{}

func (s *ExpressSupplement) Name() string {
	return "express"
}

// Detect checks if the project uses Express.js
func (s *ExpressSupplement) Detect(files []string) bool {
	for _, f := range files {
		// Check for package.json with express dependency
		if strings.HasSuffix(f, "package.json") {
			content, err := os.ReadFile(f)
			if err == nil && strings.Contains(string(content), "\"express\"") {
				return true
			}
		}
		// Check for express imports in JS/TS files
		if strings.HasSuffix(f, ".js") || strings.HasSuffix(f, ".ts") {
			content, err := os.ReadFile(f)
			if err == nil {
				contentStr := string(content)
				if strings.Contains(contentStr, "require('express')") ||
					strings.Contains(contentStr, "require(\"express\")") ||
					strings.Contains(contentStr, "from 'express'") ||
					strings.Contains(contentStr, "from \"express\"") {
					return true
				}
			}
		}
	}
	return false
}

// Analyze finds Express routes and adds them to the model
func (s *ExpressSupplement) Analyze(m *model.SystemModel) error {
	// Collect all JS/TS files
	var jsFiles []string
	for _, mod := range m.Modules {
		for _, f := range mod.Files {
			if strings.HasSuffix(f, ".js") || strings.HasSuffix(f, ".ts") {
				jsFiles = append(jsFiles, f)
			}
		}
	}

	// Patterns for Express route definitions
	// app.get('/path', handler)
	// router.post('/path', middleware, handler)
	// app.use('/path', router)
	routePattern := regexp.MustCompile(`(?i)(app|router)\.(get|post|put|patch|delete|use|all)\s*\(\s*['"]([^'"]+)['"]`)

	// Handler patterns
	// app.get('/path', (req, res) => {})
	// app.get('/path', functionName)
	// app.get('/path', controller.method)
	handlerPattern := regexp.MustCompile(`(?i)(app|router)\.(get|post|put|patch|delete)\s*\(\s*['"]([^'"]+)['"]\s*,\s*(?:[\w.]+,\s*)*(\w+(?:\.\w+)?|(?:\([^)]*\)|async\s*)?\s*(?:=>|\{))`)

	for _, filePath := range jsFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		contentStr := string(content)

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Find route definitions
			if matches := routePattern.FindStringSubmatch(line); len(matches) >= 4 {
				method := strings.ToUpper(matches[2])
				path := matches[3]

				// Skip 'use' as it's middleware mounting
				if method == "USE" {
					continue
				}

				// Try to find handler name
				handler := "anonymous"
				if handlerMatches := handlerPattern.FindStringSubmatch(line); len(handlerMatches) >= 5 {
					h := handlerMatches[4]
					// Clean up handler name
					if !strings.Contains(h, "=>") && !strings.Contains(h, "{") {
						handler = h
					}
				}

				// Create endpoint
				endpoint := model.Endpoint{
					ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), method, lineNum),
					Method:    method,
					Path:      path,
					Handler:   handler,
					File:      filePath,
					Line:      lineNum,
					Framework: "express",
				}

				// Extract path parameters (e.g., :id, :userId)
				paramPattern := regexp.MustCompile(`:(\w+)`)
				if paramMatches := paramPattern.FindAllStringSubmatch(path, -1); len(paramMatches) > 0 {
					for _, pm := range paramMatches {
						endpoint.PathParams = append(endpoint.PathParams, pm[1])
					}
				}

				// Try to find middleware in the same line
				endpoint.Middleware = extractMiddleware(line)

				// Extract request body schema (TypeScript typed requests)
				if schema := s.extractRequestSchema(line, contentStr, m); schema != nil {
					endpoint.RequestBody = schema.TypeName
					endpoint.RequestSchema = schema
				}

				m.Endpoints = append(m.Endpoints, endpoint)
			}
		}
		file.Close()

		// Also check for Router definitions to get the base path
		routerMountPattern := regexp.MustCompile(`app\.use\s*\(\s*['"]([^'"]+)['"]\s*,\s*(\w+)`)
		routerMatches := routerMountPattern.FindAllStringSubmatch(contentStr, -1)
		if len(routerMatches) > 0 {
			// Update endpoints with router base paths if applicable
			// This is a simplified version - full implementation would track router definitions
			for _, rm := range routerMatches {
				basePath := rm[1]
				routerName := rm[2]
				_ = basePath
				_ = routerName
				// TODO: Track router definitions and update endpoint paths
			}
		}
	}

	return nil
}

// extractMiddleware extracts middleware function names from a route definition line
func extractMiddleware(line string) []string {
	var middleware []string

	// Pattern: app.get('/path', middleware1, middleware2, handler)
	// We look for function names between the path and the final handler
	parts := strings.Split(line, ",")
	if len(parts) > 2 {
		// Skip first (app.get('/path') and last (handler) parts
		for _, p := range parts[1 : len(parts)-1] {
			p = strings.TrimSpace(p)
			// Skip arrow functions and inline handlers
			if strings.Contains(p, "=>") || strings.Contains(p, "{") || strings.Contains(p, "(") {
				continue
			}
			// This is likely a middleware function name
			if p != "" && isValidIdentifier(p) {
				middleware = append(middleware, p)
			}
		}
	}

	return middleware
}

// isValidIdentifier checks if string is a valid JS identifier
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	// Simple check: starts with letter or underscore, contains only word chars or dots
	matched, _ := regexp.MatchString(`^[a-zA-Z_][\w.]*$`, s)
	return matched
}

// extractRequestSchema extracts request body schema from TypeScript typed requests
// e.g., Request<{}, {}, UserDTO> or req.body as UserDTO
func (s *ExpressSupplement) extractRequestSchema(line string, fileContent string, m *model.SystemModel) *model.BodySchema {
	// Skip primitive types
	skipTypes := map[string]bool{
		"string": true, "number": true, "boolean": true, "object": true,
		"any": true, "unknown": true, "void": true, "never": true,
		"null": true, "undefined": true, "Request": true, "Response": true,
	}

	// Pattern 1: Request<ParamsType, ResBodyType, ReqBodyType>
	// The third type parameter is the request body type
	reqTypePattern := regexp.MustCompile(`Request\s*<[^,]*,\s*[^,]*,\s*(\w+)`)
	if matches := reqTypePattern.FindStringSubmatch(line); len(matches) >= 2 {
		typeName := matches[1]
		if !skipTypes[typeName] && isTypeCapitalized(typeName) {
			return s.buildBodySchema(typeName, m)
		}
	}

	// Pattern 2: req.body as TypeName
	bodyAsPattern := regexp.MustCompile(`req\.body\s+as\s+(\w+)`)
	if matches := bodyAsPattern.FindStringSubmatch(fileContent); len(matches) >= 2 {
		typeName := matches[1]
		if !skipTypes[typeName] && isTypeCapitalized(typeName) {
			return s.buildBodySchema(typeName, m)
		}
	}

	// Pattern 3: const { ... }: TypeName = req.body
	destructurePattern := regexp.MustCompile(`:\s*(\w+)\s*=\s*req\.body`)
	if matches := destructurePattern.FindStringSubmatch(fileContent); len(matches) >= 2 {
		typeName := matches[1]
		if !skipTypes[typeName] && isTypeCapitalized(typeName) {
			return s.buildBodySchema(typeName, m)
		}
	}

	return nil
}

// buildBodySchema creates a BodySchema from a type name
func (s *ExpressSupplement) buildBodySchema(typeName string, m *model.SystemModel) *model.BodySchema {
	schema := &model.BodySchema{
		TypeName:    typeName,
		ContentType: "application/json",
		Required:    true,
	}

	// Look up the type in the model to get field information
	for i := range m.Types {
		if m.Types[i].Name == typeName {
			schema.Fields = s.extractFieldsFromType(&m.Types[i])
			break
		}
	}

	return schema
}

// extractFieldsFromType extracts schema fields from a TypeDef (TypeScript interface)
func (s *ExpressSupplement) extractFieldsFromType(t *model.TypeDef) []model.SchemaField {
	var fields []model.SchemaField

	for _, f := range t.Fields {
		field := model.SchemaField{
			Name:     f.Name,
			Type:     f.Type,
			Required: true, // Default to required
		}

		// Check for optional type (name?)
		if strings.HasSuffix(f.Name, "?") {
			field.Required = false
			field.Name = strings.TrimSuffix(f.Name, "?")
		}

		// Use camelCase name as JSON name for JavaScript
		field.JSONName = f.Name

		fields = append(fields, field)
	}

	return fields
}

// isTypeCapitalized checks if a string starts with an uppercase letter
func isTypeCapitalized(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] >= 'A' && s[0] <= 'Z'
}
