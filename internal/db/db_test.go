package db

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDB_Fields(t *testing.T) {
	// DB struct should have pool field
	db := &DB{pool: nil}
	if db.pool != nil {
		t.Error("pool should be nil")
	}
}

func TestDB_Pool_Nil(t *testing.T) {
	db := &DB{pool: nil}

	pool := db.Pool()
	if pool != nil {
		t.Error("Pool() should return nil when pool is nil")
	}
}

func TestRepository_Fields(t *testing.T) {
	id := uuid.New()
	lang := "go"
	sha := "abc123"

	repo := Repository{
		ID:            id,
		URL:           "https://github.com/test/repo",
		Name:          "repo",
		Owner:         "test",
		DefaultBranch: "main",
		Language:      &lang,
		LastCommitSHA: &sha,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if repo.ID != id {
		t.Errorf("ID mismatch")
	}
	if repo.URL != "https://github.com/test/repo" {
		t.Errorf("URL = %s, want https://github.com/test/repo", repo.URL)
	}
	if repo.Name != "repo" {
		t.Errorf("Name = %s, want repo", repo.Name)
	}
	if repo.Owner != "test" {
		t.Errorf("Owner = %s, want test", repo.Owner)
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %s, want main", repo.DefaultBranch)
	}
	if *repo.Language != "go" {
		t.Errorf("Language = %s, want go", *repo.Language)
	}
	if *repo.LastCommitSHA != "abc123" {
		t.Errorf("LastCommitSHA = %s, want abc123", *repo.LastCommitSHA)
	}
	if repo.Status != "active" {
		t.Errorf("Status = %s, want active", repo.Status)
	}
}

func TestRepository_JSON(t *testing.T) {
	lang := "go"
	repo := Repository{
		ID:            uuid.New(),
		URL:           "https://github.com/test/repo",
		Name:          "repo",
		Owner:         "test",
		DefaultBranch: "main",
		Language:      &lang,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	data, err := json.Marshal(repo)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var unmarshaled Repository
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if unmarshaled.URL != repo.URL {
		t.Errorf("URL = %s, want %s", unmarshaled.URL, repo.URL)
	}
	if unmarshaled.Name != repo.Name {
		t.Errorf("Name = %s, want %s", unmarshaled.Name, repo.Name)
	}
}

func TestSystemModel_Fields(t *testing.T) {
	id := uuid.New()
	repoID := uuid.New()
	modelData := json.RawMessage(`{"functions": []}`)

	sm := SystemModel{
		ID:           id,
		RepositoryID: repoID,
		CommitSHA:    "abc123",
		ModelData:    modelData,
		CreatedAt:    time.Now(),
	}

	if sm.ID != id {
		t.Error("ID mismatch")
	}
	if sm.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if sm.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %s, want abc123", sm.CommitSHA)
	}
	if sm.ModelData == nil {
		t.Error("ModelData should not be nil")
	}
}

func TestSystemModel_JSON(t *testing.T) {
	sm := SystemModel{
		ID:           uuid.New(),
		RepositoryID: uuid.New(),
		CommitSHA:    "abc123",
		ModelData:    json.RawMessage(`{"test": true}`),
		CreatedAt:    time.Now(),
	}

	data, err := json.Marshal(sm)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var unmarshaled SystemModel
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if unmarshaled.CommitSHA != sm.CommitSHA {
		t.Errorf("CommitSHA = %s, want %s", unmarshaled.CommitSHA, sm.CommitSHA)
	}
}

func TestGenerationRun_Fields(t *testing.T) {
	id := uuid.New()
	repoID := uuid.New()
	modelID := uuid.New()
	config := json.RawMessage(`{"tier": 2}`)
	summary := json.RawMessage(`{"total": 10}`)
	startedAt := time.Now()
	completedAt := time.Now()

	run := GenerationRun{
		ID:            id,
		RepositoryID:  repoID,
		SystemModelID: &modelID,
		Status:        "completed",
		Config:        config,
		Summary:       &summary,
		StartedAt:     &startedAt,
		CompletedAt:   &completedAt,
		CreatedAt:     time.Now(),
	}

	if run.ID != id {
		t.Error("ID mismatch")
	}
	if run.RepositoryID != repoID {
		t.Error("RepositoryID mismatch")
	}
	if *run.SystemModelID != modelID {
		t.Error("SystemModelID mismatch")
	}
	if run.Status != "completed" {
		t.Errorf("Status = %s, want completed", run.Status)
	}
	if run.Config == nil {
		t.Error("Config should not be nil")
	}
	if run.Summary == nil {
		t.Error("Summary should not be nil")
	}
	if run.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
	if run.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestGenerationRun_JSON(t *testing.T) {
	run := GenerationRun{
		ID:           uuid.New(),
		RepositoryID: uuid.New(),
		Status:       "running",
		Config:       json.RawMessage(`{}`),
		CreatedAt:    time.Now(),
	}

	data, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var unmarshaled GenerationRun
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if unmarshaled.Status != run.Status {
		t.Errorf("Status = %s, want %s", unmarshaled.Status, run.Status)
	}
}

func TestGeneratedTest_Fields(t *testing.T) {
	id := uuid.New()
	runID := uuid.New()
	targetFunc := "TestFunc"
	generatedCode := "func Test() {}"
	framework := "go"
	rejectionReason := "compilation failed"
	mutationScore := 85.5
	metadata := json.RawMessage(`{"loc": 50}`)

	test := GeneratedTest{
		ID:              id,
		RunID:           runID,
		Name:            "TestSomething",
		Type:            "unit",
		TargetFile:      "main.go",
		TargetFunction:  &targetFunc,
		DSL:             json.RawMessage(`{"tests": []}`),
		GeneratedCode:   &generatedCode,
		Framework:       &framework,
		Status:          "accepted",
		RejectionReason: &rejectionReason,
		MutationScore:   &mutationScore,
		Metadata:        &metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if test.ID != id {
		t.Error("ID mismatch")
	}
	if test.RunID != runID {
		t.Error("RunID mismatch")
	}
	if test.Name != "TestSomething" {
		t.Errorf("Name = %s, want TestSomething", test.Name)
	}
	if test.Type != "unit" {
		t.Errorf("Type = %s, want unit", test.Type)
	}
	if test.TargetFile != "main.go" {
		t.Errorf("TargetFile = %s, want main.go", test.TargetFile)
	}
	if *test.TargetFunction != "TestFunc" {
		t.Errorf("TargetFunction = %s, want TestFunc", *test.TargetFunction)
	}
	if *test.GeneratedCode != "func Test() {}" {
		t.Error("GeneratedCode mismatch")
	}
	if *test.Framework != "go" {
		t.Errorf("Framework = %s, want go", *test.Framework)
	}
	if test.Status != "accepted" {
		t.Errorf("Status = %s, want accepted", test.Status)
	}
	if *test.RejectionReason != "compilation failed" {
		t.Error("RejectionReason mismatch")
	}
	if *test.MutationScore != 85.5 {
		t.Errorf("MutationScore = %f, want 85.5", *test.MutationScore)
	}
	if test.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
}

func TestGeneratedTest_JSON(t *testing.T) {
	test := GeneratedTest{
		ID:         uuid.New(),
		RunID:      uuid.New(),
		Name:       "TestSomething",
		Type:       "unit",
		TargetFile: "main.go",
		DSL:        json.RawMessage(`{}`),
		Status:     "pending",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	data, err := json.Marshal(test)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var unmarshaled GeneratedTest
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if unmarshaled.Name != test.Name {
		t.Errorf("Name = %s, want %s", unmarshaled.Name, test.Name)
	}
	if unmarshaled.TargetFile != test.TargetFile {
		t.Errorf("TargetFile = %s, want %s", unmarshaled.TargetFile, test.TargetFile)
	}
}

func TestStore_Fields(t *testing.T) {
	// Store with nil pool
	store := &Store{pool: nil}
	if store.pool != nil {
		t.Error("pool should be nil")
	}
}

func TestNewStore_NilDB(t *testing.T) {
	// This would panic if db is nil
	// Just test that the struct exists
	db := &DB{pool: nil}
	store := NewStore(db)

	if store == nil {
		t.Error("NewStore should not return nil")
	}
}

func TestRepository_Defaults(t *testing.T) {
	repo := Repository{}

	if repo.ID != uuid.Nil {
		t.Error("Default ID should be nil UUID")
	}
	if repo.URL != "" {
		t.Error("Default URL should be empty")
	}
	if repo.Language != nil {
		t.Error("Default Language should be nil")
	}
	if repo.LastCommitSHA != nil {
		t.Error("Default LastCommitSHA should be nil")
	}
}

func TestGenerationRun_Defaults(t *testing.T) {
	run := GenerationRun{}

	if run.ID != uuid.Nil {
		t.Error("Default ID should be nil UUID")
	}
	if run.SystemModelID != nil {
		t.Error("Default SystemModelID should be nil")
	}
	if run.Summary != nil {
		t.Error("Default Summary should be nil")
	}
	if run.StartedAt != nil {
		t.Error("Default StartedAt should be nil")
	}
	if run.CompletedAt != nil {
		t.Error("Default CompletedAt should be nil")
	}
}

func TestGeneratedTest_Defaults(t *testing.T) {
	test := GeneratedTest{}

	if test.ID != uuid.Nil {
		t.Error("Default ID should be nil UUID")
	}
	if test.TargetFunction != nil {
		t.Error("Default TargetFunction should be nil")
	}
	if test.GeneratedCode != nil {
		t.Error("Default GeneratedCode should be nil")
	}
	if test.Framework != nil {
		t.Error("Default Framework should be nil")
	}
	if test.RejectionReason != nil {
		t.Error("Default RejectionReason should be nil")
	}
	if test.MutationScore != nil {
		t.Error("Default MutationScore should be nil")
	}
	if test.Metadata != nil {
		t.Error("Default Metadata should be nil")
	}
}
