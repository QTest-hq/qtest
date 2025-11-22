package adapters

import (
	"github.com/QTest-hq/qtest/pkg/dsl"
	"github.com/QTest-hq/qtest/pkg/model"
)

// Framework represents a testing framework
type Framework string

const (
	FrameworkGoTest Framework = "go"
	FrameworkJest   Framework = "jest"
	FrameworkPytest Framework = "pytest"
	FrameworkJUnit  Framework = "junit"
)

// Adapter converts DSL tests to framework-specific code
type Adapter interface {
	Framework() Framework
	Generate(test *dsl.TestDSL) (string, error)
	FileExtension() string
	TestFileSuffix() string
}

// SpecAdapter converts TestSpecs (from IRSpec) to framework-specific code
// This is the newer, richer interface that supports type hints and structured assertions
type SpecAdapter interface {
	Framework() Framework
	GenerateFromSpecs(specs []model.TestSpec, sourceFile string) (string, error)
	FileExtension() string
	TestFileSuffix() string
}

// GeneratedCode represents generated test code
type GeneratedCode struct {
	Framework Framework
	Code      string
	FileName  string
	Imports   []string
}
