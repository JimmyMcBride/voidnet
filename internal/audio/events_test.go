package audio

import "testing"

func TestGenerateEventSFX(t *testing.T) {
	events := []Event{
		EventDaemonCaptured,
		EventPatchRestore,
		EventBackfire,
		EventCrash,
		EventUnlock,
		EventRunVictory,
		EventRunDefeat,
	}

	for i, event := range events {
		samples, sr, err := GenerateEventSFX(event, int64(55+i))
		if err != nil {
			t.Fatalf("generate event sfx for %s: %v", event, err)
		}
		if sr <= 0 || len(samples) == 0 {
			t.Fatalf("unexpected output for %s sr=%d samples=%d", event, sr, len(samples))
		}
	}
}
