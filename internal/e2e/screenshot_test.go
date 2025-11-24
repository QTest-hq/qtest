package e2e

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultScreenshotConfig(t *testing.T) {
	config := DefaultScreenshotConfig()

	if config.BaselineDir != "screenshots/baseline" {
		t.Errorf("Expected BaselineDir='screenshots/baseline', got %s", config.BaselineDir)
	}
	if config.CurrentDir != "screenshots/current" {
		t.Errorf("Expected CurrentDir='screenshots/current', got %s", config.CurrentDir)
	}
	if config.DiffDir != "screenshots/diff" {
		t.Errorf("Expected DiffDir='screenshots/diff', got %s", config.DiffDir)
	}
	if config.Threshold != 0.01 {
		t.Errorf("Expected Threshold=0.01, got %f", config.Threshold)
	}
	if !config.IgnoreAntialiasing {
		t.Error("Expected IgnoreAntialiasing=true")
	}
	if config.HighlightColor != (color.RGBA{R: 255, G: 0, B: 255, A: 255}) {
		t.Error("Expected magenta highlight color")
	}
}

func TestNewScreenshotComparer(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir: filepath.Join(tempDir, "baseline"),
		CurrentDir:  filepath.Join(tempDir, "current"),
		DiffDir:     filepath.Join(tempDir, "diff"),
		Threshold:   0.01,
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}
	if comparer == nil {
		t.Fatal("Expected non-nil comparer")
	}

	// Check directories were created
	for _, dir := range []string{config.BaselineDir, config.CurrentDir, config.DiffDir} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}

func TestScreenshotComparer_CompareIdentical(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir:     filepath.Join(tempDir, "baseline"),
		CurrentDir:      filepath.Join(tempDir, "current"),
		DiffDir:         filepath.Join(tempDir, "diff"),
		Threshold:       0.01,
		UpdateBaselines: false,
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	// Create a test image
	img := createTestImage(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	imgData := encodeImage(t, img)

	// Set up baseline
	baselinePath := filepath.Join(config.BaselineDir, "test.png")
	if err := os.WriteFile(baselinePath, imgData, 0644); err != nil {
		t.Fatalf("Failed to write baseline: %v", err)
	}

	// Compare identical image
	result, err := comparer.Compare("test", imgData)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if !result.Match {
		t.Error("Expected identical images to match")
	}
	if result.DiffPercentage != 0 {
		t.Errorf("Expected 0%% diff for identical images, got %f%%", result.DiffPercentage*100)
	}
	if result.BaselineHash != result.CurrentHash {
		t.Error("Expected identical hashes")
	}
}

func TestScreenshotComparer_CompareDifferent(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir: filepath.Join(tempDir, "baseline"),
		CurrentDir:  filepath.Join(tempDir, "current"),
		DiffDir:     filepath.Join(tempDir, "diff"),
		Threshold:   0.01,
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	// Create baseline (red)
	baselineImg := createTestImage(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	baselineData := encodeImage(t, baselineImg)

	// Create current (blue) - completely different
	currentImg := createTestImage(100, 100, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	currentData := encodeImage(t, currentImg)

	// Set up baseline
	baselinePath := filepath.Join(config.BaselineDir, "test.png")
	if err := os.WriteFile(baselinePath, baselineData, 0644); err != nil {
		t.Fatalf("Failed to write baseline: %v", err)
	}

	// Compare different image
	result, err := comparer.Compare("test", currentData)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Match {
		t.Error("Expected different images not to match")
	}
	if result.DiffPercentage == 0 {
		t.Error("Expected non-zero diff percentage")
	}
	if result.DiffPixels == 0 {
		t.Error("Expected non-zero diff pixels")
	}
	if result.DiffPath == "" {
		t.Error("Expected diff image to be generated")
	}
}

func TestScreenshotComparer_NewBaseline(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir:     filepath.Join(tempDir, "baseline"),
		CurrentDir:      filepath.Join(tempDir, "current"),
		DiffDir:         filepath.Join(tempDir, "diff"),
		Threshold:       0.01,
		UpdateBaselines: true,
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	// Create a test image (no baseline exists)
	img := createTestImage(50, 50, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	imgData := encodeImage(t, img)

	result, err := comparer.Compare("new_test", imgData)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if !result.NewBaseline {
		t.Error("Expected NewBaseline=true")
	}
	if !result.Match {
		t.Error("Expected Match=true for new baseline")
	}

	// Check baseline was created
	baselinePath := filepath.Join(config.BaselineDir, "new_test.png")
	if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
		t.Error("Expected baseline to be created")
	}
}

func TestScreenshotComparer_SizeMismatch(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir: filepath.Join(tempDir, "baseline"),
		CurrentDir:  filepath.Join(tempDir, "current"),
		DiffDir:     filepath.Join(tempDir, "diff"),
		Threshold:   0.01,
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	// Create baseline (100x100)
	baselineImg := createTestImage(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	baselineData := encodeImage(t, baselineImg)

	// Create current (50x50) - different size
	currentImg := createTestImage(50, 50, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	currentData := encodeImage(t, currentImg)

	// Set up baseline
	baselinePath := filepath.Join(config.BaselineDir, "test.png")
	if err := os.WriteFile(baselinePath, baselineData, 0644); err != nil {
		t.Fatalf("Failed to write baseline: %v", err)
	}

	result, err := comparer.Compare("test", currentData)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if !result.SizeMismatch {
		t.Error("Expected SizeMismatch=true")
	}
	if result.Match {
		t.Error("Expected Match=false for size mismatch")
	}
}

func TestScreenshotComparer_ThresholdTolerance(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir:        filepath.Join(tempDir, "baseline"),
		CurrentDir:         filepath.Join(tempDir, "current"),
		DiffDir:            filepath.Join(tempDir, "diff"),
		Threshold:          0.05, // 5% tolerance
		IgnoreAntialiasing: false,
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	// Create baseline (all red)
	baselineImg := createTestImage(100, 100, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	baselineData := encodeImage(t, baselineImg)

	// Create current with 3% different pixels (within tolerance)
	currentImg := createTestImageWithDiff(100, 100,
		color.RGBA{R: 255, G: 0, B: 0, A: 255},
		color.RGBA{R: 0, G: 0, B: 255, A: 255},
		0.03, // 3% different
	)
	currentData := encodeImage(t, currentImg)

	// Set up baseline
	baselinePath := filepath.Join(config.BaselineDir, "test.png")
	if err := os.WriteFile(baselinePath, baselineData, 0644); err != nil {
		t.Fatalf("Failed to write baseline: %v", err)
	}

	result, err := comparer.Compare("test", currentData)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	// Should match because diff is within 5% threshold
	if !result.Match {
		t.Errorf("Expected Match=true with %f%% diff (threshold 5%%)", result.DiffPercentage*100)
	}
}

func TestScreenshotComparer_UpdateBaseline(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir: filepath.Join(tempDir, "baseline"),
		CurrentDir:  filepath.Join(tempDir, "current"),
		DiffDir:     filepath.Join(tempDir, "diff"),
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	img := createTestImage(50, 50, color.RGBA{R: 128, G: 128, B: 128, A: 255})
	imgData := encodeImage(t, img)

	err = comparer.UpdateBaseline("manual_test", imgData)
	if err != nil {
		t.Fatalf("UpdateBaseline failed: %v", err)
	}

	// Verify baseline exists
	if !comparer.HasBaseline("manual_test") {
		t.Error("Expected baseline to exist")
	}
}

func TestScreenshotComparer_ListBaselines(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir: filepath.Join(tempDir, "baseline"),
		CurrentDir:  filepath.Join(tempDir, "current"),
		DiffDir:     filepath.Join(tempDir, "diff"),
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	// Create some baselines
	for _, name := range []string{"test1", "test2", "test3"} {
		img := createTestImage(10, 10, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		imgData := encodeImage(t, img)
		comparer.UpdateBaseline(name, imgData)
	}

	baselines, err := comparer.ListBaselines()
	if err != nil {
		t.Fatalf("ListBaselines failed: %v", err)
	}

	if len(baselines) != 3 {
		t.Errorf("Expected 3 baselines, got %d", len(baselines))
	}
}

func TestScreenshotComparer_CleanupDiffs(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir: filepath.Join(tempDir, "baseline"),
		CurrentDir:  filepath.Join(tempDir, "current"),
		DiffDir:     filepath.Join(tempDir, "diff"),
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	// Create a diff file
	diffPath := filepath.Join(config.DiffDir, "test_diff.png")
	if err := os.WriteFile(diffPath, []byte("fake"), 0644); err != nil {
		t.Fatalf("Failed to write diff: %v", err)
	}

	err = comparer.CleanupDiffs()
	if err != nil {
		t.Fatalf("CleanupDiffs failed: %v", err)
	}

	// Verify diff was removed
	if _, err := os.Stat(diffPath); !os.IsNotExist(err) {
		t.Error("Expected diff to be removed")
	}
}

func TestComparisonResult_Fields(t *testing.T) {
	result := &ComparisonResult{
		Name:           "test-screenshot",
		Match:          true,
		DiffPercentage: 0.005,
		DiffPixels:     50,
		TotalPixels:    10000,
		BaselinePath:   "/path/to/baseline.png",
		CurrentPath:    "/path/to/current.png",
	}

	if result.Name != "test-screenshot" {
		t.Errorf("Expected Name='test-screenshot', got %s", result.Name)
	}
	if !result.Match {
		t.Error("Expected Match=true")
	}
	if result.DiffPixels != 50 {
		t.Errorf("Expected DiffPixels=50, got %d", result.DiffPixels)
	}
}

func TestBatchComparisonResult_Fields(t *testing.T) {
	result := &BatchComparisonResult{
		TotalCount:    10,
		MatchCount:    8,
		MismatchCount: 1,
		NewCount:      1,
		ErrorCount:    0,
	}

	if result.TotalCount != 10 {
		t.Errorf("Expected TotalCount=10, got %d", result.TotalCount)
	}
	if result.MatchCount != 8 {
		t.Errorf("Expected MatchCount=8, got %d", result.MatchCount)
	}
}

func TestScreenshotComparer_GenerateReport(t *testing.T) {
	tempDir := t.TempDir()
	config := &ScreenshotConfig{
		BaselineDir: filepath.Join(tempDir, "baseline"),
		CurrentDir:  filepath.Join(tempDir, "current"),
		DiffDir:     filepath.Join(tempDir, "diff"),
	}

	comparer, err := NewScreenshotComparer(config)
	if err != nil {
		t.Fatalf("Failed to create comparer: %v", err)
	}

	batchResult := &BatchComparisonResult{
		TotalCount:    5,
		MatchCount:    3,
		MismatchCount: 1,
		NewCount:      1,
		Results: []*ComparisonResult{
			{Name: "pass1", Match: true, DiffPercentage: 0},
			{Name: "pass2", Match: true, DiffPercentage: 0.001},
			{Name: "pass3", Match: true, DiffPercentage: 0.002},
			{Name: "fail1", Match: false, DiffPercentage: 0.15},
			{Name: "new1", NewBaseline: true, Match: true},
		},
	}

	report := comparer.GenerateReport(batchResult)

	if report.Summary.TotalTests != 5 {
		t.Errorf("Expected TotalTests=5, got %d", report.Summary.TotalTests)
	}
	if report.Summary.Passed != 3 {
		t.Errorf("Expected Passed=3, got %d", report.Summary.Passed)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("Expected Failed=1, got %d", report.Summary.Failed)
	}
	if report.Summary.New != 1 {
		t.Errorf("Expected New=1, got %d", report.Summary.New)
	}
	if len(report.Failures) != 1 {
		t.Errorf("Expected 1 failure, got %d", len(report.Failures))
	}
}

func TestHashBytes(t *testing.T) {
	data1 := []byte("test data")
	data2 := []byte("test data")
	data3 := []byte("different data")

	hash1 := hashBytes(data1)
	hash2 := hashBytes(data2)
	hash3 := hashBytes(data3)

	if hash1 != hash2 {
		t.Error("Expected identical hashes for identical data")
	}
	if hash1 == hash3 {
		t.Error("Expected different hashes for different data")
	}
	if len(hash1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("Expected 64 char hash, got %d", len(hash1))
	}
}

func TestAbsDiff(t *testing.T) {
	tests := []struct {
		a, b     uint32
		expected uint32
	}{
		{10, 5, 5},
		{5, 10, 5},
		{10, 10, 0},
		{0, 5, 5},
	}

	for _, tt := range tests {
		result := absDiff(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("absDiff(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{5.5, 5.5},
		{-5.5, 5.5},
		{0, 0},
	}

	for _, tt := range tests {
		result := abs(tt.input)
		if result != tt.expected {
			t.Errorf("abs(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

// Helper functions

func createTestImage(width, height int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func createTestImageWithDiff(width, height int, baseColor, diffColor color.Color, diffPercent float64) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	totalPixels := width * height
	diffPixels := int(float64(totalPixels) * diffPercent)
	diffCount := 0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if diffCount < diffPixels {
				img.Set(x, y, diffColor)
				diffCount++
			} else {
				img.Set(x, y, baseColor)
			}
		}
	}
	return img
}

func encodeImage(t *testing.T, img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("Failed to encode image: %v", err)
	}
	return buf.Bytes()
}
