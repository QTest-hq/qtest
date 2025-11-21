package parser

// Language represents a programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageJava       Language = "java"
	LanguageUnknown    Language = "unknown"
)

// ParsedFile represents a parsed source file
type ParsedFile struct {
	Path      string
	Language  Language
	Functions []Function
	Classes   []Class
	Imports   []Import
	Exports   []Export
}

// Function represents a parsed function
type Function struct {
	ID          string      // Unique identifier: file:line:name
	Name        string
	StartLine   int
	EndLine     int
	Parameters  []Parameter
	ReturnType  string
	Body        string      // Full function body
	Comments    string      // Doc comments
	Exported    bool        // Is publicly accessible
	Async       bool        // Is async function
	Class       string      // Parent class (if method)
}

// Class represents a parsed class
type Class struct {
	ID         string
	Name       string
	StartLine  int
	EndLine    int
	Methods    []Function
	Properties []Property
	Comments   string
	Exported   bool
	Extends    string   // Parent class
	Implements []string // Interfaces
}

// Property represents a class property
type Property struct {
	Name     string
	Type     string
	Exported bool
}

// Parameter represents a function parameter
type Parameter struct {
	Name     string
	Type     string
	Default  string // Default value if any
	Optional bool
}

// Import represents an import statement
type Import struct {
	Module string
	Names  []string // Specific imports
	Alias  string   // Import alias
}

// Export represents an export statement
type Export struct {
	Name    string
	Kind    string // function, class, const, etc.
	Default bool
}

// SystemModel represents the parsed system model
type SystemModel struct {
	Repository  string
	CommitSHA   string
	Language    Language // Primary language
	Files       []ParsedFile
	Endpoints   []Endpoint
	Dependencies []Dependency
}

// Endpoint represents an API endpoint
type Endpoint struct {
	Method   string // GET, POST, etc.
	Path     string
	Handler  string // Function that handles this endpoint
	File     string
	Line     int
}

// Dependency represents a project dependency
type Dependency struct {
	Name    string
	Version string
	Type    string // runtime, dev, peer
}
