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
	// @app.get("/path", dependencies=[Depends(auth)])
	decoratorPattern := regexp.MustCompile(`@(\w+)\.(get|post|put|patch|delete|options|head)\s*\(\s*["']([^"']+)["']`)

	for _, filePath := range pyFiles {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		var pendingRoute *model.Endpoint
		var routeDecoratorLine string

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
				routeDecoratorLine = line

				// Extract path parameters (e.g., {id}, {user_id})
				paramPattern := regexp.MustCompile(`\{(\w+)\}`)
				if paramMatches := paramPattern.FindAllStringSubmatch(path, -1); len(paramMatches) > 0 {
					for _, pm := range paramMatches {
						pendingRoute.PathParams = append(pendingRoute.PathParams, pm[1])
					}
				}

				// Extract middleware from dependencies in decorator
				pendingRoute.Middleware = s.extractMiddleware(routeDecoratorLine)
			}

			// Look for handler function after decorator
			if pendingRoute != nil && strings.Contains(line, "def ") || strings.Contains(line, "async def ") {
				funcPattern := regexp.MustCompile(`(?:async\s+)?def\s+(\w+)`)
				if funcMatches := funcPattern.FindStringSubmatch(line); len(funcMatches) >= 2 {
					pendingRoute.Handler = funcMatches[1]

					// Also extract Depends() from function parameters as middleware
					paramMiddleware := s.extractDependsFromParams(line)
					pendingRoute.Middleware = append(pendingRoute.Middleware, paramMiddleware...)

					// Extract request body schema from function parameters
					if schema := s.extractRequestSchema(line, m); schema != nil {
						pendingRoute.RequestBody = schema.TypeName
						pendingRoute.RequestSchema = schema
					}

					m.Endpoints = append(m.Endpoints, *pendingRoute)
					pendingRoute = nil
					routeDecoratorLine = ""
				}
			}
		}
		file.Close()
	}

	return nil
}

// extractMiddleware extracts middleware/dependencies from FastAPI route decorator
// e.g., @app.get("/path", dependencies=[Depends(auth), Depends(rate_limit)])
// returns []string{"auth", "rate_limit"}
func (s *FastAPISupplement) extractMiddleware(line string) []string {
	var middleware []string

	// Pattern: dependencies=[Depends(name), ...]
	dependsPattern := regexp.MustCompile(`Depends\s*\(\s*(\w+)`)
	matches := dependsPattern.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			middleware = append(middleware, match[1])
		}
	}

	return middleware
}

// extractDependsFromParams extracts Depends() calls from function parameters
// e.g., async def get_user(user: User = Depends(get_current_user)):
// returns []string{"get_current_user"}
func (s *FastAPISupplement) extractDependsFromParams(line string) []string {
	var middleware []string

	// Pattern: param_name: type = Depends(func_name) or Depends(Class)
	dependsPattern := regexp.MustCompile(`=\s*Depends\s*\(\s*(\w+)`)
	matches := dependsPattern.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			// Skip if it looks like a common parameter type (lowercase starting)
			funcName := match[1]
			// Common auth/middleware patterns
			middleware = append(middleware, funcName)
		}
	}

	return middleware
}

// extractRequestSchema extracts request body schema from function parameters
// e.g., async def create_user(user: User): or async def update(item: Item = Body(...)):
// Returns the Pydantic model name used for the request body
func (s *FastAPISupplement) extractRequestSchema(line string, m *model.SystemModel) *model.BodySchema {
	// Skip non-body parameter types
	skipTypes := map[string]bool{
		"Request": true, "Response": true, "Depends": true,
		"Query": true, "Path": true, "Header": true, "Cookie": true,
		"str": true, "int": true, "float": true, "bool": true,
		"list": true, "dict": true, "List": true, "Dict": true,
		"Optional": true, "Any": true, "None": true,
		"BackgroundTasks": true, "UploadFile": true, "File": true,
		"HTTPException": true, "Session": true, "DBSession": true,
	}

	// Pattern 1: param: Type = Body(...) - explicit body
	bodyPattern := regexp.MustCompile(`(\w+)\s*:\s*(\w+)\s*=\s*Body\s*\(`)
	if matches := bodyPattern.FindStringSubmatch(line); len(matches) >= 3 {
		typeName := matches[2]
		if !skipTypes[typeName] && isCapitalized(typeName) {
			return s.buildBodySchema(typeName, m)
		}
	}

	// Pattern 2: param: Type (without default) - Pydantic model as body
	// Look for type-hinted params that are Pydantic models (capitalized, not in skip list)
	paramPattern := regexp.MustCompile(`(\w+)\s*:\s*(\w+)(?:\s*[,)]|\s*=)`)
	matches := paramPattern.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			typeName := match[2]
			// Skip known non-body types and lowercase types (primitives)
			if skipTypes[typeName] {
				continue
			}
			// Pydantic models are typically capitalized
			if isCapitalized(typeName) {
				return s.buildBodySchema(typeName, m)
			}
		}
	}

	return nil
}

// buildBodySchema creates a BodySchema from a type name
func (s *FastAPISupplement) buildBodySchema(typeName string, m *model.SystemModel) *model.BodySchema {
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

// extractFieldsFromType extracts schema fields from a TypeDef (Pydantic model)
func (s *FastAPISupplement) extractFieldsFromType(t *model.TypeDef) []model.SchemaField {
	var fields []model.SchemaField

	for _, f := range t.Fields {
		field := model.SchemaField{
			Name:     f.Name,
			Type:     f.Type,
			Required: true, // Pydantic fields are required by default
		}

		// Check for Optional type
		if strings.Contains(f.Type, "Optional") {
			field.Required = false
		}

		// Use snake_case name as JSON name for Python
		field.JSONName = toSnakeCase(f.Name)

		fields = append(fields, field)
	}

	return fields
}

// isCapitalized checks if a string starts with an uppercase letter
func isCapitalized(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] >= 'A' && s[0] <= 'Z'
}

// toSnakeCase converts CamelCase to snake_case
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+32) // to lowercase
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
