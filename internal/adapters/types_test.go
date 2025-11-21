package adapters

import (
	"testing"
)

func TestFrameworkConstants(t *testing.T) {
	if FrameworkGoTest != "go" {
		t.Errorf("FrameworkGoTest = %s, want go", FrameworkGoTest)
	}
	if FrameworkJest != "jest" {
		t.Errorf("FrameworkJest = %s, want jest", FrameworkJest)
	}
	if FrameworkPytest != "pytest" {
		t.Errorf("FrameworkPytest = %s, want pytest", FrameworkPytest)
	}
	if FrameworkJUnit != "junit" {
		t.Errorf("FrameworkJUnit = %s, want junit", FrameworkJUnit)
	}
}

func TestGeneratedCode_Fields(t *testing.T) {
	gc := GeneratedCode{
		Framework: FrameworkGoTest,
		Code:      "package test",
		FileName:  "test_file.go",
		Imports:   []string{"testing", "fmt"},
	}

	if gc.Framework != FrameworkGoTest {
		t.Errorf("Framework = %s, want go", gc.Framework)
	}
	if gc.Code != "package test" {
		t.Errorf("Code = %s, want 'package test'", gc.Code)
	}
	if gc.FileName != "test_file.go" {
		t.Errorf("FileName = %s, want test_file.go", gc.FileName)
	}
	if len(gc.Imports) != 2 {
		t.Errorf("len(Imports) = %d, want 2", len(gc.Imports))
	}
}

func TestGeneratedCode_EmptyFields(t *testing.T) {
	gc := GeneratedCode{}

	if gc.Framework != "" {
		t.Errorf("default Framework = %s, want empty", gc.Framework)
	}
	if gc.Code != "" {
		t.Errorf("default Code = %s, want empty", gc.Code)
	}
	if gc.FileName != "" {
		t.Errorf("default FileName = %s, want empty", gc.FileName)
	}
	if gc.Imports != nil {
		t.Error("default Imports should be nil")
	}
}

func TestFramework_TypeConversion(t *testing.T) {
	// Framework is a string type alias
	f := Framework("custom")
	if string(f) != "custom" {
		t.Errorf("Framework string conversion failed")
	}
}
