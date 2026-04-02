package audio

import "testing"

func TestGenerateEventSFX(t *testing.T) {
	samples, sr, err := GenerateEventSFX(EventDaemonCaptured, 55)
	if err != nil {
		t.Fatalf("generate event sfx: %v", err)
	}
	if sr <= 0 || len(samples) == 0 {
		t.Fatalf("unexpected output sr=%d samples=%d", sr, len(samples))
	}
}
