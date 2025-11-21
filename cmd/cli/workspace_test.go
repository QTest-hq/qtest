package main

import (
	"testing"
)

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "HTTP URL with .git",
			url:  "https://github.com/user/myrepo.git",
			want: "myrepo",
		},
		{
			name: "HTTP URL without .git",
			url:  "https://github.com/user/myrepo",
			want: "myrepo",
		},
		{
			name: "SSH URL",
			url:  "git@github.com:user/myrepo.git",
			want: "myrepo",
		},
		{
			name: "SSH URL without .git",
			url:  "git@github.com:user/myrepo",
			want: "myrepo",
		},
		{
			name: "GitLab HTTP URL",
			url:  "https://gitlab.com/group/project.git",
			want: "project",
		},
		{
			name: "Nested path HTTP",
			url:  "https://github.com/org/suborg/project.git",
			want: "project",
		},
		{
			name: "Simple name",
			url:  "myproject",
			want: "myproject",
		},
		{
			name: "Trailing slash HTTP",
			url:  "https://github.com/user/repo/",
			want: "",
		},
		{
			name: "Empty string",
			url:  "",
			want: "unnamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRepoName(tt.url)
			if got != tt.want {
				t.Errorf("extractRepoName(%s) = %s, want %s", tt.url, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{
			name:  "string shorter than limit",
			input: "hello",
			n:     10,
			want:  "hello",
		},
		{
			name:  "string equal to limit",
			input: "hello",
			n:     5,
			want:  "hello",
		},
		{
			name:  "string longer than limit",
			input: "hello world",
			n:     8,
			want:  "hello ..",
		},
		{
			name:  "empty string",
			input: "",
			n:     5,
			want:  "",
		},
		{
			name:  "truncate to very short",
			input: "abcdefghij",
			n:     4,
			want:  "ab..",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%s, %d) = %s, want %s", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestRepeatStr(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{
			name: "repeat dash 3 times",
			s:    "-",
			n:    3,
			want: "---",
		},
		{
			name: "repeat equals 5 times",
			s:    "=",
			n:    5,
			want: "=====",
		},
		{
			name: "repeat word",
			s:    "ab",
			n:    2,
			want: "abab",
		},
		{
			name: "repeat zero times",
			s:    "-",
			n:    0,
			want: "",
		},
		{
			name: "repeat empty string",
			s:    "",
			n:    5,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repeatStr(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("repeatStr(%s, %d) = %s, want %s", tt.s, tt.n, got, tt.want)
			}
		})
	}
}
