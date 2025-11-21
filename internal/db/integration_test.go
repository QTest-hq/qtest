//go:build integration
// +build integration

package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/QTest-hq/qtest/internal/testutil"
	"github.com/google/uuid"
)

func TestIntegration_CreateAndGetRepository(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository
	lang := "go"
	repo := &Repository{
		URL:           "https://github.com/test/integration-test-repo",
		Name:          "integration-test-repo",
		Owner:         "test",
		DefaultBranch: "main",
		Language:      &lang,
	}

	err := store.CreateRepository(ctx, repo)
	if err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	if repo.ID == uuid.Nil {
		t.Error("CreateRepository() should set ID")
	}
	if repo.Status != "pending" {
		t.Errorf("CreateRepository() status = %s, want pending", repo.Status)
	}

	// Get by ID
	fetched, err := store.GetRepository(ctx, repo.ID)
	if err != nil {
		t.Fatalf("GetRepository() error: %v", err)
	}
	if fetched == nil {
		t.Fatal("GetRepository() returned nil")
	}
	if fetched.URL != repo.URL {
		t.Errorf("URL = %s, want %s", fetched.URL, repo.URL)
	}
	if fetched.Name != repo.Name {
		t.Errorf("Name = %s, want %s", fetched.Name, repo.Name)
	}
	if *fetched.Language != "go" {
		t.Errorf("Language = %s, want go", *fetched.Language)
	}
}

func TestIntegration_GetRepositoryByURL(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository
	repo := &Repository{
		URL:           "https://github.com/test/url-test-repo",
		Name:          "url-test-repo",
		Owner:         "test",
		DefaultBranch: "main",
	}

	err := store.CreateRepository(ctx, repo)
	if err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	// Get by URL
	fetched, err := store.GetRepositoryByURL(ctx, repo.URL)
	if err != nil {
		t.Fatalf("GetRepositoryByURL() error: %v", err)
	}
	if fetched == nil {
		t.Fatal("GetRepositoryByURL() returned nil")
	}
	if fetched.ID != repo.ID {
		t.Errorf("ID = %s, want %s", fetched.ID, repo.ID)
	}

	// Non-existent URL
	notFound, err := store.GetRepositoryByURL(ctx, "https://github.com/nonexistent/repo")
	if err != nil {
		t.Fatalf("GetRepositoryByURL() error for non-existent: %v", err)
	}
	if notFound != nil {
		t.Error("GetRepositoryByURL() should return nil for non-existent")
	}
}

func TestIntegration_ListRepositories(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create multiple repositories
	for i := 0; i < 5; i++ {
		repo := &Repository{
			URL:           "https://github.com/test/list-test-repo-" + string(rune('a'+i)),
			Name:          "list-test-repo-" + string(rune('a'+i)),
			Owner:         "test",
			DefaultBranch: "main",
		}
		if err := store.CreateRepository(ctx, repo); err != nil {
			t.Fatalf("CreateRepository() error: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List with limit
	repos, err := store.ListRepositories(ctx, 3, 0)
	if err != nil {
		t.Fatalf("ListRepositories() error: %v", err)
	}
	if len(repos) != 3 {
		t.Errorf("len(repos) = %d, want 3", len(repos))
	}

	// List with offset
	repos, err = store.ListRepositories(ctx, 10, 2)
	if err != nil {
		t.Fatalf("ListRepositories() error: %v", err)
	}
	if len(repos) != 3 {
		t.Errorf("len(repos) = %d, want 3", len(repos))
	}
}

func TestIntegration_UpdateRepositoryStatus(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository
	repo := &Repository{
		URL:           "https://github.com/test/status-test-repo",
		Name:          "status-test-repo",
		Owner:         "test",
		DefaultBranch: "main",
	}

	err := store.CreateRepository(ctx, repo)
	if err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	// Update status
	sha := "abc123def456"
	err = store.UpdateRepositoryStatus(ctx, repo.ID, "active", &sha)
	if err != nil {
		t.Fatalf("UpdateRepositoryStatus() error: %v", err)
	}

	// Verify
	fetched, _ := store.GetRepository(ctx, repo.ID)
	if fetched.Status != "active" {
		t.Errorf("Status = %s, want active", fetched.Status)
	}
	if *fetched.LastCommitSHA != sha {
		t.Errorf("LastCommitSHA = %s, want %s", *fetched.LastCommitSHA, sha)
	}
}

func TestIntegration_CreateAndGetGenerationRun(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// First create a repository
	repo := &Repository{
		URL:           "https://github.com/test/run-test-repo",
		Name:          "run-test-repo",
		Owner:         "test",
		DefaultBranch: "main",
	}
	if err := store.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	// Create generation run
	run := &GenerationRun{
		RepositoryID: repo.ID,
		Config:       json.RawMessage(`{"tier": 1, "maxTests": 10}`),
	}

	err := store.CreateGenerationRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateGenerationRun() error: %v", err)
	}

	if run.ID == uuid.Nil {
		t.Error("CreateGenerationRun() should set ID")
	}
	if run.Status != "pending" {
		t.Errorf("Status = %s, want pending", run.Status)
	}

	// Get run
	fetched, err := store.GetGenerationRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetGenerationRun() error: %v", err)
	}
	if fetched == nil {
		t.Fatal("GetGenerationRun() returned nil")
	}
	if fetched.RepositoryID != repo.ID {
		t.Errorf("RepositoryID = %s, want %s", fetched.RepositoryID, repo.ID)
	}
}

func TestIntegration_UpdateGenerationRunStatus(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository and run
	repo := &Repository{
		URL:           "https://github.com/test/run-status-test-repo",
		Name:          "run-status-test-repo",
		Owner:         "test",
		DefaultBranch: "main",
	}
	if err := store.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	run := &GenerationRun{RepositoryID: repo.ID}
	if err := store.CreateGenerationRun(ctx, run); err != nil {
		t.Fatalf("CreateGenerationRun() error: %v", err)
	}

	// Update to running
	err := store.UpdateGenerationRunStatus(ctx, run.ID, "running")
	if err != nil {
		t.Fatalf("UpdateGenerationRunStatus() error: %v", err)
	}

	fetched, _ := store.GetGenerationRun(ctx, run.ID)
	if fetched.Status != "running" {
		t.Errorf("Status = %s, want running", fetched.Status)
	}
	if fetched.StartedAt == nil {
		t.Error("StartedAt should be set when status is running")
	}

	// Update to completed
	err = store.UpdateGenerationRunStatus(ctx, run.ID, "completed")
	if err != nil {
		t.Fatalf("UpdateGenerationRunStatus() error: %v", err)
	}

	fetched, _ = store.GetGenerationRun(ctx, run.ID)
	if fetched.Status != "completed" {
		t.Errorf("Status = %s, want completed", fetched.Status)
	}
	if fetched.CompletedAt == nil {
		t.Error("CompletedAt should be set when status is completed")
	}
}

func TestIntegration_CreateAndListTests(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository and run
	repo := &Repository{
		URL:           "https://github.com/test/tests-list-repo",
		Name:          "tests-list-repo",
		Owner:         "test",
		DefaultBranch: "main",
	}
	if err := store.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	run := &GenerationRun{RepositoryID: repo.ID}
	if err := store.CreateGenerationRun(ctx, run); err != nil {
		t.Fatalf("CreateGenerationRun() error: %v", err)
	}

	// Create tests
	framework := "go"
	for i := 0; i < 3; i++ {
		test := &GeneratedTest{
			RunID:      run.ID,
			Name:       "TestFunc" + string(rune('A'+i)),
			Type:       "unit",
			TargetFile: "main.go",
			DSL:        json.RawMessage(`{"tests": []}`),
			Framework:  &framework,
		}
		if err := store.CreateGeneratedTest(ctx, test); err != nil {
			t.Fatalf("CreateGeneratedTest() error: %v", err)
		}
	}

	// List tests
	tests, err := store.ListTestsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListTestsByRun() error: %v", err)
	}
	if len(tests) != 3 {
		t.Errorf("len(tests) = %d, want 3", len(tests))
	}

	// Verify test fields
	for _, test := range tests {
		if test.RunID != run.ID {
			t.Errorf("RunID = %s, want %s", test.RunID, run.ID)
		}
		if test.Status != "pending" {
			t.Errorf("Status = %s, want pending", test.Status)
		}
		if *test.Framework != "go" {
			t.Errorf("Framework = %s, want go", *test.Framework)
		}
	}
}

func TestIntegration_GetNonExistentRepository(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Get non-existent
	repo, err := store.GetRepository(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetRepository() error: %v", err)
	}
	if repo != nil {
		t.Error("GetRepository() should return nil for non-existent ID")
	}
}

func TestIntegration_GetNonExistentGenerationRun(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Get non-existent
	run, err := store.GetGenerationRun(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetGenerationRun() error: %v", err)
	}
	if run != nil {
		t.Error("GetGenerationRun() should return nil for non-existent ID")
	}
}

func TestIntegration_DBHealthCheck(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	ctx := context.Background()

	err := db.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck() error: %v", err)
	}
}

func TestIntegration_DBNew(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	dbURL := testutil.GetTestDBURL()

	db, err := New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping test: could not connect to database: %v", err)
	}
	defer db.Close()

	if db.Pool() == nil {
		t.Error("Pool() should not be nil")
	}

	if err := db.HealthCheck(ctx); err != nil {
		t.Errorf("HealthCheck() error: %v", err)
	}
}

func TestIntegration_CreateAndGetMutationRun(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository first
	repo := &Repository{
		URL:           "https://github.com/test/mutation-run-test",
		Name:          "mutation-run-test",
		Owner:         "test",
		DefaultBranch: "main",
	}
	if err := store.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	// Create mutation run
	run := &MutationRun{
		RepositoryID: &repo.ID,
		SourceFile:   "internal/service/user.go",
		TestFile:     "internal/service/user_test.go",
		TotalMutants: 10,
		Killed:       7,
		Survived:     2,
		Timeout:      1,
		Score:        0.70,
		Quality:      "good",
	}

	err := store.CreateMutationRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateMutationRun() error: %v", err)
	}

	if run.ID == uuid.Nil {
		t.Error("CreateMutationRun() should set ID")
	}

	// Get by ID
	fetched, err := store.GetMutationRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetMutationRun() error: %v", err)
	}
	if fetched == nil {
		t.Fatal("GetMutationRun() returned nil")
	}
	if fetched.SourceFile != run.SourceFile {
		t.Errorf("SourceFile = %s, want %s", fetched.SourceFile, run.SourceFile)
	}
	if fetched.TestFile != run.TestFile {
		t.Errorf("TestFile = %s, want %s", fetched.TestFile, run.TestFile)
	}
	if fetched.TotalMutants != 10 {
		t.Errorf("TotalMutants = %d, want 10", fetched.TotalMutants)
	}
	if fetched.Score != 0.70 {
		t.Errorf("Score = %f, want 0.70", fetched.Score)
	}
	if fetched.Quality != "good" {
		t.Errorf("Quality = %s, want good", fetched.Quality)
	}
}

func TestIntegration_ListMutationRunsByRepository(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository
	repo := &Repository{
		URL:           "https://github.com/test/mutation-list-test",
		Name:          "mutation-list-test",
		Owner:         "test",
		DefaultBranch: "main",
	}
	if err := store.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	// Create multiple mutation runs
	files := []string{"user.go", "order.go", "product.go"}
	for _, f := range files {
		run := &MutationRun{
			RepositoryID: &repo.ID,
			SourceFile:   "internal/" + f,
			TestFile:     "internal/" + f[:len(f)-3] + "_test.go",
			TotalMutants: 5,
			Killed:       4,
			Survived:     1,
			Score:        0.80,
			Quality:      "good",
		}
		if err := store.CreateMutationRun(ctx, run); err != nil {
			t.Fatalf("CreateMutationRun() error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// List runs
	runs, err := store.ListMutationRunsByRepository(ctx, repo.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListMutationRunsByRepository() error: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("len(runs) = %d, want 3", len(runs))
	}

	// Verify all belong to the repository
	for _, run := range runs {
		if *run.RepositoryID != repo.ID {
			t.Errorf("RepositoryID = %s, want %s", *run.RepositoryID, repo.ID)
		}
	}
}

func TestIntegration_UpdateMutationRunResults(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository
	repo := &Repository{
		URL:           "https://github.com/test/mutation-update-test",
		Name:          "mutation-update-test",
		Owner:         "test",
		DefaultBranch: "main",
	}
	if err := store.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	// Create mutation run with pending status
	run := &MutationRun{
		RepositoryID: &repo.ID,
		SourceFile:   "service.go",
		TestFile:     "service_test.go",
	}
	if err := store.CreateMutationRun(ctx, run); err != nil {
		t.Fatalf("CreateMutationRun() error: %v", err)
	}

	// Update with results
	err := store.UpdateMutationRunResults(ctx, run.ID, 15, 12, 2, 1, 0.80, "good")
	if err != nil {
		t.Fatalf("UpdateMutationRunResults() error: %v", err)
	}

	// Verify
	fetched, _ := store.GetMutationRun(ctx, run.ID)
	if fetched.TotalMutants != 15 {
		t.Errorf("TotalMutants = %d, want 15", fetched.TotalMutants)
	}
	if fetched.Killed != 12 {
		t.Errorf("Killed = %d, want 12", fetched.Killed)
	}
	if fetched.Survived != 2 {
		t.Errorf("Survived = %d, want 2", fetched.Survived)
	}
	if fetched.Timeout != 1 {
		t.Errorf("Timeout = %d, want 1", fetched.Timeout)
	}
	if fetched.Score != 0.80 {
		t.Errorf("Score = %f, want 0.80", fetched.Score)
	}
	if fetched.Quality != "good" {
		t.Errorf("Quality = %s, want good", fetched.Quality)
	}
	if fetched.CompletedAt == nil {
		t.Error("CompletedAt should be set after update")
	}
}

func TestIntegration_CreateAndListMutants(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository
	repo := &Repository{
		URL:           "https://github.com/test/mutants-test",
		Name:          "mutants-test",
		Owner:         "test",
		DefaultBranch: "main",
	}
	if err := store.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	// Create mutation run
	run := &MutationRun{
		RepositoryID: &repo.ID,
		SourceFile:   "calc.go",
		TestFile:     "calc_test.go",
	}
	if err := store.CreateMutationRun(ctx, run); err != nil {
		t.Fatalf("CreateMutationRun() error: %v", err)
	}

	// Create mutants
	mutantsData := []struct {
		line    int
		mutType string
		status  string
	}{
		{10, "arithmetic", "killed"},
		{15, "comparison", "killed"},
		{20, "boolean", "survived"},
		{25, "return", "killed"},
		{30, "statement", "timeout"},
	}

	for _, m := range mutantsData {
		desc := "mutation at line " + string(rune('0'+m.line%10))
		mutant := &Mutant{
			MutationRunID: run.ID,
			LineNumber:    m.line,
			MutationType:  m.mutType,
			Status:        m.status,
			Description:   &desc,
		}
		if err := store.CreateMutant(ctx, mutant); err != nil {
			t.Fatalf("CreateMutant() error: %v", err)
		}
	}

	// List mutants
	mutants, err := store.ListMutantsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListMutantsByRun() error: %v", err)
	}
	if len(mutants) != 5 {
		t.Errorf("len(mutants) = %d, want 5", len(mutants))
	}

	// Verify they are ordered by line number
	for i := 1; i < len(mutants); i++ {
		if mutants[i].LineNumber < mutants[i-1].LineNumber {
			t.Errorf("mutants not ordered by line number")
		}
	}

	// Count statuses
	killedCount := 0
	survivedCount := 0
	timeoutCount := 0
	for _, m := range mutants {
		switch m.Status {
		case "killed":
			killedCount++
		case "survived":
			survivedCount++
		case "timeout":
			timeoutCount++
		}
	}
	if killedCount != 3 {
		t.Errorf("killed count = %d, want 3", killedCount)
	}
	if survivedCount != 1 {
		t.Errorf("survived count = %d, want 1", survivedCount)
	}
	if timeoutCount != 1 {
		t.Errorf("timeout count = %d, want 1", timeoutCount)
	}
}

func TestIntegration_GetNonExistentMutationRun(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Get non-existent
	run, err := store.GetMutationRun(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetMutationRun() error: %v", err)
	}
	if run != nil {
		t.Error("GetMutationRun() should return nil for non-existent ID")
	}
}

func TestIntegration_MutationRunWithGenerationRun(t *testing.T) {
	testDB := testutil.RequireDB(t)

	db := &DB{pool: testDB.Pool}
	store := NewStore(db)
	ctx := context.Background()

	// Create repository
	repo := &Repository{
		URL:           "https://github.com/test/mutation-gen-run-test",
		Name:          "mutation-gen-run-test",
		Owner:         "test",
		DefaultBranch: "main",
	}
	if err := store.CreateRepository(ctx, repo); err != nil {
		t.Fatalf("CreateRepository() error: %v", err)
	}

	// Create generation run
	genRun := &GenerationRun{
		RepositoryID: repo.ID,
		Config:       json.RawMessage(`{}`),
	}
	if err := store.CreateGenerationRun(ctx, genRun); err != nil {
		t.Fatalf("CreateGenerationRun() error: %v", err)
	}

	// Create mutation runs linked to generation run
	for i := 0; i < 3; i++ {
		run := &MutationRun{
			RepositoryID:    &repo.ID,
			GenerationRunID: &genRun.ID,
			SourceFile:      "file" + string(rune('a'+i)) + ".go",
			TestFile:        "file" + string(rune('a'+i)) + "_test.go",
			TotalMutants:    5,
			Killed:          4,
			Survived:        1,
			Score:           0.80,
			Quality:         "good",
		}
		if err := store.CreateMutationRun(ctx, run); err != nil {
			t.Fatalf("CreateMutationRun() error: %v", err)
		}
	}

	// List by generation run
	runs, err := store.ListMutationRunsByGenerationRun(ctx, genRun.ID)
	if err != nil {
		t.Fatalf("ListMutationRunsByGenerationRun() error: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("len(runs) = %d, want 3", len(runs))
	}

	// Verify all belong to the generation run
	for _, run := range runs {
		if *run.GenerationRunID != genRun.ID {
			t.Errorf("GenerationRunID = %s, want %s", *run.GenerationRunID, genRun.ID)
		}
	}
}
