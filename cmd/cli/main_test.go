package main

import (
	"fmt"
	"testing"
)

func TestIsSupportedExt(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".go", true},
		{".py", true},
		{".js", true},
		{".jsx", true},
		{".ts", true},
		{".tsx", true},
		{".java", true},
		{".rb", false},
		{".php", false},
		{".c", false},
		{".cpp", false},
		{".rs", false},
		{"", false},
		{".txt", false},
		{".md", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := isSupportedExt(tt.ext)
			if got != tt.want {
				t.Errorf("isSupportedExt(%s) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestMaskConnectionString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "postgres URL with password",
			input: "postgres://user:secretpass@localhost:5432/db",
			want:  "postgres://user:****@localhost:5432/db",
		},
		{
			name:  "postgres URL with query params",
			input: "postgres://admin:secret123@host:5432/mydb?sslmode=disable",
			want:  "postgres://admin:****@host:5432/mydb?sslmode=disable",
		},
		{
			name:  "URL without password",
			input: "redis://localhost:6379",
			want:  "redis://localhost:6379",
		},
		{
			name:  "URL with user but no password",
			input: "nats://user@localhost:4222",
			want:  "nats://user@localhost:4222",
		},
		{
			name:  "simple string no URL format",
			input: "localhost:5432",
			want:  "localhost:5432",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskConnectionString(tt.input)
			if got != tt.want {
				t.Errorf("maskConnectionString(%s) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateFilePath_Empty(t *testing.T) {
	_, err := validateFilePath("")
	if err == nil {
		t.Error("validateFilePath('') should return error")
	}
}

func TestValidateFilePath_NonExistent(t *testing.T) {
	_, err := validateFilePath("/nonexistent/path/to/file.go")
	if err == nil {
		t.Error("validateFilePath with non-existent file should return error")
	}
}

func TestValidateDirPath_Empty(t *testing.T) {
	_, err := validateDirPath("")
	if err == nil {
		t.Error("validateDirPath('') should return error")
	}
}

func TestValidateDirPath_NonExistent(t *testing.T) {
	_, err := validateDirPath("/nonexistent/directory/path")
	if err == nil {
		t.Error("validateDirPath with non-existent directory should return error")
	}
}

func TestValidateDirPath_CurrentDir(t *testing.T) {
	// Current directory should be valid
	path, err := validateDirPath(".")
	if err != nil {
		t.Errorf("validateDirPath('.') should not error: %v", err)
	}
	if path == "" {
		t.Error("validateDirPath('.') should return non-empty path")
	}
}

func TestGetMethodIcon(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{"GET", "🔵"},
		{"POST", "🟢"},
		{"PUT", "🟡"},
		{"PATCH", "🟡"},
		{"DELETE", "🔴"},
		{"OPTIONS", "⚪"},
		{"", "⚪"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := getMethodIcon(tt.method); got != tt.want {
				t.Errorf("getMethodIcon(%s) = %s, want %s", tt.method, got, tt.want)
			}
		})
	}
}

func TestGetPriorityIcon(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{100, "🔴"},
		{90, "🔴"},
		{80, "🟠"},
		{70, "🟠"},
		{60, "🟡"},
		{50, "🟡"},
		{40, "🟢"},
		{0, "🟢"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("priority_%d", tt.priority), func(t *testing.T) {
			if got := getPriorityIcon(tt.priority); got != tt.want {
				t.Errorf("getPriorityIcon(%d) = %s, want %s", tt.priority, got, tt.want)
			}
		})
	}
}

func TestFormatTargetKind(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"endpoint", "API"},
		{"function", "FN"},
		{"method", "MTH"},
		{"class", "CLS"},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := formatTargetKind(tt.kind); got != tt.want {
				t.Errorf("formatTargetKind(%s) = %s, want %s", tt.kind, got, tt.want)
			}
		})
	}
}
