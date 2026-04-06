package buildinfo

import "testing"

func TestInfoStringVersionOnly(t *testing.T) {
	if got := (Info{Version: "v0.1.0"}).String(); got != "v0.1.0" {
		t.Fatalf("String() = %q, want %q", got, "v0.1.0")
	}
}

func TestInfoStringWithCommitAndDate(t *testing.T) {
	info := Info{
		Version: "v0.1.0",
		Commit:  "abcdef123456",
		Date:    "2026-04-06T16:00:00Z",
	}
	if got := info.String(); got != "v0.1.0 (abcdef123456, 2026-04-06T16:00:00Z)" {
		t.Fatalf("String() = %q", got)
	}
}

func TestCurrentDefaultsVersion(t *testing.T) {
	originalVersion := version
	originalCommit := commit
	originalDate := date
	t.Cleanup(func() {
		version = originalVersion
		commit = originalCommit
		date = originalDate
	})

	version = ""
	commit = "abc"
	date = ""

	info := Current()
	if info.Version != "dev" {
		t.Fatalf("Current().Version = %q, want %q", info.Version, "dev")
	}
}
