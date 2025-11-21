package adapters

import "github.com/QTest-hq/qtest/pkg/dsl"

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

// GeneratedCode represents generated test code
type GeneratedCode struct {
	Framework Framework
	Code      string
	FileName  string
	Imports   []string
}
