package music

import "testing"

func TestRenderLoopDeterministic(t *testing.T) {
	pattern, ok := PatternForLoop(LoopBoot)
	if !ok {
		t.Fatalf("missing loop pattern")
	}
	a, err := RenderLoop(pattern)
	if err != nil {
		t.Fatalf("render A: %v", err)
	}
	b, err := RenderLoop(pattern)
	if err != nil {
		t.Fatalf("render B: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("determinism mismatch at byte %d", i)
		}
	}
}

func TestRenderLoopExpectedDurationAndNonSilence(t *testing.T) {
	pattern, ok := PatternForLoop(LoopBoot)
	if !ok {
		t.Fatalf("missing loop pattern")
	}
	pcm, err := RenderLoop(pattern)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	expectedSamples := int(float64(pattern.BeatsPerLoop) * (60.0 / pattern.BPM) * float64(pattern.SampleRate))
	expectedBytes := expectedSamples * 2
	if len(pcm) != expectedBytes {
		t.Fatalf("unexpected length got=%d want=%d", len(pcm), expectedBytes)
	}

	nonZero := false
	for _, b := range pcm {
		if b != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected non-silent loop")
	}
}

func TestRenderLoopNoClippingExplosion(t *testing.T) {
	pattern, ok := PatternForLoop(LoopBoot)
	if !ok {
		t.Fatalf("missing loop pattern")
	}
	samples, err := RenderLoopPCM(pattern)
	if err != nil {
		t.Fatalf("render pcm: %v", err)
	}
	for i, s := range samples {
		if s < -1 || s > 1 {
			t.Fatalf("sample out of range at %d: %f", i, s)
		}
	}
}
