// Package progress provides CLI progress output utilities
package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Spinner provides an animated spinner for long-running operations
type Spinner struct {
	mu       sync.Mutex
	message  string
	done     bool
	frames   []string
	current  int
	writer   io.Writer
	interval time.Duration
	stopCh   chan struct{}
	running  bool
}

// NewSpinner creates a new spinner with a message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message:  message,
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		writer:   os.Stdout,
		interval: 80 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.done = false
	s.mu.Unlock()

	go s.animate()
}

// Stop stops the spinner and clears the line
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.clearLine()
}

// StopWithMessage stops the spinner and displays a final message
func (s *Spinner) StopWithMessage(icon, message string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.clearLine()
	fmt.Fprintf(s.writer, "%s %s\n", icon, message)
}

// Success stops with a success message
func (s *Spinner) Success(message string) {
	s.StopWithMessage("✓", message)
}

// Fail stops with a failure message
func (s *Spinner) Fail(message string) {
	s.StopWithMessage("✗", message)
}

// UpdateMessage updates the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

func (s *Spinner) animate() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			frame := s.frames[s.current]
			msg := s.message
			s.current = (s.current + 1) % len(s.frames)
			s.mu.Unlock()

			s.clearLine()
			fmt.Fprintf(s.writer, "%s %s", frame, msg)
		}
	}
}

func (s *Spinner) clearLine() {
	fmt.Fprintf(s.writer, "\r\033[K")
}

// Bar provides a progress bar
type Bar struct {
	mu       sync.Mutex
	total    int
	current  int
	width    int
	message  string
	writer   io.Writer
	startTime time.Time
}

// NewBar creates a new progress bar
func NewBar(total int, message string) *Bar {
	return &Bar{
		total:     total,
		width:     40,
		message:   message,
		writer:    os.Stdout,
		startTime: time.Now(),
	}
}

// Update updates the progress bar
func (b *Bar) Update(current int) {
	b.mu.Lock()
	b.current = current
	b.mu.Unlock()

	b.render()
}

// Increment increments the progress by 1
func (b *Bar) Increment() {
	b.mu.Lock()
	b.current++
	b.mu.Unlock()

	b.render()
}

// SetMessage updates the progress message
func (b *Bar) SetMessage(message string) {
	b.mu.Lock()
	b.message = message
	b.mu.Unlock()

	b.render()
}

// Done completes the progress bar
func (b *Bar) Done() {
	b.mu.Lock()
	b.current = b.total
	b.mu.Unlock()

	b.render()
	fmt.Fprintln(b.writer)
}

func (b *Bar) render() {
	b.mu.Lock()
	current := b.current
	total := b.total
	message := b.message
	b.mu.Unlock()

	if total == 0 {
		return
	}

	percentage := float64(current) / float64(total)
	filled := int(percentage * float64(b.width))
	if filled > b.width {
		filled = b.width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", b.width-filled)

	// Calculate ETA
	elapsed := time.Since(b.startTime)
	var eta string
	if current > 0 && current < total {
		remaining := time.Duration(float64(elapsed) / float64(current) * float64(total-current))
		if remaining < time.Minute {
			eta = fmt.Sprintf("ETA: %ds", int(remaining.Seconds()))
		} else {
			eta = fmt.Sprintf("ETA: %dm%ds", int(remaining.Minutes()), int(remaining.Seconds())%60)
		}
	} else if current >= total {
		eta = fmt.Sprintf("Done in %s", elapsed.Round(time.Second))
	}

	fmt.Fprintf(b.writer, "\r%s [%s] %d/%d %.0f%% %s",
		message, bar, current, total, percentage*100, eta)
}

// MultiProgress manages multiple progress items
type MultiProgress struct {
	mu     sync.Mutex
	items  map[string]*ProgressItem
	writer io.Writer
}

// ProgressItem represents a single progress item
type ProgressItem struct {
	Name    string
	Current int
	Total   int
	Status  string // pending, running, done, error
}

// NewMultiProgress creates a new multi-progress tracker
func NewMultiProgress() *MultiProgress {
	return &MultiProgress{
		items:  make(map[string]*ProgressItem),
		writer: os.Stdout,
	}
}

// Add adds a new progress item
func (m *MultiProgress) Add(name string, total int) {
	m.mu.Lock()
	m.items[name] = &ProgressItem{
		Name:   name,
		Total:  total,
		Status: "pending",
	}
	m.mu.Unlock()
}

// Start marks an item as running
func (m *MultiProgress) Start(name string) {
	m.mu.Lock()
	if item, ok := m.items[name]; ok {
		item.Status = "running"
	}
	m.mu.Unlock()
}

// Update updates the progress of an item
func (m *MultiProgress) Update(name string, current int) {
	m.mu.Lock()
	if item, ok := m.items[name]; ok {
		item.Current = current
	}
	m.mu.Unlock()
}

// Done marks an item as complete
func (m *MultiProgress) Done(name string) {
	m.mu.Lock()
	if item, ok := m.items[name]; ok {
		item.Current = item.Total
		item.Status = "done"
	}
	m.mu.Unlock()
}

// Error marks an item as failed
func (m *MultiProgress) Error(name string) {
	m.mu.Lock()
	if item, ok := m.items[name]; ok {
		item.Status = "error"
	}
	m.mu.Unlock()
}

// Render displays all progress items
func (m *MultiProgress) Render() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder
	for _, item := range m.items {
		icon := getStatusIcon(item.Status)
		if item.Total > 0 {
			percentage := float64(item.Current) / float64(item.Total) * 100
			sb.WriteString(fmt.Sprintf("%s %-20s %d/%d (%.0f%%)\n",
				icon, item.Name, item.Current, item.Total, percentage))
		} else {
			sb.WriteString(fmt.Sprintf("%s %-20s %s\n",
				icon, item.Name, item.Status))
		}
	}
	return sb.String()
}

func getStatusIcon(status string) string {
	switch status {
	case "pending":
		return "○"
	case "running":
		return "◐"
	case "done":
		return "●"
	case "error":
		return "✗"
	default:
		return "?"
	}
}

// Phase represents a processing phase
type Phase struct {
	Name      string
	Current   int
	Total     int
	StartTime time.Time
	EndTime   time.Time
}

// Pipeline tracks progress across multiple phases
type Pipeline struct {
	mu       sync.Mutex
	phases   []*Phase
	current  int
	writer   io.Writer
	verbose  bool
}

// NewPipeline creates a new pipeline progress tracker
func NewPipeline(phaseNames ...string) *Pipeline {
	phases := make([]*Phase, len(phaseNames))
	for i, name := range phaseNames {
		phases[i] = &Phase{Name: name}
	}
	return &Pipeline{
		phases: phases,
		writer: os.Stdout,
	}
}

// SetVerbose enables verbose output
func (p *Pipeline) SetVerbose(v bool) {
	p.verbose = v
}

// StartPhase starts a phase
func (p *Pipeline) StartPhase(name string, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, phase := range p.phases {
		if phase.Name == name {
			phase.Total = total
			phase.StartTime = time.Now()
			p.current = i
			break
		}
	}

	fmt.Fprintf(p.writer, "\n%s %s...\n", getPhaseIcon(p.current), name)
}

// UpdatePhase updates the current phase progress
func (p *Pipeline) UpdatePhase(current int, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.current >= len(p.phases) {
		return
	}

	phase := p.phases[p.current]
	phase.Current = current

	if phase.Total > 0 {
		percentage := float64(current) / float64(phase.Total) * 100
		fmt.Fprintf(p.writer, "\r  [%d/%d] %.0f%% %s",
			current, phase.Total, percentage, message)
	} else {
		fmt.Fprintf(p.writer, "\r  %s", message)
	}
}

// CompletePhase completes the current phase
func (p *Pipeline) CompletePhase(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.current >= len(p.phases) {
		return
	}

	phase := p.phases[p.current]
	phase.EndTime = time.Now()

	elapsed := phase.EndTime.Sub(phase.StartTime).Round(time.Millisecond)
	fmt.Fprintf(p.writer, "\r\033[K  ✓ %s (%s)\n", message, elapsed)
}

// Summary returns a summary of all phases
func (p *Pipeline) Summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("\n" + strings.Repeat("=", 50) + "\n")
	sb.WriteString("Pipeline Summary\n")
	sb.WriteString(strings.Repeat("-", 50) + "\n")

	for _, phase := range p.phases {
		if phase.EndTime.IsZero() {
			continue
		}
		elapsed := phase.EndTime.Sub(phase.StartTime).Round(time.Millisecond)
		sb.WriteString(fmt.Sprintf("  %-20s %s\n", phase.Name+":", elapsed))
	}

	return sb.String()
}

func getPhaseIcon(index int) string {
	icons := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}
	if index < len(icons) {
		return icons[index]
	}
	return "•"
}

// WithProgress wraps a function with progress tracking
func WithProgress(message string, fn func() error) error {
	spinner := NewSpinner(message)
	spinner.Start()

	err := fn()

	if err != nil {
		spinner.Fail(fmt.Sprintf("%s: %v", message, err))
	} else {
		spinner.Success(message)
	}

	return err
}

// Table formats data as a table
type Table struct {
	headers []string
	rows    [][]string
	widths  []int
}

// NewTable creates a new table
func NewTable(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{
		headers: headers,
		widths:  widths,
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	for i, c := range cells {
		if i < len(t.widths) && len(c) > t.widths[i] {
			t.widths[i] = len(c)
		}
	}
	t.rows = append(t.rows, cells)
}

// Render returns the formatted table
func (t *Table) Render() string {
	var sb strings.Builder

	// Header
	for i, h := range t.headers {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(padRight(h, t.widths[i]))
	}
	sb.WriteString("\n")

	// Separator
	for i, w := range t.widths {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(strings.Repeat("─", w))
	}
	sb.WriteString("\n")

	// Rows
	for _, row := range t.rows {
		for i, cell := range row {
			if i > 0 {
				sb.WriteString("  ")
			}
			if i < len(t.widths) {
				sb.WriteString(padRight(cell, t.widths[i]))
			} else {
				sb.WriteString(cell)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
