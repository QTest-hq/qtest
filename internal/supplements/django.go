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

// DjangoSupplement detects Django and Django REST Framework endpoints (Python)
type DjangoSupplement struct{}

func (s *DjangoSupplement) Name() string {
	return "django"
}

// Detect checks if the project uses Django/DRF
func (s *DjangoSupplement) Detect(files []string) bool {
	for _, f := range files {
		// Check for requirements.txt with Django
		if strings.HasSuffix(f, "requirements.txt") {
			content, err := os.ReadFile(f)
			if err == nil {
				str := string(content)
				if strings.Contains(str, "django") || strings.Contains(str, "Django") ||
					strings.Contains(str, "djangorestframework") {
					return true
				}
			}
		}
		// Check for pyproject.toml with Django
		if strings.HasSuffix(f, "pyproject.toml") {
			content, err := os.ReadFile(f)
			if err == nil {
				str := string(content)
				if strings.Contains(str, "django") || strings.Contains(str, "djangorestframework") {
					return true
				}
			}
		}
		// Check for manage.py (Django project indicator)
		if strings.HasSuffix(f, "manage.py") {
			content, err := os.ReadFile(f)
			if err == nil && strings.Contains(string(content), "django") {
				return true
			}
		}
		// Check for Django/DRF imports in Python files
		if strings.HasSuffix(f, ".py") {
			content, err := os.ReadFile(f)
			if err == nil {
				str := string(content)
				if strings.Contains(str, "from django") || strings.Contains(str, "from rest_framework") {
					return true
				}
			}
		}
	}
	return false
}

// Analyze finds Django endpoints and adds them to the model
func (s *DjangoSupplement) Analyze(m *model.SystemModel) error {
	// Collect all Python files
	var pyFiles []string
	for _, mod := range m.Modules {
		for _, f := range mod.Files {
			if strings.HasSuffix(f, ".py") {
				pyFiles = append(pyFiles, f)
			}
		}
	}

	// Find urls.py files first to map URL patterns
	urlPatterns := s.parseURLPatterns(pyFiles)

	// Patterns for Django views and DRF viewsets
	// class MyView(APIView):
	// class MyViewSet(ViewSet):
	// @api_view(['GET', 'POST'])
	// def my_function(request):
	classPattern := regexp.MustCompile(`class\s+(\w+)\s*\((?:[^)]*(?:APIView|ViewSet|GenericViewSet|ModelViewSet|ReadOnlyModelViewSet|View|TemplateView|CreateView|UpdateView|DeleteView|ListView|DetailView)[^)]*)\):`)

	// DRF method decorators
	// @action(detail=True, methods=['get', 'post'])
	actionPattern := regexp.MustCompile(`@action\s*\(\s*(?:detail\s*=\s*(True|False))?\s*(?:,\s*methods\s*=\s*\[([^\]]+)\])?\s*\)`)

	// Function-based views with decorators
	apiViewPattern := regexp.MustCompile(`@api_view\s*\(\s*\[([^\]]+)\]\s*\)`)
	funcPattern := regexp.MustCompile(`def\s+(\w+)\s*\(\s*request`)

	// Standard HTTP methods in class-based views
	httpMethods := []string{"get", "post", "put", "patch", "delete", "head", "options"}

	for _, filePath := range pyFiles {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			file.Close()
			continue
		}

		lines := strings.Split(string(content), "\n")
		scanner := bufio.NewScanner(file)
		lineNum := 0
		var currentClass string
		var pendingAPIView *struct {
			methods []string
			line    int
		}
		var pendingAction *struct {
			methods []string
			detail  bool
			line    int
		}

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// Check for class definition
			if matches := classPattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				currentClass = matches[1]

				// Add endpoint for the viewset/view class
				if basePath, ok := urlPatterns[currentClass]; ok {
					endpoint := model.Endpoint{
						ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), "VIEW", lineNum),
						Method:    "GET", // Default for views
						Path:      basePath,
						Handler:   currentClass,
						File:      filePath,
						Line:      lineNum,
						Framework: "django",
					}
					m.Endpoints = append(m.Endpoints, endpoint)
				}
			}

			// Check for @api_view decorator
			if matches := apiViewPattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				methods := s.parseMethods(matches[1])
				pendingAPIView = &struct {
					methods []string
					line    int
				}{methods, lineNum}
			}

			// Check for @action decorator
			if matches := actionPattern.FindStringSubmatch(trimmed); len(matches) >= 3 {
				methods := []string{"GET"}
				if matches[2] != "" {
					methods = s.parseMethods(matches[2])
				}
				detail := matches[1] == "True"
				pendingAction = &struct {
					methods []string
					detail  bool
					line    int
				}{methods, detail, lineNum}
			}

			// Check for function definition after decorator
			if matches := funcPattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				funcName := matches[1]

				if pendingAPIView != nil {
					// Function-based view with @api_view
					basePath := urlPatterns[funcName]
					if basePath == "" {
						basePath = "/" + strings.ToLower(strings.ReplaceAll(funcName, "_", "-"))
					}

					for _, method := range pendingAPIView.methods {
						endpoint := model.Endpoint{
							ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), method, pendingAPIView.line),
							Method:    method,
							Path:      basePath,
							Handler:   funcName,
							File:      filePath,
							Line:      pendingAPIView.line,
							Framework: "django",
						}
						m.Endpoints = append(m.Endpoints, endpoint)
					}
					pendingAPIView = nil
				} else if pendingAction != nil && currentClass != "" {
					// Action in a viewset
					basePath := urlPatterns[currentClass]
					if basePath == "" {
						basePath = "/" + strings.ToLower(strings.ReplaceAll(currentClass, "ViewSet", ""))
					}

					actionPath := basePath + "/" + strings.ToLower(strings.ReplaceAll(funcName, "_", "-"))
					if pendingAction.detail {
						actionPath = basePath + "/{id}/" + strings.ToLower(strings.ReplaceAll(funcName, "_", "-"))
					}

					for _, method := range pendingAction.methods {
						endpoint := model.Endpoint{
							ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), method, pendingAction.line),
							Method:    method,
							Path:      actionPath,
							Handler:   currentClass + "." + funcName,
							File:      filePath,
							Line:      pendingAction.line,
							Framework: "django",
						}
						m.Endpoints = append(m.Endpoints, endpoint)
					}
					pendingAction = nil
				}
			}

			// Check for HTTP method handlers in class-based views
			if currentClass != "" {
				for _, method := range httpMethods {
					methodPattern := regexp.MustCompile(fmt.Sprintf(`def\s+%s\s*\(\s*self`, method))
					if methodPattern.MatchString(trimmed) {
						basePath := urlPatterns[currentClass]
						if basePath == "" {
							basePath = "/" + strings.ToLower(strings.ReplaceAll(currentClass, "View", ""))
						}

						endpoint := model.Endpoint{
							ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), strings.ToUpper(method), lineNum),
							Method:    strings.ToUpper(method),
							Path:      basePath,
							Handler:   currentClass + "." + method,
							File:      filePath,
							Line:      lineNum,
							Framework: "django",
						}
						m.Endpoints = append(m.Endpoints, endpoint)
					}
				}
			}
		}

		// Scan content for standard def patterns without decorator
		for lineIdx, line := range lines {
			lineNum := lineIdx + 1
			trimmed := strings.TrimSpace(line)
			if funcMatch := funcPattern.FindStringSubmatch(trimmed); len(funcMatch) >= 2 {
				funcName := funcMatch[1]
				// Check if this function is in url patterns
				if basePath, ok := urlPatterns[funcName]; ok {
					endpoint := model.Endpoint{
						ID:        fmt.Sprintf("ep:%s:%s:%d", filepath.Base(filePath), "VIEW", lineNum),
						Method:    "GET",
						Path:      basePath,
						Handler:   funcName,
						File:      filePath,
						Line:      lineNum,
						Framework: "django",
					}
					m.Endpoints = append(m.Endpoints, endpoint)
				}
			}
		}

		file.Close()
	}

	return nil
}

// parseURLPatterns extracts URL patterns from urls.py files
func (s *DjangoSupplement) parseURLPatterns(files []string) map[string]string {
	patterns := make(map[string]string)

	// URL pattern regex
	// path('users/', views.UserView.as_view())
	// path('api/items/', ItemViewSet.as_view({'get': 'list'}))
	// re_path(r'^users/$', views.users)
	pathPattern := regexp.MustCompile(`path\s*\(\s*['"]([^'"]+)['"]`)
	viewPattern := regexp.MustCompile(`(\w+)(?:\.as_view|\s*,)`)

	for _, f := range files {
		if !strings.HasSuffix(f, "urls.py") {
			continue
		}

		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			// Find path definition
			if pathMatch := pathPattern.FindStringSubmatch(line); len(pathMatch) >= 2 {
				path := pathMatch[1]
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}

				// Find view class/function
				if viewMatch := viewPattern.FindStringSubmatch(line); len(viewMatch) >= 2 {
					view := viewMatch[1]
					// Store the mapping
					patterns[view] = path
				}
			}
		}
	}

	return patterns
}

// parseMethods extracts HTTP methods from string like "'GET', 'POST'"
func (s *DjangoSupplement) parseMethods(str string) []string {
	var methods []string
	methodPattern := regexp.MustCompile(`['"](\w+)['"]`)
	matches := methodPattern.FindAllStringSubmatch(str, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			methods = append(methods, strings.ToUpper(m[1]))
		}
	}
	if len(methods) == 0 {
		return []string{"GET"}
	}
	return methods
}
