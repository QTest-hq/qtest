package model

import (
	"testing"
)

// =============================================================================
// Builder Tests
// =============================================================================

func TestNewBuilder(t *testing.T) {
	b := NewBuilder("https://github.com/test/repo", "main", "abc123")

	if b == nil {
		t.Fatal("NewBuilder() returned nil")
	}
	if b.model == nil {
		t.Fatal("model should not be nil")
	}
	if b.model.Repository != "https://github.com/test/repo" {
		t.Errorf("Repository = %s, want https://github.com/test/repo", b.model.Repository)
	}
	if b.model.Branch != "main" {
		t.Errorf("Branch = %s, want main", b.model.Branch)
	}
	if b.model.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %s, want abc123", b.model.CommitSHA)
	}
	if b.model.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if b.model.Modules == nil {
		t.Error("Modules should be initialized")
	}
	if b.model.Functions == nil {
		t.Error("Functions should be initialized")
	}
}

func TestBuilder_RegisterSupplement(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")
	supp := &mockSupplement{name: "test-supplement"}

	b.RegisterSupplement(supp)

	if len(b.supplements) != 1 {
		t.Errorf("len(supplements) = %d, want 1", len(b.supplements))
	}
	if b.supplements[0].Name() != "test-supplement" {
		t.Errorf("supplement name = %s, want test-supplement", b.supplements[0].Name())
	}
}

func TestBuilder_AddParsedFile_Function(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	functions := []ParsedFunction{
		{
			Name:       "GetUser",
			StartLine:  10,
			EndLine:    25,
			Parameters: []ParsedParam{{Name: "id", Type: "int"}},
			Returns:    []ParsedParam{{Type: "User"}},
			Exported:   true,
		},
	}

	b.AddParsedFile("src/user.go", "go", functions, nil)

	if len(b.model.Functions) != 1 {
		t.Fatalf("len(Functions) = %d, want 1", len(b.model.Functions))
	}

	fn := b.model.Functions[0]
	if fn.Name != "GetUser" {
		t.Errorf("Name = %s, want GetUser", fn.Name)
	}
	if fn.File != "src/user.go" {
		t.Errorf("File = %s, want src/user.go", fn.File)
	}
	if fn.StartLine != 10 {
		t.Errorf("StartLine = %d, want 10", fn.StartLine)
	}
	if fn.LOC != 16 { // 25 - 10 + 1
		t.Errorf("LOC = %d, want 16", fn.LOC)
	}
	if !fn.Exported {
		t.Error("Exported should be true")
	}
	if len(fn.Parameters) != 1 {
		t.Errorf("len(Parameters) = %d, want 1", len(fn.Parameters))
	}
}

func TestBuilder_AddParsedFile_Class(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	classes := []ParsedClass{
		{
			Name:      "UserService",
			StartLine: 5,
			EndLine:   50,
			Methods: []ParsedFunction{
				{Name: "GetUser", StartLine: 10, EndLine: 20, Exported: true},
				{Name: "CreateUser", StartLine: 22, EndLine: 35, Exported: true},
			},
			Properties: []ParsedProperty{
				{Name: "db", Type: "*DB", Exported: false},
			},
			Exported: true,
		},
	}

	b.AddParsedFile("src/service.go", "go", nil, classes)

	// Should have type and methods as functions
	if len(b.model.Types) != 1 {
		t.Fatalf("len(Types) = %d, want 1", len(b.model.Types))
	}
	if len(b.model.Functions) != 2 {
		t.Fatalf("len(Functions) = %d, want 2 (methods)", len(b.model.Functions))
	}

	typeDef := b.model.Types[0]
	if typeDef.Name != "UserService" {
		t.Errorf("Name = %s, want UserService", typeDef.Name)
	}
	if typeDef.Kind != TypeKindClass {
		t.Errorf("Kind = %s, want class", typeDef.Kind)
	}
	if len(typeDef.Fields) != 1 {
		t.Errorf("len(Fields) = %d, want 1", len(typeDef.Fields))
	}
	if len(typeDef.Methods) != 2 {
		t.Errorf("len(Methods) = %d, want 2", len(typeDef.Methods))
	}
}

func TestBuilder_AddParsedFile_Module(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	b.AddParsedFile("src/handlers/user.go", "go", []ParsedFunction{{Name: "Func1", StartLine: 1, EndLine: 5}}, nil)
	b.AddParsedFile("src/handlers/order.go", "go", []ParsedFunction{{Name: "Func2", StartLine: 1, EndLine: 5}}, nil)
	b.AddParsedFile("src/models/user.go", "go", []ParsedFunction{{Name: "Func3", StartLine: 1, EndLine: 5}}, nil)

	// Should have 2 modules: src/handlers and src/models
	if len(b.model.Modules) != 2 {
		t.Errorf("len(Modules) = %d, want 2", len(b.model.Modules))
	}

	// Find handlers module
	var handlersModule *Module
	for i := range b.model.Modules {
		if b.model.Modules[i].Name == "handlers" {
			handlersModule = &b.model.Modules[i]
			break
		}
	}

	if handlersModule == nil {
		t.Fatal("handlers module not found")
	}
	if len(handlersModule.Files) != 2 {
		t.Errorf("handlers module has %d files, want 2", len(handlersModule.Files))
	}
}

func TestBuilder_AddParsedFile_Languages(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	b.AddParsedFile("app.go", "go", []ParsedFunction{{Name: "F1", StartLine: 1, EndLine: 2}}, nil)
	b.AddParsedFile("app.py", "python", []ParsedFunction{{Name: "F2", StartLine: 1, EndLine: 2}}, nil)
	b.AddParsedFile("main.go", "go", []ParsedFunction{{Name: "F3", StartLine: 1, EndLine: 2}}, nil)

	if len(b.model.Languages) != 2 {
		t.Errorf("len(Languages) = %d, want 2", len(b.model.Languages))
	}

	// Should contain both go and python
	hasGo, hasPython := false, false
	for _, lang := range b.model.Languages {
		if lang == "go" {
			hasGo = true
		}
		if lang == "python" {
			hasPython = true
		}
	}
	if !hasGo || !hasPython {
		t.Errorf("Languages = %v, should contain both go and python", b.model.Languages)
	}
}

func TestBuilder_Build_Basic(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	b.AddParsedFile("main.go", "go", []ParsedFunction{
		{Name: "Main", StartLine: 1, EndLine: 10, Exported: true},
		{Name: "helper", StartLine: 12, EndLine: 20, Exported: false},
	}, nil)

	model, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if model == nil {
		t.Fatal("model should not be nil")
	}
	if len(model.RiskScores) != 2 {
		t.Errorf("len(RiskScores) = %d, want 2", len(model.RiskScores))
	}
	// Only exported functions become test targets
	if len(model.TestTargets) != 1 {
		t.Errorf("len(TestTargets) = %d, want 1 (only exported)", len(model.TestTargets))
	}
}

func TestBuilder_Build_WithSupplement(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	supp := &mockSupplement{
		name:       "test-supplement",
		shouldRun:  true,
		shouldFail: false,
	}
	b.RegisterSupplement(supp)

	b.AddParsedFile("main.go", "go", []ParsedFunction{{Name: "F1", StartLine: 1, EndLine: 2}}, nil)

	_, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if !supp.analyzed {
		t.Error("supplement should have been called")
	}
}

func TestBuilder_Build_SupplementFails(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	supp := &mockSupplement{
		name:       "failing-supplement",
		shouldRun:  true,
		shouldFail: true,
	}
	b.RegisterSupplement(supp)

	b.AddParsedFile("main.go", "go", []ParsedFunction{{Name: "F1", StartLine: 1, EndLine: 2}}, nil)

	_, err := b.Build()
	if err == nil {
		t.Error("Build() should return error when supplement fails")
	}
}

func TestBuilder_ComputeRiskScores_LOC(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	// Functions with different LOC values
	b.AddParsedFile("main.go", "go", []ParsedFunction{
		{Name: "Tiny", StartLine: 1, EndLine: 5, Exported: true},      // LOC=5
		{Name: "Small", StartLine: 10, EndLine: 20, Exported: true},   // LOC=11
		{Name: "Medium", StartLine: 30, EndLine: 55, Exported: true},  // LOC=26
		{Name: "Large", StartLine: 60, EndLine: 120, Exported: true},  // LOC=61
	}, nil)

	model, _ := b.Build()

	tests := []struct {
		name       string
		funcName   string
		wantLOC    int
		wantCmplx  float64
	}{
		{"tiny function", "Tiny", 5, 0.1},
		{"small function", "Small", 11, 0.3},
		{"medium function", "Medium", 26, 0.6},
		{"large function", "Large", 61, 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fn *Function
			for i := range model.Functions {
				if model.Functions[i].Name == tt.funcName {
					fn = &model.Functions[i]
					break
				}
			}
			if fn == nil {
				t.Fatalf("function %s not found", tt.funcName)
			}

			score := model.RiskScores[fn.ID]
			if score.Complexity != tt.wantCmplx {
				t.Errorf("Complexity = %f, want %f", score.Complexity, tt.wantCmplx)
			}
		})
	}
}

func TestBuilder_GenerateTestTargets_Endpoints(t *testing.T) {
	b := NewBuilder("repo", "main", "sha")

	// Add an endpoint to the model
	b.model.Endpoints = []Endpoint{
		{ID: "ep1", Method: "GET", Path: "/users"},
		{ID: "ep2", Method: "POST", Path: "/users"},
	}

	model, _ := b.Build()

	// API endpoints get priority
	apiTargets := 0
	for _, target := range model.TestTargets {
		if target.Kind == TargetKindAPI {
			apiTargets++
		}
	}

	if apiTargets != 2 {
		t.Errorf("apiTargets = %d, want 2", apiTargets)
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestConvertParameters(t *testing.T) {
	parsed := []ParsedParam{
		{Name: "id", Type: "int", Optional: false},
		{Name: "limit", Type: "int", Optional: true, Default: "10"},
	}

	result := convertParameters(parsed)

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}
	if result[0].Name != "id" {
		t.Errorf("result[0].Name = %s, want id", result[0].Name)
	}
	if result[1].Optional != true {
		t.Error("result[1].Optional should be true")
	}
	if result[1].Default != "10" {
		t.Errorf("result[1].Default = %s, want 10", result[1].Default)
	}
}

func TestConvertParameters_Empty(t *testing.T) {
	result := convertParameters(nil)
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}

	result = convertParameters([]ParsedParam{})
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestConvertFields(t *testing.T) {
	props := []ParsedProperty{
		{Name: "ID", Type: "int", Exported: true},
		{Name: "name", Type: "string", Exported: false},
	}

	result := convertFields(props)

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}
	if result[0].Name != "ID" {
		t.Errorf("result[0].Name = %s, want ID", result[0].Name)
	}
	if result[0].Exported != true {
		t.Error("result[0].Exported should be true")
	}
	if result[1].Exported != false {
		t.Error("result[1].Exported should be false")
	}
}

func TestContainsAny(t *testing.T) {
	files := []string{
		"src/routes.js",
		"src/app.ts",
		"package.json",
	}

	tests := []struct {
		name     string
		patterns []string
		want     bool
	}{
		{"match single", []string{".js"}, true},
		{"match multiple", []string{".py", ".ts"}, true},
		{"no match", []string{".go", ".java"}, false},
		{"empty patterns", []string{}, false},
		{"exact match", []string{"package.json"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsAny(files, tt.patterns)
			if got != tt.want {
				t.Errorf("containsAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsAny_EmptyFiles(t *testing.T) {
	got := containsAny([]string{}, []string{".js"})
	if got != false {
		t.Error("containsAny with empty files should return false")
	}
}

// =============================================================================
// Mock Supplement
// =============================================================================

type mockSupplement struct {
	name       string
	shouldRun  bool
	shouldFail bool
	analyzed   bool
}

func (m *mockSupplement) Name() string {
	return m.name
}

func (m *mockSupplement) Detect(files []string) bool {
	return m.shouldRun
}

func (m *mockSupplement) Analyze(model *SystemModel) error {
	m.analyzed = true
	if m.shouldFail {
		return errMock
	}
	return nil
}

var errMock = &mockError{}

type mockError struct{}

func (e *mockError) Error() string { return "mock error" }
