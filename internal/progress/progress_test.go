package progress

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNewSpinner(t *testing.T) {
	s := NewSpinner("Loading...")
	if s == nil {
		t.Fatal("expected spinner to be created")
	}
	if s.message != "Loading..." {
		t.Errorf("expected message 'Loading...', got '%s'", s.message)
	}
	if len(s.frames) == 0 {
		t.Error("expected frames to be set")
	}
}

func TestSpinner_StartStop(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("Test")
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	// Should not panic on double stop
	s.Stop()
}

func TestSpinner_StartTwice(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("Test")
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	s.Start() // Should be no-op
	time.Sleep(30 * time.Millisecond)
	s.Stop()
}

func TestSpinner_StopWithMessage(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("Test")
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.StopWithMessage("✓", "Done")

	output := buf.String()
	if !strings.Contains(output, "Done") {
		t.Errorf("expected output to contain 'Done', got '%s'", output)
	}
}

func TestSpinner_StopWithMessageNotRunning(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("Test")
	s.writer = &buf

	// Should not panic when not running
	s.StopWithMessage("✓", "Done")

	output := buf.String()
	if output != "" {
		t.Errorf("expected no output when not running, got '%s'", output)
	}
}

func TestSpinner_Success(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("Test")
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Success("Completed")

	output := buf.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected output to contain '✓', got '%s'", output)
	}
}

func TestSpinner_Fail(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("Test")
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Fail("Failed")

	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected output to contain '✗', got '%s'", output)
	}
}

func TestSpinner_UpdateMessage(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("Initial")
	s.writer = &buf
	s.interval = 10 * time.Millisecond

	s.Start()
	time.Sleep(20 * time.Millisecond)
	s.UpdateMessage("Updated")
	time.Sleep(20 * time.Millisecond)
	s.Stop()

	if s.message != "Updated" {
		t.Errorf("expected message to be 'Updated', got '%s'", s.message)
	}
}

func TestNewBar(t *testing.T) {
	b := NewBar(100, "Progress")
	if b == nil {
		t.Fatal("expected bar to be created")
	}
	if b.total != 100 {
		t.Errorf("expected total 100, got %d", b.total)
	}
	if b.message != "Progress" {
		t.Errorf("expected message 'Progress', got '%s'", b.message)
	}
}

func TestBar_Update(t *testing.T) {
	var buf bytes.Buffer
	b := NewBar(100, "Progress")
	b.writer = &buf

	b.Update(50)

	output := buf.String()
	if !strings.Contains(output, "50/100") {
		t.Errorf("expected output to contain '50/100', got '%s'", output)
	}
	if !strings.Contains(output, "50%") {
		t.Errorf("expected output to contain '50%%', got '%s'", output)
	}
}

func TestBar_Increment(t *testing.T) {
	var buf bytes.Buffer
	b := NewBar(10, "Progress")
	b.writer = &buf

	b.Increment()
	if b.current != 1 {
		t.Errorf("expected current 1, got %d", b.current)
	}

	b.Increment()
	if b.current != 2 {
		t.Errorf("expected current 2, got %d", b.current)
	}
}

func TestBar_SetMessage(t *testing.T) {
	var buf bytes.Buffer
	b := NewBar(10, "Initial")
	b.writer = &buf

	b.SetMessage("Updated")

	if b.message != "Updated" {
		t.Errorf("expected message 'Updated', got '%s'", b.message)
	}
}

func TestBar_Done(t *testing.T) {
	var buf bytes.Buffer
	b := NewBar(10, "Progress")
	b.writer = &buf

	b.Update(5)
	b.Done()

	if b.current != b.total {
		t.Errorf("expected current to equal total, got %d vs %d", b.current, b.total)
	}
}

func TestBar_ZeroTotal(t *testing.T) {
	var buf bytes.Buffer
	b := NewBar(0, "Progress")
	b.writer = &buf

	b.Update(0) // Should not panic
	if buf.Len() != 0 {
		t.Error("expected no output for zero total")
	}
}

func TestBar_OverflowFilled(t *testing.T) {
	var buf bytes.Buffer
	b := NewBar(10, "Progress")
	b.writer = &buf

	b.Update(15) // More than total

	output := buf.String()
	// Bar fill is capped at width, but percentage shows actual value
	if !strings.Contains(output, "15/10") {
		t.Errorf("expected output to contain '15/10', got '%s'", output)
	}
}

func TestNewMultiProgress(t *testing.T) {
	mp := NewMultiProgress()
	if mp == nil {
		t.Fatal("expected multi-progress to be created")
	}
	if mp.items == nil {
		t.Error("expected items map to be initialized")
	}
}

func TestMultiProgress_Add(t *testing.T) {
	mp := NewMultiProgress()
	mp.Add("task1", 10)

	if _, ok := mp.items["task1"]; !ok {
		t.Error("expected task1 to be added")
	}
	if mp.items["task1"].Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", mp.items["task1"].Status)
	}
}

func TestMultiProgress_Start(t *testing.T) {
	mp := NewMultiProgress()
	mp.Add("task1", 10)
	mp.Start("task1")

	if mp.items["task1"].Status != "running" {
		t.Errorf("expected status 'running', got '%s'", mp.items["task1"].Status)
	}
}

func TestMultiProgress_Update(t *testing.T) {
	mp := NewMultiProgress()
	mp.Add("task1", 10)
	mp.Update("task1", 5)

	if mp.items["task1"].Current != 5 {
		t.Errorf("expected current 5, got %d", mp.items["task1"].Current)
	}
}

func TestMultiProgress_Done(t *testing.T) {
	mp := NewMultiProgress()
	mp.Add("task1", 10)
	mp.Done("task1")

	if mp.items["task1"].Status != "done" {
		t.Errorf("expected status 'done', got '%s'", mp.items["task1"].Status)
	}
	if mp.items["task1"].Current != 10 {
		t.Errorf("expected current 10, got %d", mp.items["task1"].Current)
	}
}

func TestMultiProgress_Error(t *testing.T) {
	mp := NewMultiProgress()
	mp.Add("task1", 10)
	mp.Error("task1")

	if mp.items["task1"].Status != "error" {
		t.Errorf("expected status 'error', got '%s'", mp.items["task1"].Status)
	}
}

func TestMultiProgress_Render(t *testing.T) {
	mp := NewMultiProgress()
	mp.Add("task1", 10)
	mp.Update("task1", 5)
	mp.Start("task1")

	output := mp.Render()
	if !strings.Contains(output, "task1") {
		t.Errorf("expected output to contain 'task1', got '%s'", output)
	}
	if !strings.Contains(output, "5/10") {
		t.Errorf("expected output to contain '5/10', got '%s'", output)
	}
}

func TestMultiProgress_RenderNoTotal(t *testing.T) {
	mp := NewMultiProgress()
	mp.Add("task1", 0)
	mp.Start("task1")

	output := mp.Render()
	if !strings.Contains(output, "task1") {
		t.Errorf("expected output to contain 'task1', got '%s'", output)
	}
	if !strings.Contains(output, "running") {
		t.Errorf("expected output to contain 'running', got '%s'", output)
	}
}

func TestMultiProgress_UpdateNonExistent(t *testing.T) {
	mp := NewMultiProgress()
	// Should not panic
	mp.Update("nonexistent", 5)
	mp.Start("nonexistent")
	mp.Done("nonexistent")
	mp.Error("nonexistent")
}

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		icon   string
	}{
		{"pending", "○"},
		{"running", "◐"},
		{"done", "●"},
		{"error", "✗"},
		{"unknown", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			icon := getStatusIcon(tt.status)
			if icon != tt.icon {
				t.Errorf("expected icon '%s', got '%s'", tt.icon, icon)
			}
		})
	}
}

func TestNewPipeline(t *testing.T) {
	p := NewPipeline("phase1", "phase2", "phase3")
	if p == nil {
		t.Fatal("expected pipeline to be created")
	}
	if len(p.phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(p.phases))
	}
}

func TestPipeline_SetVerbose(t *testing.T) {
	p := NewPipeline("phase1")
	p.SetVerbose(true)
	if !p.verbose {
		t.Error("expected verbose to be true")
	}
}

func TestPipeline_StartPhase(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline("phase1", "phase2")
	p.writer = &buf

	p.StartPhase("phase1", 10)

	if p.current != 0 {
		t.Errorf("expected current 0, got %d", p.current)
	}
	if p.phases[0].Total != 10 {
		t.Errorf("expected total 10, got %d", p.phases[0].Total)
	}
}

func TestPipeline_UpdatePhase(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline("phase1")
	p.writer = &buf

	p.StartPhase("phase1", 10)
	p.UpdatePhase(5, "Processing...")

	if p.phases[0].Current != 5 {
		t.Errorf("expected current 5, got %d", p.phases[0].Current)
	}
}

func TestPipeline_UpdatePhaseNoTotal(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline("phase1")
	p.writer = &buf

	p.StartPhase("phase1", 0)
	p.UpdatePhase(0, "Processing...")

	output := buf.String()
	if !strings.Contains(output, "Processing...") {
		t.Errorf("expected output to contain 'Processing...', got '%s'", output)
	}
}

func TestPipeline_UpdatePhaseOutOfBounds(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline()
	p.writer = &buf

	// Should not panic
	p.UpdatePhase(5, "Processing...")
}

func TestPipeline_CompletePhase(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline("phase1")
	p.writer = &buf

	p.StartPhase("phase1", 10)
	time.Sleep(10 * time.Millisecond)
	p.CompletePhase("Completed")

	output := buf.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected output to contain '✓', got '%s'", output)
	}
}

func TestPipeline_CompletePhaseOutOfBounds(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline()
	p.writer = &buf

	// Should not panic
	p.CompletePhase("Done")
}

func TestPipeline_Summary(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline("phase1", "phase2")
	p.writer = &buf

	p.StartPhase("phase1", 10)
	p.CompletePhase("Done")

	summary := p.Summary()
	if !strings.Contains(summary, "Pipeline Summary") {
		t.Errorf("expected summary to contain 'Pipeline Summary', got '%s'", summary)
	}
	if !strings.Contains(summary, "phase1") {
		t.Errorf("expected summary to contain 'phase1', got '%s'", summary)
	}
}

func TestGetPhaseIcon(t *testing.T) {
	tests := []struct {
		index int
		want  string
	}{
		{0, "1️⃣"},
		{1, "2️⃣"},
		{9, "🔟"},
		{10, "•"},
		{100, "•"},
	}

	for _, tt := range tests {
		icon := getPhaseIcon(tt.index)
		if icon != tt.want {
			t.Errorf("getPhaseIcon(%d) = '%s', want '%s'", tt.index, icon, tt.want)
		}
	}
}

func TestWithProgress_Success(t *testing.T) {
	err := WithProgress("Test operation", func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestWithProgress_Error(t *testing.T) {
	expectedErr := bytes.ErrTooLarge
	err := WithProgress("Test operation", func() error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestNewTable(t *testing.T) {
	tbl := NewTable("Name", "Age", "City")
	if tbl == nil {
		t.Fatal("expected table to be created")
	}
	if len(tbl.headers) != 3 {
		t.Errorf("expected 3 headers, got %d", len(tbl.headers))
	}
}

func TestTable_AddRow(t *testing.T) {
	tbl := NewTable("Name", "Age")
	tbl.AddRow("Alice", "30")
	tbl.AddRow("Bob", "25")

	if len(tbl.rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(tbl.rows))
	}
}

func TestTable_Render(t *testing.T) {
	tbl := NewTable("Name", "Age")
	tbl.AddRow("Alice", "30")
	tbl.AddRow("Bob", "25")

	output := tbl.Render()

	if !strings.Contains(output, "Name") {
		t.Errorf("expected output to contain 'Name', got '%s'", output)
	}
	if !strings.Contains(output, "Alice") {
		t.Errorf("expected output to contain 'Alice', got '%s'", output)
	}
	if !strings.Contains(output, "30") {
		t.Errorf("expected output to contain '30', got '%s'", output)
	}
	if !strings.Contains(output, "─") {
		t.Errorf("expected output to contain separator, got '%s'", output)
	}
}

func TestTable_RenderWithLongValues(t *testing.T) {
	tbl := NewTable("A", "B")
	tbl.AddRow("VeryLongName", "Value")

	output := tbl.Render()
	if !strings.Contains(output, "VeryLongName") {
		t.Errorf("expected output to contain 'VeryLongName', got '%s'", output)
	}
}

func TestTable_RenderExtraColumns(t *testing.T) {
	tbl := NewTable("A", "B")
	tbl.AddRow("1", "2", "3", "4") // More columns than headers

	output := tbl.Render()
	if !strings.Contains(output, "3") {
		t.Errorf("expected output to contain extra column '3', got '%s'", output)
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		s     string
		width int
		want  string
	}{
		{"abc", 5, "abc  "},
		{"abc", 3, "abc"},
		{"abc", 2, "abc"},
		{"", 3, "   "},
	}

	for _, tt := range tests {
		got := padRight(tt.s, tt.width)
		if got != tt.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
		}
	}
}
