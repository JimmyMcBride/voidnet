package main

import (
	"testing"

	"voidnet/internal/audio/sfx"
)

func TestPresetParamsForDemoRejectsExcessiveSampleRate(t *testing.T) {
	_, err := presetParamsForDemo(sfx.PresetAlert, 1, sfx.MaxSampleRate+1)
	if err == nil {
		t.Fatalf("expected error for excessive sample rate override")
	}
}
