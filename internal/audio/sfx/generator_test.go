package sfx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePresetDeterministic(t *testing.T) {
	a, srA, err := GeneratePreset(PresetHackSuccess, 42)
	if err != nil {
		t.Fatalf("generate A: %v", err)
	}
	b, srB, err := GeneratePreset(PresetHackSuccess, 42)
	if err != nil {
		t.Fatalf("generate B: %v", err)
	}
	if srA != srB {
		t.Fatalf("sample rates differ: %d vs %d", srA, srB)
	}
	if len(a) != len(b) {
		t.Fatalf("sample lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("samples differ at %d", i)
		}
	}
}

func TestGeneratePresetNonEmpty(t *testing.T) {
	samples, sr, err := GeneratePreset(PresetCursorMove, 7)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if sr <= 0 {
		t.Fatalf("invalid sample rate: %d", sr)
	}
	if len(samples) == 0 {
		t.Fatalf("expected non-empty samples")
	}
}

func TestWriteWAV(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "test.wav")
	samples, sr, err := GeneratePreset(PresetAlert, 22)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := WriteWAV(out, sr, samples); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat wav: %v", err)
	}
	if st.Size() <= 44 {
		t.Fatalf("wav too small: %d", st.Size())
	}
}

func TestParamsClamp(t *testing.T) {
	p := clampParams(Params{
		SampleRate: 0,
		Duration:   -1,
		BaseFreq:   50000,
		DutyCycle:  -2,
		Volume:     3,
		BitCrush:   0,
	})
	if p.SampleRate != 44100 {
		t.Fatalf("expected default sample rate, got %d", p.SampleRate)
	}
	if p.BaseFreq > 18000 {
		t.Fatalf("base freq not clamped: %f", p.BaseFreq)
	}
	if p.DutyCycle < 0.02 {
		t.Fatalf("duty not clamped: %f", p.DutyCycle)
	}
	if p.Volume > 1 {
		t.Fatalf("volume not clamped: %f", p.Volume)
	}
	if p.BitCrush < 1 {
		t.Fatalf("bitcrush not clamped: %d", p.BitCrush)
	}
}

func TestAllPresetsGenerate(t *testing.T) {
	for i, preset := range AllPresets() {
		samples, _, err := GeneratePreset(preset, int64(100+i))
		if err != nil {
			t.Fatalf("preset %s failed: %v", preset, err)
		}
		if len(samples) == 0 {
			t.Fatalf("preset %s returned empty sample", preset)
		}
	}
}
