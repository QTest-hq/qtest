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

	// Security annotations (middleware)
	// @PreAuthorize("hasRole('ADMIN')")
	// @Secured("ROLE_USER")
	// @RolesAllowed("admin")
	securityPattern := regexp.MustCompile(`@(PreAuthorize|Secured|RolesAllowed)\s*\(\s*"?([^")]+)"?\s*\)`)

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
			method     string
			path       string
			line       int
			middleware []string
		}

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// Check for security annotations (middleware)
			if secMatches := securityPattern.FindStringSubmatch(trimmed); len(secMatches) >= 3 {
				annotType := secMatches[1]
				value := secMatches[2]
				middleware := fmt.Sprintf("%s(%s)", annotType, value)
				if pendingAnnotation != nil {
					pendingAnnotation.middleware = append(pendingAnnotation.middleware, middleware)
				}
			}

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
					method     string
					path       string
					line       int
					middleware []string
				}{method, path, lineNum, nil}
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
						Framework:  "springboot",
						Middleware: pendingAnnotation.middleware,
					}

					// Extract path variables (e.g., {id}, {userId})
					paramPattern := regexp.MustCompile(`\{(\w+)\}`)
					if paramMatches := paramPattern.FindAllStringSubmatch(fullPath, -1); len(paramMatches) > 0 {
						for _, pm := range paramMatches {
							endpoint.PathParams = append(endpoint.PathParams, pm[1])
						}
					}

					// Extract request body from @RequestBody annotation
					if schema := s.extractRequestSchema(trimmed, m); schema != nil {
						endpoint.RequestBody = schema.TypeName
						endpoint.RequestSchema = schema
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

// extractRequestSchema extracts request body schema from @RequestBody annotation
// e.g., public ResponseEntity<User> createUser(@RequestBody User user) {...}
// Returns the Java class name used for the request body
func (s *SpringBootSupplement) extractRequestSchema(line string, m *model.SystemModel) *model.BodySchema {
	// Skip types that are not request body types
	skipTypes := map[string]bool{
		"String": true, "Integer": true, "Long": true, "Double": true,
		"Float": true, "Boolean": true, "Object": true,
		"HttpServletRequest": true, "HttpServletResponse": true,
		"Model": true, "ModelAndView": true, "BindingResult": true,
		"Principal": true, "Authentication": true,
	}

	// Pattern: @RequestBody TypeName varName
	// Also handles @RequestBody @Valid TypeName varName
	requestBodyPattern := regexp.MustCompile(`@RequestBody\s+(?:@\w+\s+)*(\w+)(?:<[^>]+>)?\s+\w+`)
	if matches := requestBodyPattern.FindStringSubmatch(line); len(matches) >= 2 {
		typeName := matches[1]
		if !skipTypes[typeName] {
			return s.buildBodySchema(typeName, m)
		}
	}

	return nil
}

// buildBodySchema creates a BodySchema from a type name
func (s *SpringBootSupplement) buildBodySchema(typeName string, m *model.SystemModel) *model.BodySchema {
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

// extractFieldsFromType extracts schema fields from a TypeDef (Java class)
func (s *SpringBootSupplement) extractFieldsFromType(t *model.TypeDef) []model.SchemaField {
	var fields []model.SchemaField

	for _, f := range t.Fields {
		field := model.SchemaField{
			Name:     f.Name,
			Type:     f.Type,
			Required: true, // Default to required
		}

		// Check for Optional type or @Nullable annotation
		if strings.Contains(f.Type, "Optional") || strings.Contains(f.Type, "Nullable") {
			field.Required = false
		}

		// Use camelCase name as JSON name for Java
		field.JSONName = f.Name

		fields = append(fields, field)
	}

	return fields
}
