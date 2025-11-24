package main

import (
	"testing"
)

// =============================================================================
// Auth Command Tests
// =============================================================================

func TestAuthCmd(t *testing.T) {
	cmd := authCmd()

	if cmd.Use != "auth" {
		t.Errorf("expected Use 'auth', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"login", "logout", "status"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

func TestAuthLoginCmd(t *testing.T) {
	cmd := authLoginCmd()

	if cmd.Use != "login" {
		t.Errorf("expected Use 'login', got '%s'", cmd.Use)
	}

	// Check flags
	tokenFlag := cmd.Flags().Lookup("token")
	if tokenFlag == nil {
		t.Error("expected --token flag")
	}

	serverFlag := cmd.Flags().Lookup("server")
	if serverFlag == nil {
		t.Error("expected --server flag")
	}
}

func TestAuthLogoutCmd(t *testing.T) {
	cmd := authLogoutCmd()

	if cmd.Use != "logout" {
		t.Errorf("expected Use 'logout', got '%s'", cmd.Use)
	}
}

func TestAuthStatusCmd(t *testing.T) {
	cmd := authStatusCmd()

	if cmd.Use != "status" {
		t.Errorf("expected Use 'status', got '%s'", cmd.Use)
	}
}

// =============================================================================
// CI Command Tests
// =============================================================================

func TestCICmd(t *testing.T) {
	cmd := ciCmd()

	if cmd.Use != "ci" {
		t.Errorf("expected Use 'ci', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"generate", "detect", "preview", "list"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

func TestCIGenerateCmd(t *testing.T) {
	cmd := ciGenerateCmd()

	if cmd.Use != "generate" {
		t.Errorf("expected Use 'generate', got '%s'", cmd.Use)
	}

	// Check flags
	dirFlag := cmd.Flags().Lookup("dir")
	if dirFlag == nil {
		t.Error("expected --dir flag")
	}
	if dirFlag.DefValue != "." {
		t.Errorf("expected dir default '.', got '%s'", dirFlag.DefValue)
	}

	platformFlag := cmd.Flags().Lookup("platform")
	if platformFlag == nil {
		t.Error("expected --platform flag")
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected --output flag")
	}

	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("expected --force flag")
	}
}

func TestCIDetectCmd(t *testing.T) {
	cmd := ciDetectCmd()

	if cmd.Use != "detect" {
		t.Errorf("expected Use 'detect', got '%s'", cmd.Use)
	}

	// Check flags
	dirFlag := cmd.Flags().Lookup("dir")
	if dirFlag == nil {
		t.Error("expected --dir flag")
	}
}

func TestCIPreviewCmd(t *testing.T) {
	cmd := ciPreviewCmd()

	if cmd.Use != "preview" {
		t.Errorf("expected Use 'preview', got '%s'", cmd.Use)
	}

	// Check flags
	dirFlag := cmd.Flags().Lookup("dir")
	if dirFlag == nil {
		t.Error("expected --dir flag")
	}

	platformFlag := cmd.Flags().Lookup("platform")
	if platformFlag == nil {
		t.Error("expected --platform flag")
	}
}

func TestCIListCmd(t *testing.T) {
	cmd := ciListCmd()

	if cmd.Use != "list" {
		t.Errorf("expected Use 'list', got '%s'", cmd.Use)
	}
}

// =============================================================================
// Contract Command Tests
// =============================================================================

func TestContractCmd(t *testing.T) {
	cmd := contractCmd()

	if cmd.Use != "contract" {
		t.Errorf("expected Use 'contract', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"generate", "validate", "tests"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

func TestContractGenerateCmd(t *testing.T) {
	cmd := contractGenerateCmd()

	if cmd.Use != "generate" {
		t.Errorf("expected Use 'generate', got '%s'", cmd.Use)
	}

	// Check flags
	modelFlag := cmd.Flags().Lookup("model")
	if modelFlag == nil {
		t.Error("expected --model flag")
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected --output flag")
	}
}

func TestContractValidateCmd(t *testing.T) {
	cmd := contractValidateCmd()

	if cmd.Use != "validate" {
		t.Errorf("expected Use 'validate', got '%s'", cmd.Use)
	}

	// Check flags
	contractFlag := cmd.Flags().Lookup("contract")
	if contractFlag == nil {
		t.Error("expected --contract flag")
	}

	urlFlag := cmd.Flags().Lookup("url")
	if urlFlag == nil {
		t.Error("expected --url flag")
	}
}

func TestContractTestsCmd(t *testing.T) {
	cmd := contractTestsCmd()

	if cmd.Use != "tests" {
		t.Errorf("expected Use 'tests', got '%s'", cmd.Use)
	}

	// Check flags
	contractFlag := cmd.Flags().Lookup("contract")
	if contractFlag == nil {
		t.Error("expected --contract flag")
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected --output flag")
	}
}

// =============================================================================
// Datagen Command Tests
// =============================================================================

func TestDatagenCmd(t *testing.T) {
	cmd := datagenCmd()

	if cmd.Use != "datagen" {
		t.Errorf("expected Use 'datagen', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"sample", "schema", "field"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

func TestDatagenSampleCmd(t *testing.T) {
	cmd := datagenSampleCmd()

	if cmd.Use != "sample" {
		t.Errorf("expected Use 'sample', got '%s'", cmd.Use)
	}

	// Check flags
	countFlag := cmd.Flags().Lookup("count")
	if countFlag == nil {
		t.Error("expected --count flag")
	}
}

func TestDatagenSchemaCmd(t *testing.T) {
	cmd := datagenSchemaCmd()

	if cmd.Use != "schema" {
		t.Errorf("expected Use 'schema', got '%s'", cmd.Use)
	}

	// Check flags
	schemaFlag := cmd.Flags().Lookup("schema")
	if schemaFlag == nil {
		t.Error("expected --schema flag")
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected --output flag")
	}
}

func TestDatagenFieldCmd(t *testing.T) {
	cmd := datagenFieldCmd()

	if cmd.Use != "field <field-name> [type]" {
		t.Errorf("expected Use 'field <field-name> [type]', got '%s'", cmd.Use)
	}

	// This command takes arguments, not flags
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

// =============================================================================
// Validate Command Tests
// =============================================================================

func TestValidateCmd(t *testing.T) {
	cmd := validateCmd()

	if cmd.Use != "validate" {
		t.Errorf("expected Use 'validate', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"run", "fix"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

func TestValidateRunCmd(t *testing.T) {
	cmd := validateRunCmd()

	if cmd.Use != "run <test-file>" {
		t.Errorf("expected Use 'run <test-file>', got '%s'", cmd.Use)
	}

	// Check flags
	languageFlag := cmd.Flags().Lookup("language")
	if languageFlag == nil {
		t.Error("expected --language flag")
	}
}

func TestValidateFixCmd(t *testing.T) {
	cmd := validateFixCmd()

	if cmd.Use != "fix <test-file>" {
		t.Errorf("expected Use 'fix <test-file>', got '%s'", cmd.Use)
	}

	// Check flags
	languageFlag := cmd.Flags().Lookup("language")
	if languageFlag == nil {
		t.Error("expected --language flag")
	}

	tierFlag := cmd.Flags().Lookup("tier")
	if tierFlag == nil {
		t.Error("expected --tier flag")
	}

	retriesFlag := cmd.Flags().Lookup("retries")
	if retriesFlag == nil {
		t.Error("expected --retries flag")
	}
}

// =============================================================================
// Mutation Command Tests
// =============================================================================

func TestMutationCmd(t *testing.T) {
	cmd := mutationCmd()

	if cmd.Use != "mutation" {
		t.Errorf("expected Use 'mutation', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands exist
	subCmds := cmd.Commands()
	if len(subCmds) == 0 {
		t.Error("mutation command should have subcommands")
	}
}

// =============================================================================
// Model Command Tests
// =============================================================================

func TestModelCmd(t *testing.T) {
	cmd := modelCmd()

	if cmd.Use != "model" {
		t.Errorf("expected Use 'model', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands exist
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"build", "show"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

// =============================================================================
// Plan Command Tests
// =============================================================================

func TestPlanCmd(t *testing.T) {
	cmd := planCmd()

	if cmd.Use != "plan" {
		t.Errorf("expected Use 'plan', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// plan is a parent command with subcommands
	subCmds := cmd.Commands()
	if len(subCmds) == 0 {
		t.Error("plan command should have subcommands")
	}
}

// =============================================================================
// PR Command Tests
// =============================================================================

func TestPRCmd(t *testing.T) {
	cmd := prCmd()

	if cmd.Use != "pr" {
		t.Errorf("expected Use 'pr', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"create"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

// =============================================================================
// Emit Command Tests
// =============================================================================

func TestEmitTestsCmd(t *testing.T) {
	cmd := emitTestsCmd()

	if cmd.Use != "emit-tests" {
		t.Errorf("expected Use 'emit-tests', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check flags
	specsFlag := cmd.Flags().Lookup("specs")
	if specsFlag == nil {
		t.Error("expected --specs flag")
	}

	emitterFlag := cmd.Flags().Lookup("emitter")
	if emitterFlag == nil {
		t.Error("expected --emitter flag")
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected --output flag")
	}
}

// =============================================================================
// Status Command Tests
// =============================================================================

func TestStatusCmd(t *testing.T) {
	cmd := statusCmd()

	if cmd.Use != "status" {
		t.Errorf("expected Use 'status', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

// =============================================================================
// Job Command Tests
// =============================================================================

func TestJobCmd(t *testing.T) {
	cmd := jobCmd()

	if cmd.Use != "job" {
		t.Errorf("expected Use 'job', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	if len(subCmds) == 0 {
		t.Error("job command should have subcommands")
	}
}

// =============================================================================
// Coverage Command Tests
// =============================================================================

func TestCoverageCmd(t *testing.T) {
	cmd := coverageCmd()

	if cmd.Use != "coverage" {
		t.Errorf("expected Use 'coverage', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"collect", "analyze", "generate"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

// =============================================================================
// Workspace Command Tests (additional)
// =============================================================================

func TestWorkspaceCmd(t *testing.T) {
	cmd := workspaceCmd()

	if cmd.Use != "workspace" {
		t.Errorf("expected Use 'workspace', got '%s'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Check subcommands
	subCmds := cmd.Commands()
	expectedSubCmds := []string{"init", "list", "status", "run"}

	for _, expected := range expectedSubCmds {
		found := false
		for _, sub := range subCmds {
			if sub.Name() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' not found", expected)
		}
	}
}

