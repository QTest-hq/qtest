package model

import (
	"testing"
)

// =============================================================================
// ParserAdapter Tests
// =============================================================================

func TestNewParserAdapter(t *testing.T) {
	adapter := NewParserAdapter("https://github.com/test/repo", "main", "abc123")

	if adapter == nil {
		t.Fatal("NewParserAdapter() returned nil")
	}
	if adapter.builder == nil {
		t.Fatal("builder should not be nil")
	}
}

func TestParserAdapter_RegisterSupplement(t *testing.T) {
	adapter := NewParserAdapter("repo", "main", "sha")
	supp := &mockSupplement{name: "test"}

	adapter.RegisterSupplement(supp)

	if len(adapter.builder.supplements) != 1 {
		t.Errorf("len(supplements) = %d, want 1", len(adapter.builder.supplements))
	}
}

func TestParserAdapter_AddFile_Functions(t *testing.T) {
	adapter := NewParserAdapter("repo", "main", "sha")

	pf := &ParsedFile{
		Path:     "src/main.go",
		Language: "go",
		Functions: []ParserFunction{
			{
				Name:       "GetUser",
				StartLine:  10,
				EndLine:    25,
				ReturnType: "User",
				Parameters: []ParserParameter{
					{Name: "id", Type: "int", Optional: false},
				},
				Exported: true,
				Async:    false,
				Body:     "func body",
				Comments: "// GetUser retrieves a user",
			},
		},
	}

	adapter.AddFile(pf)

	model, err := adapter.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(model.Functions) != 1 {
		t.Fatalf("len(Functions) = %d, want 1", len(model.Functions))
	}

	fn := model.Functions[0]
	if fn.Name != "GetUser" {
		t.Errorf("Name = %s, want GetUser", fn.Name)
	}
	if fn.File != "src/main.go" {
		t.Errorf("File = %s, want src/main.go", fn.File)
	}
	if fn.DocComment != "// GetUser retrieves a user" {
		t.Errorf("DocComment mismatch")
	}
	if len(fn.Parameters) != 1 {
		t.Errorf("len(Parameters) = %d, want 1", len(fn.Parameters))
	}
	if len(fn.Returns) != 1 {
		t.Errorf("len(Returns) = %d, want 1", len(fn.Returns))
	}
}

func TestParserAdapter_AddFile_Classes(t *testing.T) {
	adapter := NewParserAdapter("repo", "main", "sha")

	pf := &ParsedFile{
		Path:     "src/service.py",
		Language: "python",
		Classes: []ParserClass{
			{
				Name:      "UserService",
				StartLine: 5,
				EndLine:   50,
				Methods: []ParserFunction{
					{Name: "get_user", StartLine: 10, EndLine: 20, Exported: true},
					{Name: "_helper", StartLine: 22, EndLine: 30, Exported: false},
				},
				Properties: []ParserProperty{
					{Name: "db", Type: "Database", Exported: false},
				},
				Extends:  "BaseService",
				Exported: true,
			},
		},
	}

	adapter.AddFile(pf)

	model, err := adapter.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(model.Types) != 1 {
		t.Fatalf("len(Types) = %d, want 1", len(model.Types))
	}
	if len(model.Functions) != 2 {
		t.Fatalf("len(Functions) = %d, want 2", len(model.Functions))
	}

	typeDef := model.Types[0]
	if typeDef.Name != "UserService" {
		t.Errorf("Name = %s, want UserService", typeDef.Name)
	}
	if typeDef.Extends != "BaseService" {
		t.Errorf("Extends = %s, want BaseService", typeDef.Extends)
	}
}

func TestParserAdapter_AddFile_NoReturnType(t *testing.T) {
	adapter := NewParserAdapter("repo", "main", "sha")

	pf := &ParsedFile{
		Path:     "main.go",
		Language: "go",
		Functions: []ParserFunction{
			{
				Name:       "DoSomething",
				StartLine:  1,
				EndLine:    5,
				ReturnType: "", // No return type
				Exported:   true,
			},
		},
	}

	adapter.AddFile(pf)

	model, _ := adapter.Build()

	if len(model.Functions[0].Returns) != 0 {
		t.Errorf("len(Returns) = %d, want 0 for no return type", len(model.Functions[0].Returns))
	}
}

func TestParserAdapter_Build(t *testing.T) {
	adapter := NewParserAdapter("repo", "main", "sha")

	pf := &ParsedFile{
		Path:     "main.go",
		Language: "go",
		Functions: []ParserFunction{
			{Name: "Main", StartLine: 1, EndLine: 10, Exported: true},
		},
	}
	adapter.AddFile(pf)

	model, err := adapter.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if model == nil {
		t.Fatal("model should not be nil")
	}
	if model.Repository != "repo" {
		t.Errorf("Repository = %s, want repo", model.Repository)
	}
}

func TestParserAdapter_MultipleFiles(t *testing.T) {
	adapter := NewParserAdapter("repo", "main", "sha")

	adapter.AddFile(&ParsedFile{
		Path:      "src/user.go",
		Language:  "go",
		Functions: []ParserFunction{{Name: "GetUser", StartLine: 1, EndLine: 10}},
	})
	adapter.AddFile(&ParsedFile{
		Path:      "src/order.go",
		Language:  "go",
		Functions: []ParserFunction{{Name: "GetOrder", StartLine: 1, EndLine: 10}},
	})
	adapter.AddFile(&ParsedFile{
		Path:      "utils/helper.py",
		Language:  "python",
		Functions: []ParserFunction{{Name: "helper", StartLine: 1, EndLine: 5}},
	})

	model, err := adapter.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(model.Functions) != 3 {
		t.Errorf("len(Functions) = %d, want 3", len(model.Functions))
	}
	if len(model.Languages) != 2 {
		t.Errorf("len(Languages) = %d, want 2", len(model.Languages))
	}
}

// =============================================================================
// isSourceFile Tests
// =============================================================================

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".go", true},
		{".py", true},
		{".js", true},
		{".jsx", true},
		{".ts", true},
		{".tsx", true},
		{".java", true},
		{".txt", false},
		{".md", false},
		{".json", false},
		{".yaml", false},
		{".css", false},
		{".html", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := isSourceFile(tt.ext)
			if got != tt.want {
				t.Errorf("isSourceFile(%s) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ParsedFile Struct Tests
// =============================================================================

func TestParsedFile_Fields(t *testing.T) {
	pf := ParsedFile{
		Path:     "src/main.go",
		Language: "go",
		Functions: []ParserFunction{
			{ID: "fn1", Name: "Main"},
		},
		Classes: []ParserClass{
			{ID: "cls1", Name: "Service"},
		},
		Imports: []ParserImport{
			{Module: "fmt"},
		},
	}

	if pf.Path != "src/main.go" {
		t.Errorf("Path = %s, want src/main.go", pf.Path)
	}
	if pf.Language != "go" {
		t.Errorf("Language = %s, want go", pf.Language)
	}
	if len(pf.Functions) != 1 {
		t.Errorf("len(Functions) = %d, want 1", len(pf.Functions))
	}
	if len(pf.Classes) != 1 {
		t.Errorf("len(Classes) = %d, want 1", len(pf.Classes))
	}
	if len(pf.Imports) != 1 {
		t.Errorf("len(Imports) = %d, want 1", len(pf.Imports))
	}
}

func TestParserFunction_Fields(t *testing.T) {
	fn := ParserFunction{
		ID:         "fn:main:GetUser",
		Name:       "GetUser",
		StartLine:  10,
		EndLine:    25,
		Parameters: []ParserParameter{{Name: "id", Type: "int"}},
		ReturnType: "*User",
		Body:       "return nil",
		Comments:   "// Gets user by ID",
		Exported:   true,
		Async:      true,
		Class:      "UserService",
	}

	if fn.Name != "GetUser" {
		t.Errorf("Name = %s, want GetUser", fn.Name)
	}
	if fn.Async != true {
		t.Error("Async should be true")
	}
	if fn.Class != "UserService" {
		t.Errorf("Class = %s, want UserService", fn.Class)
	}
}

func TestParserClass_Fields(t *testing.T) {
	cls := ParserClass{
		ID:        "cls:UserService",
		Name:      "UserService",
		StartLine: 5,
		EndLine:   100,
		Methods: []ParserFunction{
			{Name: "GetUser"},
		},
		Properties: []ParserProperty{
			{Name: "db", Type: "*DB"},
		},
		Comments:   "// UserService handles users",
		Exported:   true,
		Extends:    "BaseService",
		Implements: []string{"IUserService"},
	}

	if cls.Name != "UserService" {
		t.Errorf("Name = %s, want UserService", cls.Name)
	}
	if cls.Extends != "BaseService" {
		t.Errorf("Extends = %s, want BaseService", cls.Extends)
	}
	if len(cls.Implements) != 1 {
		t.Errorf("len(Implements) = %d, want 1", len(cls.Implements))
	}
}

func TestParserParameter_Fields(t *testing.T) {
	param := ParserParameter{
		Name:     "limit",
		Type:     "int",
		Default:  "10",
		Optional: true,
	}

	if param.Name != "limit" {
		t.Errorf("Name = %s, want limit", param.Name)
	}
	if param.Default != "10" {
		t.Errorf("Default = %s, want 10", param.Default)
	}
	if !param.Optional {
		t.Error("Optional should be true")
	}
}

func TestParserProperty_Fields(t *testing.T) {
	prop := ParserProperty{
		Name:     "ID",
		Type:     "int64",
		Exported: true,
	}

	if prop.Name != "ID" {
		t.Errorf("Name = %s, want ID", prop.Name)
	}
	if !prop.Exported {
		t.Error("Exported should be true")
	}
}

func TestParserImport_Fields(t *testing.T) {
	imp := ParserImport{
		Module: "github.com/test/pkg",
		Names:  []string{"Func1", "Func2"},
		Alias:  "pkg",
	}

	if imp.Module != "github.com/test/pkg" {
		t.Errorf("Module = %s, want github.com/test/pkg", imp.Module)
	}
	if len(imp.Names) != 2 {
		t.Errorf("len(Names) = %d, want 2", len(imp.Names))
	}
	if imp.Alias != "pkg" {
		t.Errorf("Alias = %s, want pkg", imp.Alias)
	}
}
