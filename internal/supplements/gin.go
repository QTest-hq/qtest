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

// GinBindPatterns for detecting request body binding
var ginBindPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\.ShouldBindJSON\s*\(\s*&(\w+)\s*\)`),
	regexp.MustCompile(`\.BindJSON\s*\(\s*&(\w+)\s*\)`),
	regexp.MustCompile(`\.ShouldBind\s*\(\s*&(\w+)\s*\)`),
	regexp.MustCompile(`\.Bind\s*\(\s*&(\w+)\s*\)`),
	regexp.MustCompile(`\.ShouldBindUri\s*\(\s*&(\w+)\s*\)`),
	regexp.MustCompile(`\.ShouldBindQuery\s*\(\s*&(\w+)\s*\)`),
}

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

	// Build type map for schema lookup
	typeMap := s.buildTypeMap(m)

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

				// Extract request body schema from handler function
				if handler != "anonymous" {
					if schema := s.extractRequestSchema(handler, m, typeMap); schema != nil {
						endpoint.RequestBody = schema.TypeName
						endpoint.RequestSchema = schema
					}
				}

				// Extract middleware from route definition
				endpoint.Middleware = s.extractMiddleware(line)

				m.Endpoints = append(m.Endpoints, endpoint)
			}
		}
		file.Close()
	}

	return nil
}

// buildTypeMap creates a map of type name to TypeDef for quick lookup
func (s *GinSupplement) buildTypeMap(m *model.SystemModel) map[string]*model.TypeDef {
	typeMap := make(map[string]*model.TypeDef)
	for i := range m.Types {
		t := &m.Types[i]
		typeMap[t.Name] = t
		// Also store with full module path if available
		if t.Module != "" {
			typeMap[t.Module+"."+t.Name] = t
		}
	}
	return typeMap
}

// extractRequestSchema extracts request body schema from a handler function
func (s *GinSupplement) extractRequestSchema(handlerName string, m *model.SystemModel, typeMap map[string]*model.TypeDef) *model.BodySchema {
	// Find the handler function in the model
	var handlerFunc *model.Function
	for i := range m.Functions {
		fn := &m.Functions[i]
		if fn.Name == handlerName || strings.HasSuffix(fn.ID, handlerName) {
			handlerFunc = fn
			break
		}
	}

	if handlerFunc == nil || handlerFunc.Body == "" {
		return nil
	}

	// Look for binding patterns in the function body
	for _, pattern := range ginBindPatterns {
		if matches := pattern.FindStringSubmatch(handlerFunc.Body); len(matches) >= 2 {
			varName := matches[1]

			// Find the type of this variable by looking for var declarations
			// Pattern: var varName TypeName or varName := TypeName{}
			typePattern := regexp.MustCompile(`(?:var\s+` + varName + `\s+(\w+)|` + varName + `\s*:=\s*(\w+)\s*\{)`)
			if typeMatches := typePattern.FindStringSubmatch(handlerFunc.Body); len(typeMatches) >= 2 {
				typeName := typeMatches[1]
				if typeName == "" {
					typeName = typeMatches[2]
				}

				schema := &model.BodySchema{
					TypeName:    typeName,
					ContentType: "application/json",
					Required:    true,
				}

				// Look up the type to get field information
				if typeDef, ok := typeMap[typeName]; ok {
					schema.Fields = s.extractFieldsFromType(typeDef)
				}

				return schema
			}
		}
	}

	return nil
}

// extractFieldsFromType extracts schema fields from a TypeDef
func (s *GinSupplement) extractFieldsFromType(t *model.TypeDef) []model.SchemaField {
	var fields []model.SchemaField

	for _, f := range t.Fields {
		field := model.SchemaField{
			Name:     f.Name,
			Type:     f.Type,
			Required: !strings.Contains(f.Tags, "omitempty"),
		}

		// Parse JSON tag for field name
		if f.Tags != "" {
			jsonTagPattern := regexp.MustCompile(`json:"([^,"]+)`)
			if matches := jsonTagPattern.FindStringSubmatch(f.Tags); len(matches) >= 2 {
				field.JSONName = matches[1]
				if field.JSONName == "-" {
					continue // Skip fields marked as json:"-"
				}
			}

			// Parse binding/validation tags
			bindingPattern := regexp.MustCompile(`binding:"([^"]+)"`)
			if matches := bindingPattern.FindStringSubmatch(f.Tags); len(matches) >= 2 {
				field.Validation = matches[1]
				if strings.Contains(field.Validation, "required") {
					field.Required = true
				}
			}
		}

		fields = append(fields, field)
	}

	return fields
}

// extractMiddleware extracts middleware names from a route definition line
// e.g., r.POST("/api/users", AuthMiddleware, RateLimiter, CreateUser)
// returns []string{"AuthMiddleware", "RateLimiter"}
func (s *GinSupplement) extractMiddleware(line string) []string {
	// Pattern: method call with multiple arguments - all except the last are middleware
	// r.POST("/path", middleware1, middleware2, handler)
	argsPattern := regexp.MustCompile(`\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s*\(\s*"[^"]+"\s*,\s*([^)]+)\)`)
	matches := argsPattern.FindStringSubmatch(line)
	if len(matches) < 3 {
		return nil
	}

	// Split the arguments
	argsStr := matches[2]
	args := strings.Split(argsStr, ",")
	if len(args) <= 1 {
		return nil // No middleware, just handler
	}

	// All args except the last are middleware
	var middleware []string
	for i := 0; i < len(args)-1; i++ {
		mw := strings.TrimSpace(args[i])
		// Skip empty strings and inline function definitions
		if mw != "" && !strings.HasPrefix(mw, "func") && !strings.Contains(mw, "{") {
			middleware = append(middleware, mw)
		}
	}

	return middleware
}
