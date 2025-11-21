package mutation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

// ReportFormat represents the output format for mutation reports
type ReportFormat string

const (
	FormatHTML ReportFormat = "html"
	FormatJSON ReportFormat = "json"
	FormatText ReportFormat = "text"
)

// Reporter generates mutation testing reports
type Reporter struct {
	outputDir string
}

// NewReporter creates a new mutation report generator
func NewReporter(outputDir string) *Reporter {
	return &Reporter{outputDir: outputDir}
}

// GenerateReport creates a report in the specified format
func (r *Reporter) GenerateReport(result *Result, format ReportFormat) (string, error) {
	switch format {
	case FormatHTML:
		return r.generateHTMLReport(result)
	case FormatJSON:
		return r.generateJSONReport(result)
	case FormatText:
		return r.generateTextReport(result)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// generateHTMLReport creates an HTML mutation report
func (r *Reporter) generateHTMLReport(result *Result) (string, error) {
	data := htmlReportData{
		Title:         "Mutation Testing Report",
		GeneratedAt:   time.Now().Format("2006-01-02 15:04:05"),
		SourceFile:    result.SourceFile,
		TestFile:      result.TestFile,
		TotalMutants:  result.Total,
		Killed:        result.Killed,
		Survived:      result.Survived,
		Timeout:       result.Timeout,
		Score:         result.Score * 100,
		ScoreFormatted: fmt.Sprintf("%.1f%%", result.Score*100),
		Quality:       result.Quality(),
		QualityClass:  qualityClass(result.Quality()),
		Duration:      result.Duration.String(),
		Mutants:       result.Mutants,
	}

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Write to file
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("mutation-report-%s.html", time.Now().Format("20060102-150405"))
	outputPath := filepath.Join(r.outputDir, filename)

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	return outputPath, nil
}

// generateJSONReport creates a JSON mutation report
func (r *Reporter) generateJSONReport(result *Result) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("mutation-report-%s.json", time.Now().Format("20060102-150405"))
	outputPath := filepath.Join(r.outputDir, filename)

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	return outputPath, nil
}

// generateTextReport creates a plain text mutation report
func (r *Reporter) generateTextReport(result *Result) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("================================================================================\n")
	buf.WriteString("                        MUTATION TESTING REPORT\n")
	buf.WriteString("================================================================================\n\n")

	buf.WriteString(fmt.Sprintf("Source File: %s\n", result.SourceFile))
	buf.WriteString(fmt.Sprintf("Test File:   %s\n", result.TestFile))
	buf.WriteString(fmt.Sprintf("Generated:   %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	buf.WriteString("SUMMARY\n")
	buf.WriteString("-------\n")
	buf.WriteString(fmt.Sprintf("  Total Mutants:  %d\n", result.Total))
	buf.WriteString(fmt.Sprintf("  Killed:         %d\n", result.Killed))
	buf.WriteString(fmt.Sprintf("  Survived:       %d\n", result.Survived))
	buf.WriteString(fmt.Sprintf("  Timeout:        %d\n", result.Timeout))
	buf.WriteString(fmt.Sprintf("  Score:          %.1f%%\n", result.Score*100))
	buf.WriteString(fmt.Sprintf("  Quality:        %s\n", result.Quality()))
	buf.WriteString(fmt.Sprintf("  Duration:       %s\n\n", result.Duration))

	if len(result.Mutants) > 0 {
		buf.WriteString("MUTANT DETAILS\n")
		buf.WriteString("--------------\n\n")

		for i, m := range result.Mutants {
			statusIcon := "?"
			switch m.Status {
			case StatusKilled:
				statusIcon = "✓"
			case StatusSurvived:
				statusIcon = "✗"
			case StatusTimeout:
				statusIcon = "⏱"
			case StatusError:
				statusIcon = "!"
			}

			buf.WriteString(fmt.Sprintf("[%s] Mutant #%d (Line %d)\n", statusIcon, i+1, m.Line))
			buf.WriteString(fmt.Sprintf("    Type:   %s\n", m.Type))
			buf.WriteString(fmt.Sprintf("    Status: %s\n", m.Status))
			if m.Description != "" {
				buf.WriteString(fmt.Sprintf("    Desc:   %s\n", m.Description))
			}
			buf.WriteString("\n")
		}
	}

	buf.WriteString("================================================================================\n")

	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("mutation-report-%s.txt", time.Now().Format("20060102-150405"))
	outputPath := filepath.Join(r.outputDir, filename)

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write report: %w", err)
	}

	return outputPath, nil
}

// htmlReportData holds data for the HTML template
type htmlReportData struct {
	Title          string
	GeneratedAt    string
	SourceFile     string
	TestFile       string
	TotalMutants   int
	Killed         int
	Survived       int
	Timeout        int
	Score          float64
	ScoreFormatted string
	Quality        string
	QualityClass   string
	Duration       string
	Mutants        []Mutant
}

// qualityClass returns the CSS class for quality level
func qualityClass(quality string) string {
	switch quality {
	case "good":
		return "quality-good"
	case "acceptable":
		return "quality-acceptable"
	default:
		return "quality-poor"
	}
}

// statusIcon returns an icon for mutant status
func statusIcon(status string) string {
	switch status {
	case StatusKilled:
		return "✓"
	case StatusSurvived:
		return "✗"
	case StatusTimeout:
		return "⏱"
	default:
		return "?"
	}
}

// statusClass returns the CSS class for status
func statusClass(status string) string {
	switch status {
	case StatusKilled:
		return "status-killed"
	case StatusSurvived:
		return "status-survived"
	case StatusTimeout:
		return "status-timeout"
	default:
		return "status-error"
	}
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            line-height: 1.6;
            color: #333;
            background: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            border-radius: 10px;
            margin-bottom: 20px;
        }
        .header h1 {
            font-size: 2em;
            margin-bottom: 10px;
        }
        .header .subtitle {
            opacity: 0.9;
        }
        .card {
            background: white;
            border-radius: 10px;
            padding: 20px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .card h2 {
            color: #667eea;
            margin-bottom: 15px;
            padding-bottom: 10px;
            border-bottom: 2px solid #f0f0f0;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 15px;
        }
        .stat-box {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            text-align: center;
        }
        .stat-value {
            font-size: 2em;
            font-weight: bold;
            color: #333;
        }
        .stat-label {
            color: #666;
            font-size: 0.9em;
            margin-top: 5px;
        }
        .score-display {
            text-align: center;
            padding: 30px;
        }
        .score-circle {
            width: 150px;
            height: 150px;
            border-radius: 50%;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            font-size: 2.5em;
            font-weight: bold;
            color: white;
            margin-bottom: 15px;
        }
        .quality-good { background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%); }
        .quality-acceptable { background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); }
        .quality-poor { background: linear-gradient(135deg, #eb3349 0%, #f45c43 100%); }
        .quality-label {
            font-size: 1.2em;
            font-weight: 600;
            text-transform: uppercase;
        }
        .file-info {
            display: grid;
            grid-template-columns: 100px 1fr;
            gap: 10px;
        }
        .file-info dt {
            color: #666;
            font-weight: 600;
        }
        .file-info dd {
            font-family: 'Monaco', 'Consolas', monospace;
            background: #f8f9fa;
            padding: 5px 10px;
            border-radius: 4px;
        }
        .mutant-list {
            list-style: none;
        }
        .mutant-item {
            border: 1px solid #e0e0e0;
            border-radius: 8px;
            margin-bottom: 10px;
            overflow: hidden;
        }
        .mutant-header {
            display: flex;
            align-items: center;
            padding: 15px;
            background: #f8f9fa;
            gap: 15px;
        }
        .mutant-status {
            width: 30px;
            height: 30px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-weight: bold;
        }
        .status-killed { background: #38ef7d; }
        .status-survived { background: #f45c43; }
        .status-timeout { background: #ffc107; color: #333; }
        .status-error { background: #6c757d; }
        .mutant-info {
            flex: 1;
        }
        .mutant-type {
            font-weight: 600;
            color: #333;
        }
        .mutant-line {
            color: #666;
            font-size: 0.9em;
        }
        .mutant-description {
            padding: 15px;
            background: white;
            font-family: 'Monaco', 'Consolas', monospace;
            font-size: 0.9em;
            border-top: 1px solid #e0e0e0;
        }
        .footer {
            text-align: center;
            color: #666;
            padding: 20px;
            font-size: 0.9em;
        }
        .no-mutants {
            text-align: center;
            padding: 40px;
            color: #666;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.Title}}</h1>
            <div class="subtitle">Generated on {{.GeneratedAt}}</div>
        </div>

        <div class="card">
            <h2>Files</h2>
            <dl class="file-info">
                <dt>Source:</dt>
                <dd>{{.SourceFile}}</dd>
                <dt>Test:</dt>
                <dd>{{.TestFile}}</dd>
            </dl>
        </div>

        <div class="card">
            <h2>Score</h2>
            <div class="score-display">
                <div class="score-circle {{.QualityClass}}">
                    {{.ScoreFormatted}}
                </div>
                <div class="quality-label {{.QualityClass}}">{{.Quality}}</div>
            </div>
        </div>

        <div class="card">
            <h2>Statistics</h2>
            <div class="stats-grid">
                <div class="stat-box">
                    <div class="stat-value">{{.TotalMutants}}</div>
                    <div class="stat-label">Total Mutants</div>
                </div>
                <div class="stat-box">
                    <div class="stat-value" style="color: #38ef7d;">{{.Killed}}</div>
                    <div class="stat-label">Killed</div>
                </div>
                <div class="stat-box">
                    <div class="stat-value" style="color: #f45c43;">{{.Survived}}</div>
                    <div class="stat-label">Survived</div>
                </div>
                <div class="stat-box">
                    <div class="stat-value" style="color: #ffc107;">{{.Timeout}}</div>
                    <div class="stat-label">Timeout</div>
                </div>
                <div class="stat-box">
                    <div class="stat-value">{{.Duration}}</div>
                    <div class="stat-label">Duration</div>
                </div>
            </div>
        </div>

        <div class="card">
            <h2>Mutant Details</h2>
            {{if .Mutants}}
            <ul class="mutant-list">
                {{range .Mutants}}
                <li class="mutant-item">
                    <div class="mutant-header">
                        <div class="mutant-status {{if eq .Status "killed"}}status-killed{{else if eq .Status "survived"}}status-survived{{else if eq .Status "timeout"}}status-timeout{{else}}status-error{{end}}">
                            {{if eq .Status "killed"}}✓{{else if eq .Status "survived"}}✗{{else if eq .Status "timeout"}}⏱{{else}}!{{end}}
                        </div>
                        <div class="mutant-info">
                            <div class="mutant-type">{{.Type}}</div>
                            <div class="mutant-line">Line {{.Line}} • {{.Status}}</div>
                        </div>
                    </div>
                    {{if .Description}}
                    <div class="mutant-description">{{.Description}}</div>
                    {{end}}
                </li>
                {{end}}
            </ul>
            {{else}}
            <div class="no-mutants">
                <p>No mutant details available</p>
            </div>
            {{end}}
        </div>

        <div class="footer">
            Generated by QTest Mutation Testing
        </div>
    </div>
</body>
</html>`
