package adapters

import (
	"testing"

	"github.com/QTest-hq/qtest/internal/parser"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.adapters == nil {
		t.Error("adapters map should be initialized")
	}

	// Should have all default adapters registered
	list := r.List()
	if len(list) < 3 {
		t.Errorf("expected at least 3 adapters, got %d", len(list))
	}
}

func TestRegistry_Register(t *testing.T) {
	r := &Registry{adapters: make(map[Framework]Adapter)}

	adapter := NewGoAdapter()
	r.Register(adapter)

	if _, ok := r.adapters[FrameworkGoTest]; !ok {
		t.Error("adapter should be registered")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	t.Run("existing framework", func(t *testing.T) {
		adapter, err := r.Get(FrameworkGoTest)
		if err != nil {
			t.Errorf("Get() error: %v", err)
		}
		if adapter == nil {
			t.Error("adapter should not be nil")
		}
		if adapter.Framework() != FrameworkGoTest {
			t.Errorf("Framework() = %s, want go", adapter.Framework())
		}
	})

	t.Run("non-existing framework", func(t *testing.T) {
		_, err := r.Get("unknown")
		if err == nil {
			t.Error("Get() should return error for unknown framework")
		}
	})
}

func TestRegistry_GetForLanguage(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		name      string
		lang      parser.Language
		expected  Framework
		shouldErr bool
	}{
		{"go", parser.LanguageGo, FrameworkGoTest, false},
		{"javascript", parser.LanguageJavaScript, FrameworkJest, false},
		{"typescript", parser.LanguageTypeScript, FrameworkJest, false},
		{"python", parser.LanguagePython, FrameworkPytest, false},
		{"java", parser.LanguageJava, FrameworkJUnit, true}, // Not implemented
		{"unknown", parser.LanguageUnknown, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := r.GetForLanguage(tt.lang)

			if tt.shouldErr {
				if err == nil {
					t.Error("GetForLanguage() should return error")
				}
				return
			}

			if err != nil {
				t.Errorf("GetForLanguage() error: %v", err)
				return
			}

			if adapter.Framework() != tt.expected {
				t.Errorf("Framework() = %s, want %s", adapter.Framework(), tt.expected)
			}
		})
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	list := r.List()

	// Check that we have expected frameworks
	frameworks := make(map[Framework]bool)
	for _, f := range list {
		frameworks[f] = true
	}

	if !frameworks[FrameworkGoTest] {
		t.Error("List() should include go")
	}
	if !frameworks[FrameworkJest] {
		t.Error("List() should include jest")
	}
	if !frameworks[FrameworkPytest] {
		t.Error("List() should include pytest")
	}
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	r := NewRegistry()

	// Register a custom adapter with same framework
	custom := NewGoAdapter()
	r.Register(custom)

	// Should still work
	adapter, err := r.Get(FrameworkGoTest)
	if err != nil {
		t.Errorf("Get() error after overwrite: %v", err)
	}
	if adapter == nil {
		t.Error("adapter should not be nil after overwrite")
	}
}
