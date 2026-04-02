package sfx

import (
	"math"
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

func TestGenerateRejectsExcessiveSampleRate(t *testing.T) {
	_, err := Generate(Params{
		SampleRate:  MaxSampleRate + 1,
		Duration:    0.1,
		BaseFreq:    440,
		DutyCycle:   0.5,
		SustainTime: 0.05,
		DecayTime:   0.05,
		LPFCutoff:   1,
		Volume:      0.5,
	})
	if err == nil {
		t.Fatalf("expected error for excessive sample rate")
	}
}

func TestToFloat32PCMStaysWithinRange(t *testing.T) {
	out := ToFloat32PCM([]int16{math.MinInt16, 0, math.MaxInt16})
	if out[0] != -1 {
		t.Fatalf("expected min int16 to map to -1, got %f", out[0])
	}
	if out[1] != 0 {
		t.Fatalf("expected zero sample to remain zero, got %f", out[1])
	}
	if out[2] > 1 {
		t.Fatalf("expected max int16 to stay within range, got %f", out[2])
	}
}

func TestHighPassCutoffAffectsOutput(t *testing.T) {
	base := Params{
		WaveType:    WaveSaw,
		SampleRate:  DefaultSampleRate,
		Duration:    0.15,
		BaseFreq:    440,
		DutyCycle:   0.5,
		AttackTime:  0.005,
		SustainTime: 0.08,
		DecayTime:   0.05,
		LPFCutoff:   1,
		Volume:      0.7,
		Seed:        42,
	}

	a, err := Generate(base)
	if err != nil {
		t.Fatalf("generate baseline: %v", err)
	}
	base.HPFCutoff = 0.2
	b, err := Generate(base)
	if err != nil {
		t.Fatalf("generate with hpf: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("expected equal output lengths, got %d and %d", len(a), len(b))
	}
	same := true
	for i := range a {
		if a[i] != b[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatalf("expected high-pass cutoff to change generated output")
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
