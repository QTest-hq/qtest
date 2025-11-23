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
	ID         string // Unique identifier: file:line:name
	Name       string
	StartLine  int
	EndLine    int
	Parameters []Parameter
	ReturnType string
	Body       string // Full function body
	Comments   string // Doc comments
	Exported   bool   // Is publicly accessible
	Async      bool   // Is async function
	Static     bool   // Is static method (Java)
	Class      string // Parent class (if method)

	// Branch and call site analysis (P1-036, P1-037)
	Branches             []Branch   // Control flow branches (if/switch/try/loops)
	CallSites            []CallSite // Function calls within this function
	CyclomaticComplexity int        // Computed: len(Branches) + 1
}

// BranchType represents the type of control flow branch
type BranchType string

const (
	BranchIf      BranchType = "if"
	BranchElseIf  BranchType = "else_if"
	BranchSwitch  BranchType = "switch"
	BranchCase    BranchType = "case"
	BranchTry     BranchType = "try"
	BranchCatch   BranchType = "catch"
	BranchFor     BranchType = "for"
	BranchWhile   BranchType = "while"
	BranchDoWhile BranchType = "do_while"
)

// Branch represents a control flow branch point
type Branch struct {
	Type      BranchType // Type of branch (if, switch, try, for, while, etc.)
	StartLine int        // Line where branch starts
	EndLine   int        // Line where branch ends
	Condition string     // The condition expression (if applicable)
	IsLoop    bool       // Whether this is a loop construct
}

// CallSite represents a function call within a function
type CallSite struct {
	FunctionName string // Name of the called function
	Module       string // Module/package where function is defined (if known)
	Receiver     string // Receiver object for method calls (e.g., "obj" in obj.method())
	Line         int    // Line where call occurs
	Arguments    int    // Number of arguments
	IsAsync      bool   // Whether this is an async call (await)
	IsMethod     bool   // Whether this is a method call (has receiver)
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
	Repository   string
	CommitSHA    string
	Language     Language // Primary language
	Files        []ParsedFile
	Endpoints    []Endpoint
	Dependencies []Dependency
}

// Endpoint represents an API endpoint
type Endpoint struct {
	Method  string // GET, POST, etc.
	Path    string
	Handler string // Function that handles this endpoint
	File    string
	Line    int
}

// Dependency represents a project dependency
type Dependency struct {
	Name    string
	Version string
	Type    string // runtime, dev, peer
}
