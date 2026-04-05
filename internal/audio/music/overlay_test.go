package music

import "testing"

func TestPatternForLoopStateAddsBattleVariantDeterministically(t *testing.T) {
	state := ReactiveState{BattleTheme: BattleThemeBoss, Intensity: 2}
	withOverlay, ok := patternForLoopState(LoopBattle, state)
	if !ok {
		t.Fatalf("missing battle loop variant")
	}
	if withOverlay.BPM <= 132 {
		t.Fatalf("expected boss intensity to boost bpm, got %.2f", withOverlay.BPM)
	}

	a, err := RenderLoop(withOverlay)
	if err != nil {
		t.Fatalf("render overlay A: %v", err)
	}
	b, err := RenderLoop(withOverlay)
	if err != nil {
		t.Fatalf("render overlay B: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("deterministic render length mismatch")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("determinism mismatch at byte %d", i)
		}
	}
}

func TestPatternForLoopStateLeavesNonCombatLoopsUnchanged(t *testing.T) {
	base, ok := PatternForLoop(LoopAmbient)
	if !ok {
		t.Fatalf("missing ambient loop")
	}
	overlay, ok := patternForLoopState(LoopAmbient, ReactiveState{BattleTheme: BattleThemeCorrupted, Intensity: 2})
	if !ok {
		t.Fatalf("missing ambient loop variant")
	}
	if len(base.Tracks) != len(overlay.Tracks) {
		t.Fatalf("expected ambient loop to ignore combat overlay state")
	}
}

func TestBattleThemesAndIntensityProduceDistinctPatterns(t *testing.T) {
	standard0, ok := patternForLoopState(LoopBattle, ReactiveState{BattleTheme: BattleThemeStandard, Intensity: 0})
	if !ok {
		t.Fatalf("missing standard battle pattern")
	}
	standard2, ok := patternForLoopState(LoopBattle, ReactiveState{BattleTheme: BattleThemeStandard, Intensity: 2})
	if !ok {
		t.Fatalf("missing intense standard battle pattern")
	}
	corrupted0, ok := patternForLoopState(LoopBattle, ReactiveState{BattleTheme: BattleThemeCorrupted, Intensity: 0})
	if !ok {
		t.Fatalf("missing corrupted battle pattern")
	}
	boss0, ok := patternForLoopState(LoopBattle, ReactiveState{BattleTheme: BattleThemeBoss, Intensity: 0})
	if !ok {
		t.Fatalf("missing boss battle pattern")
	}

	if standard2.BPM <= standard0.BPM {
		t.Fatalf("expected intensity to increase bpm, got %.2f -> %.2f", standard0.BPM, standard2.BPM)
	}
	if corrupted0.Tracks[0].Notes[0].MIDI == standard0.Tracks[0].Notes[0].MIDI {
		t.Fatalf("expected corrupted theme bass line to differ from standard theme")
	}
	if len(boss0.Tracks) <= len(standard0.Tracks) {
		t.Fatalf("expected boss theme to add extra layers over standard theme")
	}
	if boss0.Tracks[0].Notes[0].MIDI >= standard0.Tracks[0].Notes[0].MIDI {
		t.Fatalf("expected boss theme to open on a lower bass note than standard")
	}
}
