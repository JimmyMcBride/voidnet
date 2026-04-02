package content

import "testing"

func TestLoadContent(t *testing.T) {
	reg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reg.Archetypes) != 4 {
		t.Fatalf("expected 4 archetypes, got %d", len(reg.Archetypes))
	}
	if _, ok := reg.Modifiers["single"]; !ok {
		t.Fatalf("expected single modifier to exist")
	}
}
