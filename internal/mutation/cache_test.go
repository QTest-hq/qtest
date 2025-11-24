package mutation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultCacheConfig(t *testing.T) {
	cfg := DefaultCacheConfig()
	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
	if cfg.Directory != ".qtest/mutation-cache" {
		t.Errorf("expected Directory .qtest/mutation-cache, got %s", cfg.Directory)
	}
	if cfg.TTL != 24*time.Hour {
		t.Errorf("expected TTL 24h, got %v", cfg.TTL)
	}
	if cfg.MaxEntries != 1000 {
		t.Errorf("expected MaxEntries 1000, got %d", cfg.MaxEntries)
	}
	if !cfg.PersistToDisk {
		t.Error("expected PersistToDisk to be true")
	}
}

func TestNewCache(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewCacheWithNilConfig(t *testing.T) {
	// Should use default config
	cache, err := NewCache(nil)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestCacheSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create test files
	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main\nfunc Add(a, b int) int { return a + b }"), 0644)
	os.WriteFile(testFile, []byte("package main\nfunc TestAdd(t *testing.T) {}"), 0644)

	// Store a result
	result := &Result{
		Score:    85.5,
		Total:    10,
		Killed:   8,
		Survived: 2,
	}

	err = cache.Set(sourceFile, testFile, result)
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Retrieve the result
	cached, found := cache.Get(sourceFile, testFile)
	if !found {
		t.Fatal("expected to find cached result")
	}
	if cached.Score != 85.5 {
		t.Errorf("expected score 85.5, got %f", cached.Score)
	}
	if cached.Total != 10 {
		t.Errorf("expected 10 mutants, got %d", cached.Total)
	}
}

func TestCacheDisabled(t *testing.T) {
	cfg := &CacheConfig{
		Enabled:       false,
		Directory:     t.TempDir(),
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	// Set should be no-op
	err = cache.Set(sourceFile, testFile, &Result{Score: 50})
	if err != nil {
		t.Fatalf("set should not error when disabled: %v", err)
	}

	// Get should return not found
	_, found := cache.Get(sourceFile, testFile)
	if found {
		t.Error("should not find cached result when cache is disabled")
	}
}

func TestCacheExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           1 * time.Millisecond, // Very short TTL
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	err = cache.Set(sourceFile, testFile, &Result{Score: 50})
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	_, found := cache.Get(sourceFile, testFile)
	if found {
		t.Error("should not find expired cached result")
	}
}

func TestCacheInvalidation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	err = cache.Set(sourceFile, testFile, &Result{Score: 50})
	if err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Verify it's cached
	_, found := cache.Get(sourceFile, testFile)
	if !found {
		t.Fatal("expected to find cached result")
	}

	// Invalidate
	cache.Invalidate(sourceFile, testFile)

	// Verify it's gone
	_, found = cache.Get(sourceFile, testFile)
	if found {
		t.Error("should not find invalidated cached result")
	}
}

func TestCacheInvalidateFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create multiple source/test pairs
	source1 := filepath.Join(tmpDir, "source1.go")
	source2 := filepath.Join(tmpDir, "source2.go")
	test1 := filepath.Join(tmpDir, "source1_test.go")
	test2 := filepath.Join(tmpDir, "source2_test.go")

	os.WriteFile(source1, []byte("package main // 1"), 0644)
	os.WriteFile(source2, []byte("package main // 2"), 0644)
	os.WriteFile(test1, []byte("package main // t1"), 0644)
	os.WriteFile(test2, []byte("package main // t2"), 0644)

	cache.Set(source1, test1, &Result{Score: 50})
	cache.Set(source2, test2, &Result{Score: 75})

	// Invalidate all entries related to source1
	cache.InvalidateFile(source1)

	// source1 entry should be gone
	_, found := cache.Get(source1, test1)
	if found {
		t.Error("should not find invalidated source1 entry")
	}

	// source2 entry should still exist
	_, found = cache.Get(source2, test2)
	if !found {
		t.Error("expected to find source2 entry")
	}
}

func TestCacheClear(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	cache.Set(sourceFile, testFile, &Result{Score: 50})

	err = cache.Clear()
	if err != nil {
		t.Fatalf("failed to clear cache: %v", err)
	}

	_, found := cache.Get(sourceFile, testFile)
	if found {
		t.Error("should not find cached result after clear")
	}
}

func TestCacheCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           1 * time.Millisecond,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	cache.Set(sourceFile, testFile, &Result{Score: 50})

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	removed := cache.Cleanup()
	if removed != 1 {
		t.Errorf("expected 1 removed entry, got %d", removed)
	}
}

func TestCacheStats(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	stats := cache.Stats()
	if stats.TotalEntries != 0 {
		t.Errorf("expected 0 entries, got %d", stats.TotalEntries)
	}
	if !stats.Enabled {
		t.Error("expected Enabled to be true")
	}

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	cache.Set(sourceFile, testFile, &Result{Score: 50})

	stats = cache.Stats()
	if stats.TotalEntries != 1 {
		t.Errorf("expected 1 entry, got %d", stats.TotalEntries)
	}
}

func TestCacheMaxEntries(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    2, // Only allow 2 entries
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create 3 source/test pairs
	for i := 0; i < 3; i++ {
		source := filepath.Join(tmpDir, filepath.Base(tmpDir), "source"+string(rune('a'+i))+".go")
		test := filepath.Join(tmpDir, filepath.Base(tmpDir), "source"+string(rune('a'+i))+"_test.go")
		os.MkdirAll(filepath.Dir(source), 0755)
		os.WriteFile(source, []byte("package main // "+string(rune('a'+i))), 0644)
		os.WriteFile(test, []byte("package main // t"+string(rune('a'+i))), 0644)
		cache.Set(source, test, &Result{Score: float64(50 + i*10)})
	}

	stats := cache.Stats()
	if stats.TotalEntries > 2 {
		t.Errorf("expected max 2 entries, got %d", stats.TotalEntries)
	}
}

func TestCacheFileContentChange(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main\nfunc Add(a, b int) int { return a + b }"), 0644)
	os.WriteFile(testFile, []byte("package main\nfunc TestAdd(t *testing.T) {}"), 0644)

	cache.Set(sourceFile, testFile, &Result{Score: 50})

	// Verify it's cached
	_, found := cache.Get(sourceFile, testFile)
	if !found {
		t.Fatal("expected to find cached result")
	}

	// Modify source file
	os.WriteFile(sourceFile, []byte("package main\nfunc Add(a, b int) int { return a + b + 0 }"), 0644)

	// Cache should be invalidated due to content change
	_, found = cache.Get(sourceFile, testFile)
	if found {
		t.Error("should not find cached result after source content change")
	}
}

func TestCachePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: true,
	}

	// Create first cache and add entry
	cache1, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	cache1.Set(sourceFile, testFile, &Result{Score: 75.5})

	// Create new cache instance - should load from disk
	cache2, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create second cache: %v", err)
	}

	cached, found := cache2.Get(sourceFile, testFile)
	if !found {
		t.Fatal("expected to find cached result from persisted cache")
	}
	if cached.Score != 75.5 {
		t.Errorf("expected score 75.5, got %f", cached.Score)
	}
}

func TestHashFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")

	os.WriteFile(file, []byte("hello world"), 0644)

	hash1, err := hashFile(file)
	if err != nil {
		t.Fatalf("failed to hash file: %v", err)
	}
	if hash1 == "" {
		t.Error("expected non-empty hash")
	}

	// Same content should produce same hash
	hash2, err := hashFile(file)
	if err != nil {
		t.Fatalf("failed to hash file: %v", err)
	}
	if hash1 != hash2 {
		t.Error("expected same hash for same content")
	}

	// Different content should produce different hash
	os.WriteFile(file, []byte("hello world!"), 0644)
	hash3, err := hashFile(file)
	if err != nil {
		t.Fatalf("failed to hash file: %v", err)
	}
	if hash1 == hash3 {
		t.Error("expected different hash for different content")
	}
}

func TestHashFileNotFound(t *testing.T) {
	_, err := hashFile("/nonexistent/file.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// MockTool for testing CachedRunner
type mockTool struct {
	runCount int
	result   *Result
	err      error
}

func (m *mockTool) Name() string {
	return "mock"
}

func (m *mockTool) Run(ctx context.Context, sourceFile, testFile string, cfg MutationConfig) (*Result, error) {
	m.runCount++
	return m.result, m.err
}

func (m *mockTool) IsAvailable(ctx context.Context) bool {
	return true
}

func TestCachedRunner(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	mock := &mockTool{
		result: &Result{Score: 90.0, Total: 10, Killed: 9},
	}

	runner := NewCachedRunner(mock, cache)

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	ctx := context.Background()

	// First run - should call mock
	result1, err := runner.Run(ctx, sourceFile, testFile, MutationConfig{})
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if mock.runCount != 1 {
		t.Errorf("expected 1 call to mock, got %d", mock.runCount)
	}
	if result1.Score != 90.0 {
		t.Errorf("expected score 90.0, got %f", result1.Score)
	}

	// Second run - should use cache
	result2, err := runner.Run(ctx, sourceFile, testFile, MutationConfig{})
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if mock.runCount != 1 {
		t.Errorf("expected still 1 call to mock, got %d", mock.runCount)
	}
	if result2.Score != 90.0 {
		t.Errorf("expected score 90.0, got %f", result2.Score)
	}
}

func TestCachedRunnerName(t *testing.T) {
	cache, _ := NewCache(&CacheConfig{Enabled: false})
	mock := &mockTool{}
	runner := NewCachedRunner(mock, cache)

	if runner.Name() != "mock (cached)" {
		t.Errorf("expected 'mock (cached)', got '%s'", runner.Name())
	}
}

func TestCachedRunnerIsAvailable(t *testing.T) {
	cache, _ := NewCache(&CacheConfig{Enabled: false})
	mock := &mockTool{}
	runner := NewCachedRunner(mock, cache)

	if !runner.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable to return true")
	}
}

func TestCachedRunnerDoesNotCacheErrors(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &CacheConfig{
		Enabled:       true,
		Directory:     tmpDir,
		TTL:           time.Hour,
		MaxEntries:    10,
		PersistToDisk: false,
	}

	cache, err := NewCache(cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	mock := &mockTool{
		result: &Result{Score: 0, Error: "mutation failed"},
	}

	runner := NewCachedRunner(mock, cache)

	sourceFile := filepath.Join(tmpDir, "source.go")
	testFile := filepath.Join(tmpDir, "source_test.go")
	os.WriteFile(sourceFile, []byte("package main"), 0644)
	os.WriteFile(testFile, []byte("package main"), 0644)

	ctx := context.Background()

	// First run - returns error result
	runner.Run(ctx, sourceFile, testFile, MutationConfig{})

	// Second run - should NOT use cache due to error
	runner.Run(ctx, sourceFile, testFile, MutationConfig{})

	if mock.runCount != 2 {
		t.Errorf("expected 2 calls to mock (error not cached), got %d", mock.runCount)
	}
}
