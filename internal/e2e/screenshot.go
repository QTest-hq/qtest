package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ScreenshotComparer provides visual regression testing capabilities.
type ScreenshotComparer struct {
	config       *ScreenshotConfig
	mu           sync.RWMutex
	baselineDir  string
	currentDir   string
	diffDir      string
}

// ScreenshotConfig configures screenshot comparison.
type ScreenshotConfig struct {
	// BaselineDir is the directory containing baseline screenshots
	BaselineDir string `json:"baselineDir"`
	// CurrentDir is the directory for current test screenshots
	CurrentDir string `json:"currentDir"`
	// DiffDir is the directory for diff images
	DiffDir string `json:"diffDir"`
	// Threshold is the percentage of different pixels allowed (0-1)
	Threshold float64 `json:"threshold"`
	// IgnoreAntialiasing ignores anti-aliasing differences
	IgnoreAntialiasing bool `json:"ignoreAntialiasing"`
	// IgnoreColors compares only luminance
	IgnoreColors bool `json:"ignoreColors"`
	// HighlightColor is the color used to highlight differences
	HighlightColor color.RGBA `json:"highlightColor"`
	// UpdateBaselines automatically updates baselines when they don't exist
	UpdateBaselines bool `json:"updateBaselines"`
	// FailOnMissing fails the test if baseline doesn't exist
	FailOnMissing bool `json:"failOnMissing"`
}

// DefaultScreenshotConfig returns default screenshot configuration.
func DefaultScreenshotConfig() *ScreenshotConfig {
	return &ScreenshotConfig{
		BaselineDir:        "screenshots/baseline",
		CurrentDir:         "screenshots/current",
		DiffDir:            "screenshots/diff",
		Threshold:          0.01, // 1% tolerance
		IgnoreAntialiasing: true,
		IgnoreColors:       false,
		HighlightColor:     color.RGBA{R: 255, G: 0, B: 255, A: 255}, // Magenta
		UpdateBaselines:    false,
		FailOnMissing:      false,
	}
}

// NewScreenshotComparer creates a new screenshot comparer.
func NewScreenshotComparer(config *ScreenshotConfig) (*ScreenshotComparer, error) {
	if config == nil {
		config = DefaultScreenshotConfig()
	}

	comparer := &ScreenshotComparer{
		config:      config,
		baselineDir: config.BaselineDir,
		currentDir:  config.CurrentDir,
		diffDir:     config.DiffDir,
	}

	// Ensure directories exist
	for _, dir := range []string{config.BaselineDir, config.CurrentDir, config.DiffDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return comparer, nil
}

// ComparisonResult represents the result of a screenshot comparison.
type ComparisonResult struct {
	// Name is the screenshot name/identifier
	Name string `json:"name"`
	// Match indicates if screenshots match within threshold
	Match bool `json:"match"`
	// DiffPercentage is the percentage of different pixels
	DiffPercentage float64 `json:"diffPercentage"`
	// DiffPixels is the number of different pixels
	DiffPixels int `json:"diffPixels"`
	// TotalPixels is the total number of pixels
	TotalPixels int `json:"totalPixels"`
	// BaselinePath is the path to the baseline image
	BaselinePath string `json:"baselinePath"`
	// CurrentPath is the path to the current image
	CurrentPath string `json:"currentPath"`
	// DiffPath is the path to the diff image (if generated)
	DiffPath string `json:"diffPath,omitempty"`
	// BaselineHash is the hash of the baseline image
	BaselineHash string `json:"baselineHash,omitempty"`
	// CurrentHash is the hash of the current image
	CurrentHash string `json:"currentHash,omitempty"`
	// Error contains any error that occurred
	Error string `json:"error,omitempty"`
	// NewBaseline indicates if this is a new baseline
	NewBaseline bool `json:"newBaseline,omitempty"`
	// SizeMismatch indicates if images have different dimensions
	SizeMismatch bool `json:"sizeMismatch,omitempty"`
	// Duration is how long the comparison took
	Duration time.Duration `json:"duration"`
}

// Compare compares a current screenshot against its baseline.
func (sc *ScreenshotComparer) Compare(name string, currentData []byte) (*ComparisonResult, error) {
	startTime := time.Now()

	result := &ComparisonResult{
		Name:        name,
		CurrentPath: filepath.Join(sc.currentDir, name+".png"),
	}

	// Save current screenshot
	if err := os.WriteFile(result.CurrentPath, currentData, 0644); err != nil {
		result.Error = fmt.Sprintf("failed to save current screenshot: %v", err)
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("failed to save current screenshot: %w", err)
	}

	result.CurrentHash = hashBytes(currentData)

	// Check for baseline
	baselinePath := filepath.Join(sc.baselineDir, name+".png")
	result.BaselinePath = baselinePath

	baselineData, err := os.ReadFile(baselinePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No baseline exists
			if sc.config.UpdateBaselines {
				// Create new baseline
				if err := os.WriteFile(baselinePath, currentData, 0644); err != nil {
					result.Error = fmt.Sprintf("failed to create baseline: %v", err)
					result.Duration = time.Since(startTime)
					return result, fmt.Errorf("failed to create baseline: %w", err)
				}
				result.NewBaseline = true
				result.Match = true
				result.BaselineHash = result.CurrentHash
				result.Duration = time.Since(startTime)
				return result, nil
			}

			if sc.config.FailOnMissing {
				result.Error = "baseline not found"
				result.Duration = time.Since(startTime)
				return result, fmt.Errorf("baseline not found for %s", name)
			}

			// No baseline, consider it a match
			result.NewBaseline = true
			result.Match = true
			result.Duration = time.Since(startTime)
			return result, nil
		}
		result.Error = fmt.Sprintf("failed to read baseline: %v", err)
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("failed to read baseline: %w", err)
	}

	result.BaselineHash = hashBytes(baselineData)

	// Quick hash comparison
	if result.BaselineHash == result.CurrentHash {
		result.Match = true
		result.DiffPercentage = 0
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Decode images
	baselineImg, err := png.Decode(bytes.NewReader(baselineData))
	if err != nil {
		result.Error = fmt.Sprintf("failed to decode baseline: %v", err)
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("failed to decode baseline: %w", err)
	}

	currentImg, err := png.Decode(bytes.NewReader(currentData))
	if err != nil {
		result.Error = fmt.Sprintf("failed to decode current: %v", err)
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("failed to decode current: %w", err)
	}

	// Compare dimensions
	baseBounds := baselineImg.Bounds()
	currBounds := currentImg.Bounds()

	if baseBounds.Dx() != currBounds.Dx() || baseBounds.Dy() != currBounds.Dy() {
		result.SizeMismatch = true
		result.Match = false
		result.Error = fmt.Sprintf("size mismatch: baseline %dx%d, current %dx%d",
			baseBounds.Dx(), baseBounds.Dy(), currBounds.Dx(), currBounds.Dy())
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Pixel-by-pixel comparison
	width := baseBounds.Dx()
	height := baseBounds.Dy()
	result.TotalPixels = width * height

	diffImg := image.NewRGBA(baseBounds)
	diffPixels := 0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			basePixel := baselineImg.At(x, y)
			currPixel := currentImg.At(x, y)

			isDifferent := sc.pixelsDiffer(basePixel, currPixel)
			if isDifferent {
				diffPixels++
				diffImg.Set(x, y, sc.config.HighlightColor)
			} else {
				// Copy baseline pixel with reduced opacity for context
				r, g, b, _ := basePixel.RGBA()
				diffImg.Set(x, y, color.RGBA{
					R: uint8(r >> 8),
					G: uint8(g >> 8),
					B: uint8(b >> 8),
					A: 128,
				})
			}
		}
	}

	result.DiffPixels = diffPixels
	result.DiffPercentage = float64(diffPixels) / float64(result.TotalPixels)
	result.Match = result.DiffPercentage <= sc.config.Threshold

	// Save diff image if there are differences
	if diffPixels > 0 {
		diffPath := filepath.Join(sc.diffDir, name+"_diff.png")
		result.DiffPath = diffPath

		diffFile, err := os.Create(diffPath)
		if err == nil {
			png.Encode(diffFile, diffImg)
			diffFile.Close()
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// pixelsDiffer checks if two pixels are different.
func (sc *ScreenshotComparer) pixelsDiffer(p1, p2 color.Color) bool {
	r1, g1, b1, a1 := p1.RGBA()
	r2, g2, b2, a2 := p2.RGBA()

	// Convert to 8-bit
	r1, g1, b1, a1 = r1>>8, g1>>8, b1>>8, a1>>8
	r2, g2, b2, a2 = r2>>8, g2>>8, b2>>8, a2>>8

	if sc.config.IgnoreColors {
		// Compare luminance only
		lum1 := 0.299*float64(r1) + 0.587*float64(g1) + 0.114*float64(b1)
		lum2 := 0.299*float64(r2) + 0.587*float64(g2) + 0.114*float64(b2)
		return abs(lum1-lum2) > 1
	}

	if sc.config.IgnoreAntialiasing {
		// Allow small differences for anti-aliasing
		threshold := uint32(10)
		return absDiff(r1, r2) > threshold ||
			absDiff(g1, g2) > threshold ||
			absDiff(b1, b2) > threshold ||
			absDiff(a1, a2) > threshold
	}

	return r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2
}

// CompareFile compares a screenshot file against its baseline.
func (sc *ScreenshotComparer) CompareFile(name string, currentPath string) (*ComparisonResult, error) {
	currentData, err := os.ReadFile(currentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read current screenshot: %w", err)
	}
	return sc.Compare(name, currentData)
}

// UpdateBaseline updates the baseline for a given screenshot.
func (sc *ScreenshotComparer) UpdateBaseline(name string, data []byte) error {
	baselinePath := filepath.Join(sc.baselineDir, name+".png")
	return os.WriteFile(baselinePath, data, 0644)
}

// UpdateBaselineFromCurrent copies current screenshot to baseline.
func (sc *ScreenshotComparer) UpdateBaselineFromCurrent(name string) error {
	currentPath := filepath.Join(sc.currentDir, name+".png")
	baselinePath := filepath.Join(sc.baselineDir, name+".png")

	data, err := os.ReadFile(currentPath)
	if err != nil {
		return fmt.Errorf("failed to read current screenshot: %w", err)
	}

	return os.WriteFile(baselinePath, data, 0644)
}

// BatchComparisonResult represents results of comparing multiple screenshots.
type BatchComparisonResult struct {
	// Results contains individual comparison results
	Results []*ComparisonResult `json:"results"`
	// TotalCount is the total number of comparisons
	TotalCount int `json:"totalCount"`
	// MatchCount is the number of matching screenshots
	MatchCount int `json:"matchCount"`
	// MismatchCount is the number of mismatching screenshots
	MismatchCount int `json:"mismatchCount"`
	// NewCount is the number of new baselines
	NewCount int `json:"newCount"`
	// ErrorCount is the number of errors
	ErrorCount int `json:"errorCount"`
	// Duration is the total comparison duration
	Duration time.Duration `json:"duration"`
}

// CompareAll compares all screenshots in the current directory.
func (sc *ScreenshotComparer) CompareAll() (*BatchComparisonResult, error) {
	startTime := time.Now()

	result := &BatchComparisonResult{
		Results: make([]*ComparisonResult, 0),
	}

	// Find all PNG files in current directory
	entries, err := os.ReadDir(sc.currentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read current directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".png" {
			continue
		}

		name := entry.Name()[:len(entry.Name())-4] // Remove .png extension
		currentPath := filepath.Join(sc.currentDir, entry.Name())

		compResult, _ := sc.CompareFile(name, currentPath)
		result.Results = append(result.Results, compResult)
		result.TotalCount++

		if compResult.Error != "" {
			result.ErrorCount++
		} else if compResult.NewBaseline {
			result.NewCount++
		} else if compResult.Match {
			result.MatchCount++
		} else {
			result.MismatchCount++
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// VisualRegressionReport generates a report of visual regression results.
type VisualRegressionReport struct {
	// Generated is when the report was generated
	Generated time.Time `json:"generated"`
	// Summary contains summary statistics
	Summary ReportSummary `json:"summary"`
	// Failures contains details of failed comparisons
	Failures []*ComparisonResult `json:"failures,omitempty"`
	// Passes contains details of passed comparisons
	Passes []*ComparisonResult `json:"passes,omitempty"`
	// New contains new baselines
	New []*ComparisonResult `json:"new,omitempty"`
}

// ReportSummary contains summary statistics.
type ReportSummary struct {
	TotalTests    int     `json:"totalTests"`
	Passed        int     `json:"passed"`
	Failed        int     `json:"failed"`
	New           int     `json:"new"`
	Errors        int     `json:"errors"`
	PassRate      float64 `json:"passRate"`
	Duration      string  `json:"duration"`
	AverageDiff   float64 `json:"averageDiff"`
	MaxDiff       float64 `json:"maxDiff"`
}

// GenerateReport generates a visual regression report.
func (sc *ScreenshotComparer) GenerateReport(batchResult *BatchComparisonResult) *VisualRegressionReport {
	report := &VisualRegressionReport{
		Generated: time.Now(),
		Summary: ReportSummary{
			TotalTests: batchResult.TotalCount,
			Passed:     batchResult.MatchCount,
			Failed:     batchResult.MismatchCount,
			New:        batchResult.NewCount,
			Errors:     batchResult.ErrorCount,
			Duration:   batchResult.Duration.String(),
		},
	}

	if batchResult.TotalCount > 0 {
		report.Summary.PassRate = float64(batchResult.MatchCount) / float64(batchResult.TotalCount) * 100
	}

	var totalDiff float64
	maxDiff := 0.0

	for _, result := range batchResult.Results {
		if result.NewBaseline {
			report.New = append(report.New, result)
		} else if result.Match {
			report.Passes = append(report.Passes, result)
		} else {
			report.Failures = append(report.Failures, result)
		}

		totalDiff += result.DiffPercentage
		if result.DiffPercentage > maxDiff {
			maxDiff = result.DiffPercentage
		}
	}

	if batchResult.TotalCount > 0 {
		report.Summary.AverageDiff = totalDiff / float64(batchResult.TotalCount) * 100
	}
	report.Summary.MaxDiff = maxDiff * 100

	return report
}

// SaveReport saves the report to a JSON file.
func (sc *ScreenshotComparer) SaveReport(report *VisualRegressionReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Helper functions

func hashBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// CleanupDiffs removes all diff images.
func (sc *ScreenshotComparer) CleanupDiffs() error {
	entries, err := os.ReadDir(sc.diffDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			os.Remove(filepath.Join(sc.diffDir, entry.Name()))
		}
	}
	return nil
}

// CleanupCurrent removes all current screenshots.
func (sc *ScreenshotComparer) CleanupCurrent() error {
	entries, err := os.ReadDir(sc.currentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			os.Remove(filepath.Join(sc.currentDir, entry.Name()))
		}
	}
	return nil
}

// ListBaselines returns a list of all baseline screenshots.
func (sc *ScreenshotComparer) ListBaselines() ([]string, error) {
	entries, err := os.ReadDir(sc.baselineDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var baselines []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".png" {
			baselines = append(baselines, entry.Name()[:len(entry.Name())-4])
		}
	}
	return baselines, nil
}

// HasBaseline checks if a baseline exists for the given name.
func (sc *ScreenshotComparer) HasBaseline(name string) bool {
	baselinePath := filepath.Join(sc.baselineDir, name+".png")
	_, err := os.Stat(baselinePath)
	return err == nil
}
