package contract

import (
	"net/http"
	"testing"

	"github.com/QTest-hq/qtest/pkg/model"
)

func TestContract_Fields(t *testing.T) {
	c := Contract{
		Name:     "Test API",
		Version:  "1.0.0",
		Provider: "test-provider",
		Consumer: "test-consumer",
		Endpoints: []EndpointContract{
			{ID: "ep1", Method: "GET", Path: "/users"},
		},
	}

	if c.Name != "Test API" {
		t.Errorf("Name = %s, want Test API", c.Name)
	}
	if c.Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", c.Version)
	}
	if c.Provider != "test-provider" {
		t.Errorf("Provider = %s, want test-provider", c.Provider)
	}
	if c.Consumer != "test-consumer" {
		t.Errorf("Consumer = %s, want test-consumer", c.Consumer)
	}
	if len(c.Endpoints) != 1 {
		t.Errorf("len(Endpoints) = %d, want 1", len(c.Endpoints))
	}
}

func TestEndpointContract_Fields(t *testing.T) {
	ec := EndpointContract{
		ID:          "ep1",
		Method:      "POST",
		Path:        "/users",
		Description: "Create user",
		Request: RequestContract{
			ContentType: "application/json",
		},
		Response: ResponseContract{
			StatusCode: 201,
		},
		Examples: []InteractionExample{
			{Name: "Create user example"},
		},
	}

	if ec.ID != "ep1" {
		t.Errorf("ID = %s, want ep1", ec.ID)
	}
	if ec.Method != "POST" {
		t.Errorf("Method = %s, want POST", ec.Method)
	}
	if ec.Path != "/users" {
		t.Errorf("Path = %s, want /users", ec.Path)
	}
	if ec.Description != "Create user" {
		t.Errorf("Description = %s, want Create user", ec.Description)
	}
	if ec.Request.ContentType != "application/json" {
		t.Errorf("Request.ContentType = %s, want application/json", ec.Request.ContentType)
	}
	if ec.Response.StatusCode != 201 {
		t.Errorf("Response.StatusCode = %d, want 201", ec.Response.StatusCode)
	}
}

func TestRequestContract_Fields(t *testing.T) {
	rc := RequestContract{
		Headers:     map[string]HeaderSpec{"Authorization": {Required: true}},
		Query:       map[string]ParamSpec{"limit": {Type: "integer", Required: false}},
		PathParams:  map[string]ParamSpec{"id": {Type: "string", Required: true}},
		Body:        &SchemaSpec{Type: "object"},
		ContentType: "application/json",
	}

	if len(rc.Headers) != 1 {
		t.Errorf("len(Headers) = %d, want 1", len(rc.Headers))
	}
	if !rc.Headers["Authorization"].Required {
		t.Error("Authorization header should be required")
	}
	if len(rc.Query) != 1 {
		t.Errorf("len(Query) = %d, want 1", len(rc.Query))
	}
	if len(rc.PathParams) != 1 {
		t.Errorf("len(PathParams) = %d, want 1", len(rc.PathParams))
	}
	if rc.Body == nil {
		t.Error("Body should not be nil")
	}
	if rc.ContentType != "application/json" {
		t.Errorf("ContentType = %s, want application/json", rc.ContentType)
	}
}

func TestResponseContract_Fields(t *testing.T) {
	rc := ResponseContract{
		StatusCode:  200,
		Headers:     map[string]HeaderSpec{"X-Request-ID": {Required: true}},
		Body:        &SchemaSpec{Type: "object"},
		ContentType: "application/json",
	}

	if rc.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", rc.StatusCode)
	}
	if len(rc.Headers) != 1 {
		t.Errorf("len(Headers) = %d, want 1", len(rc.Headers))
	}
	if rc.Body == nil {
		t.Error("Body should not be nil")
	}
	if rc.ContentType != "application/json" {
		t.Errorf("ContentType = %s, want application/json", rc.ContentType)
	}
}

func TestHeaderSpec_Fields(t *testing.T) {
	hs := HeaderSpec{
		Required: true,
		Values:   []string{"Bearer", "Basic"},
	}

	if !hs.Required {
		t.Error("Required should be true")
	}
	if len(hs.Values) != 2 {
		t.Errorf("len(Values) = %d, want 2", len(hs.Values))
	}
}

func TestParamSpec_Fields(t *testing.T) {
	ps := ParamSpec{
		Type:     "string",
		Required: true,
		Format:   "uuid",
		Values:   []string{"active", "inactive"},
	}

	if ps.Type != "string" {
		t.Errorf("Type = %s, want string", ps.Type)
	}
	if !ps.Required {
		t.Error("Required should be true")
	}
	if ps.Format != "uuid" {
		t.Errorf("Format = %s, want uuid", ps.Format)
	}
	if len(ps.Values) != 2 {
		t.Errorf("len(Values) = %d, want 2", len(ps.Values))
	}
}

func TestSchemaSpec_Fields(t *testing.T) {
	ss := SchemaSpec{
		Type: "object",
		Properties: map[string]*SchemaSpec{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Items:    &SchemaSpec{Type: "string"},
		Required: []string{"name"},
		Format:   "email",
		Enum:     []interface{}{"active", "inactive"},
	}

	if ss.Type != "object" {
		t.Errorf("Type = %s, want object", ss.Type)
	}
	if len(ss.Properties) != 2 {
		t.Errorf("len(Properties) = %d, want 2", len(ss.Properties))
	}
	if ss.Items == nil {
		t.Error("Items should not be nil")
	}
	if len(ss.Required) != 1 {
		t.Errorf("len(Required) = %d, want 1", len(ss.Required))
	}
	if ss.Format != "email" {
		t.Errorf("Format = %s, want email", ss.Format)
	}
	if len(ss.Enum) != 2 {
		t.Errorf("len(Enum) = %d, want 2", len(ss.Enum))
	}
}

func TestInteractionExample_Fields(t *testing.T) {
	ie := InteractionExample{
		Name:     "Create user example",
		Request:  map[string]interface{}{"name": "John"},
		Response: map[string]interface{}{"id": 1, "name": "John"},
	}

	if ie.Name != "Create user example" {
		t.Errorf("Name = %s, want Create user example", ie.Name)
	}
	if ie.Request["name"] != "John" {
		t.Error("Request name mismatch")
	}
	if ie.Response["id"] != 1 {
		t.Error("Response id mismatch")
	}
}

func TestNewContractGenerator(t *testing.T) {
	cg := NewContractGenerator()

	if cg == nil {
		t.Fatal("NewContractGenerator() returned nil")
	}
}

func TestGenerateFromModel(t *testing.T) {
	sysModel := &model.SystemModel{
		Repository: "test-repo",
		Endpoints: []model.Endpoint{
			{ID: "ep1", Method: "GET", Path: "/users", Handler: "ListUsers"},
			{ID: "ep2", Method: "POST", Path: "/users", Handler: "CreateUser"},
			{ID: "ep3", Method: "GET", Path: "/users/:id", Handler: "GetUser"},
		},
	}

	cg := NewContractGenerator()
	contract := cg.GenerateFromModel(sysModel)

	if contract.Name != "test-repo API Contract" {
		t.Errorf("Name = %s, want test-repo API Contract", contract.Name)
	}
	if contract.Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", contract.Version)
	}
	if contract.Provider != "test-repo" {
		t.Errorf("Provider = %s, want test-repo", contract.Provider)
	}
	if len(contract.Endpoints) != 3 {
		t.Errorf("len(Endpoints) = %d, want 3", len(contract.Endpoints))
	}
}

func TestGenerateFromModel_Empty(t *testing.T) {
	sysModel := &model.SystemModel{
		Repository: "empty-repo",
		Endpoints:  []model.Endpoint{},
	}

	cg := NewContractGenerator()
	contract := cg.GenerateFromModel(sysModel)

	if len(contract.Endpoints) != 0 {
		t.Errorf("len(Endpoints) = %d, want 0", len(contract.Endpoints))
	}
}

func TestInferRequestContract_PathParams(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantCnt int
	}{
		{"colon style", "/users/:id", 1},
		{"brace style", "/users/{id}", 1},
		{"multiple params", "/users/:userId/posts/:postId", 2},
		{"no params", "/users", 0},
	}

	cg := NewContractGenerator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := model.Endpoint{Path: tt.path, Method: "GET"}
			rc := cg.inferRequestContract(ep)
			if len(rc.PathParams) != tt.wantCnt {
				t.Errorf("len(PathParams) = %d, want %d", len(rc.PathParams), tt.wantCnt)
			}
		})
	}
}

func TestInferRequestContract_Body(t *testing.T) {
	cg := NewContractGenerator()

	// POST should have body
	postEp := model.Endpoint{Method: "POST", Path: "/users"}
	postRC := cg.inferRequestContract(postEp)
	if postRC.Body == nil {
		t.Error("POST should have body")
	}
	if postRC.ContentType != "application/json" {
		t.Errorf("ContentType = %s, want application/json", postRC.ContentType)
	}

	// GET should not have body
	getEp := model.Endpoint{Method: "GET", Path: "/users"}
	getRC := cg.inferRequestContract(getEp)
	if getRC.Body != nil {
		t.Error("GET should not have body")
	}
}

func TestInferRequestContract_AuthHeader(t *testing.T) {
	cg := NewContractGenerator()

	// Handler with "auth" should have Authorization header
	authEp := model.Endpoint{Method: "GET", Path: "/users", Handler: "authMiddleware"}
	authRC := cg.inferRequestContract(authEp)
	if _, ok := authRC.Headers["Authorization"]; !ok {
		t.Error("Should have Authorization header for auth handlers")
	}

	// Handler with "protected" should have Authorization header
	protectedEp := model.Endpoint{Method: "GET", Path: "/users", Handler: "ProtectedHandler"}
	protectedRC := cg.inferRequestContract(protectedEp)
	if _, ok := protectedRC.Headers["Authorization"]; !ok {
		t.Error("Should have Authorization header for protected handlers")
	}
}

func TestInferResponseContract_StatusCodes(t *testing.T) {
	cg := NewContractGenerator()

	tests := []struct {
		method     string
		wantStatus int
	}{
		{"GET", http.StatusOK},
		{"POST", http.StatusCreated},
		{"PUT", http.StatusOK},
		{"PATCH", http.StatusOK},
		{"DELETE", http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			ep := model.Endpoint{Method: tt.method, Path: "/test"}
			rc := cg.inferResponseContract(ep)
			if rc.StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %d, want %d", rc.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestNewContractValidator(t *testing.T) {
	cv := NewContractValidator()

	if cv == nil {
		t.Fatal("NewContractValidator() returned nil")
	}
}

func TestValidationResult_Fields(t *testing.T) {
	vr := ValidationResult{
		Valid:    false,
		Endpoint: "GET /users",
		Violations: []ContractViolation{
			{Type: "status", Expected: "200", Actual: "404"},
		},
	}

	if vr.Valid {
		t.Error("Valid should be false")
	}
	if vr.Endpoint != "GET /users" {
		t.Errorf("Endpoint = %s, want GET /users", vr.Endpoint)
	}
	if len(vr.Violations) != 1 {
		t.Errorf("len(Violations) = %d, want 1", len(vr.Violations))
	}
}

func TestContractViolation_Fields(t *testing.T) {
	cv := ContractViolation{
		Type:     "status",
		Path:     "$.id",
		Expected: "200",
		Actual:   "404",
		Message:  "Status code mismatch",
	}

	if cv.Type != "status" {
		t.Errorf("Type = %s, want status", cv.Type)
	}
	if cv.Path != "$.id" {
		t.Errorf("Path = %s, want $.id", cv.Path)
	}
	if cv.Expected != "200" {
		t.Errorf("Expected = %s, want 200", cv.Expected)
	}
	if cv.Actual != "404" {
		t.Errorf("Actual = %s, want 404", cv.Actual)
	}
	if cv.Message != "Status code mismatch" {
		t.Errorf("Message = %s, want Status code mismatch", cv.Message)
	}
}

func TestValidateResponse_StatusCode(t *testing.T) {
	cv := NewContractValidator()

	contract := EndpointContract{
		Method: "GET",
		Path:   "/users",
		Response: ResponseContract{
			StatusCode: 200,
		},
	}

	// Valid status
	result := cv.ValidateResponse(contract, 200, http.Header{}, nil)
	if !result.Valid {
		t.Error("Should be valid for matching status code")
	}

	// Invalid status
	result = cv.ValidateResponse(contract, 404, http.Header{}, nil)
	if result.Valid {
		t.Error("Should be invalid for mismatched status code")
	}
	if len(result.Violations) != 1 {
		t.Errorf("len(Violations) = %d, want 1", len(result.Violations))
	}
	if result.Violations[0].Type != "status" {
		t.Errorf("Violation type = %s, want status", result.Violations[0].Type)
	}
}

func TestValidateResponse_Headers(t *testing.T) {
	cv := NewContractValidator()

	contract := EndpointContract{
		Method: "GET",
		Path:   "/users",
		Response: ResponseContract{
			StatusCode: 200,
			Headers: map[string]HeaderSpec{
				"X-Request-ID": {Required: true},
			},
		},
	}

	// Valid with header
	headers := http.Header{}
	headers.Set("X-Request-ID", "123")
	result := cv.ValidateResponse(contract, 200, headers, nil)
	if !result.Valid {
		t.Error("Should be valid with required header")
	}

	// Invalid without header
	result = cv.ValidateResponse(contract, 200, http.Header{}, nil)
	if result.Valid {
		t.Error("Should be invalid without required header")
	}
}

func TestValidateResponse_BodyJSON(t *testing.T) {
	cv := NewContractValidator()

	contract := EndpointContract{
		Method: "GET",
		Path:   "/users",
		Response: ResponseContract{
			StatusCode: 200,
			Body:       &SchemaSpec{Type: "object"},
		},
	}

	// Valid JSON body
	result := cv.ValidateResponse(contract, 200, http.Header{}, []byte(`{"name": "John"}`))
	if !result.Valid {
		t.Error("Should be valid for correct JSON body")
	}

	// Invalid JSON body
	result = cv.ValidateResponse(contract, 200, http.Header{}, []byte(`invalid json`))
	if result.Valid {
		t.Error("Should be invalid for invalid JSON")
	}
}

func TestValidateSchema_Object(t *testing.T) {
	cv := NewContractValidator()

	schema := &SchemaSpec{
		Type:     "object",
		Required: []string{"name"},
		Properties: map[string]*SchemaSpec{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
	}

	// Valid object
	data := map[string]interface{}{"name": "John", "age": 30.0}
	violations := cv.validateSchema("$", schema, data)
	if len(violations) != 0 {
		t.Errorf("Should have no violations for valid object, got %v", violations)
	}

	// Missing required field
	data = map[string]interface{}{"age": 30.0}
	violations = cv.validateSchema("$", schema, data)
	if len(violations) != 1 {
		t.Errorf("len(violations) = %d, want 1", len(violations))
	}

	// Wrong type
	violations = cv.validateSchema("$", schema, "not an object")
	if len(violations) != 1 {
		t.Errorf("len(violations) = %d, want 1", len(violations))
	}
}

func TestValidateSchema_Array(t *testing.T) {
	cv := NewContractValidator()

	schema := &SchemaSpec{
		Type:  "array",
		Items: &SchemaSpec{Type: "string"},
	}

	// Valid array
	data := []interface{}{"a", "b", "c"}
	violations := cv.validateSchema("$", schema, data)
	if len(violations) != 0 {
		t.Errorf("Should have no violations for valid array, got %v", violations)
	}

	// Invalid item type
	data = []interface{}{"a", 123, "c"}
	violations = cv.validateSchema("$", schema, data)
	if len(violations) != 1 {
		t.Errorf("len(violations) = %d, want 1", len(violations))
	}

	// Wrong type (not array)
	violations = cv.validateSchema("$", schema, "not an array")
	if len(violations) != 1 {
		t.Errorf("len(violations) = %d, want 1", len(violations))
	}
}

func TestValidateSchema_String(t *testing.T) {
	cv := NewContractValidator()

	schema := &SchemaSpec{Type: "string"}

	// Valid string
	violations := cv.validateSchema("$", schema, "hello")
	if len(violations) != 0 {
		t.Error("Should have no violations for valid string")
	}

	// Invalid type
	violations = cv.validateSchema("$", schema, 123)
	if len(violations) != 1 {
		t.Errorf("len(violations) = %d, want 1", len(violations))
	}
}

func TestValidateSchema_Number(t *testing.T) {
	cv := NewContractValidator()

	tests := []struct {
		schemaType string
		value      interface{}
		wantValid  bool
	}{
		{"integer", float64(123), true},
		{"integer", "not a number", false},
		{"number", float64(123.45), true},
		{"number", "not a number", false},
	}

	for _, tt := range tests {
		schema := &SchemaSpec{Type: tt.schemaType}
		violations := cv.validateSchema("$", schema, tt.value)
		isValid := len(violations) == 0
		if isValid != tt.wantValid {
			t.Errorf("validateSchema(%s, %v) valid = %v, want %v", tt.schemaType, tt.value, isValid, tt.wantValid)
		}
	}
}

func TestValidateSchema_Boolean(t *testing.T) {
	cv := NewContractValidator()

	schema := &SchemaSpec{Type: "boolean"}

	// Valid boolean
	violations := cv.validateSchema("$", schema, true)
	if len(violations) != 0 {
		t.Error("Should have no violations for valid boolean")
	}

	// Invalid type
	violations = cv.validateSchema("$", schema, "true")
	if len(violations) != 1 {
		t.Errorf("len(violations) = %d, want 1", len(violations))
	}
}

func TestValidateSchema_Enum(t *testing.T) {
	cv := NewContractValidator()

	schema := &SchemaSpec{
		Type: "string",
		Enum: []interface{}{"active", "inactive"},
	}

	// Valid enum value
	violations := cv.validateSchema("$", schema, "active")
	if len(violations) != 0 {
		t.Error("Should have no violations for valid enum value")
	}

	// Invalid enum value
	violations = cv.validateSchema("$", schema, "unknown")
	if len(violations) != 1 {
		t.Errorf("len(violations) = %d, want 1", len(violations))
	}
}

func TestValidateSchema_Nil(t *testing.T) {
	cv := NewContractValidator()

	schema := &SchemaSpec{Type: "string"}

	// Nil data should not cause violations
	violations := cv.validateSchema("$", schema, nil)
	if len(violations) != 0 {
		t.Errorf("len(violations) = %d, want 0 for nil data", len(violations))
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		value interface{}
		want  bool
	}{
		{int(1), true},
		{int8(1), true},
		{int16(1), true},
		{int32(1), true},
		{int64(1), true},
		{uint(1), true},
		{uint8(1), true},
		{uint16(1), true},
		{uint32(1), true},
		{uint64(1), true},
		{float32(1.0), true},
		{float64(1.0), true},
		{"string", false},
		{true, false},
		{nil, false},
	}

	for _, tt := range tests {
		got := isNumeric(tt.value)
		if got != tt.want {
			t.Errorf("isNumeric(%T) = %v, want %v", tt.value, got, tt.want)
		}
	}
}
