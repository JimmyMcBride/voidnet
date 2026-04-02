package meta

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPathPrefersVoidnetAndFallsBackToLegacy(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}

	want := filepath.Join(configRoot, "voidnet", "meta.json")
	if path != want {
		t.Fatalf("DefaultPath() = %q, want %q", path, want)
	}

	legacy := filepath.Join(configRoot, "system-breakers", "meta.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(legacy, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	path, err = DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if path != legacy {
		t.Fatalf("DefaultPath() with legacy state = %q, want %q", path, legacy)
	}

	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(want, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	path, err = DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if path != want {
		t.Fatalf("DefaultPath() with new state = %q, want %q", path, want)
	}
}
