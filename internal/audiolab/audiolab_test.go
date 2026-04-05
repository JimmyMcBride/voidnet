package audiolab

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"voidnet/internal/audio"
	"voidnet/internal/audio/music"
	"voidnet/internal/audio/sfx"
)

type fakeAudioRuntime struct {
	played    []audio.Event
	uiPlayed   []audio.Event
	seeds     []int64
	loops     []music.LoopID
	reactive  []music.ReactiveState
	stopCalls int
	muted     bool
	available bool
}

func (f *fakeAudioRuntime) Play(event audio.Event, seed int64) {
	f.played = append(f.played, event)
	f.seeds = append(f.seeds, seed)
}

func (f *fakeAudioRuntime) PlayUI(event audio.Event, seed int64) {
	f.uiPlayed = append(f.uiPlayed, event)
	f.seeds = append(f.seeds, seed)
}

func (f *fakeAudioRuntime) StartMusicLoop(loop music.LoopID) error {
	f.loops = append(f.loops, loop)
	return nil
}

func (f *fakeAudioRuntime) StopMusicLoop() {
	f.stopCalls++
}

func (f *fakeAudioRuntime) SetMusicReactiveState(state music.ReactiveState) error {
	f.reactive = append(f.reactive, state)
	return nil
}

func (f *fakeAudioRuntime) SetMuted(muted bool) {
	f.muted = muted
}

func (f *fakeAudioRuntime) Muted() bool {
	return f.muted
}

func (f *fakeAudioRuntime) Available() bool {
	return f.available
}

func (f *fakeAudioRuntime) Close() error {
	return nil
}

func TestBuildSFXCatalogIncludesAllPresets(t *testing.T) {
	got := buildSFXCatalog()
	want := sfx.AllPresets()
	if len(got) != len(want) {
		t.Fatalf("expected %d presets, got %d", len(want), len(got))
	}
	for i, preset := range want {
		if got[i].Preset != preset {
			t.Fatalf("unexpected preset at %d: got %s want %s", i, got[i].Preset, preset)
		}
		if got[i].Seed != 1 {
			t.Fatalf("expected default seed 1, got %d", got[i].Seed)
		}
	}
}

func TestBuildLoopCatalogUsesCuratedVariants(t *testing.T) {
	got := buildLoopCatalog()
	want := []string{
		"Boot",
		"Ambient",
		"Victory",
		"Defeat",
		"Battle / Standard / Tempo 0",
		"Battle / Standard / Tempo 1",
		"Battle / Standard / Tempo 2",
		"Battle / Corrupted / Tempo 0",
		"Battle / Corrupted / Tempo 1",
		"Battle / Corrupted / Tempo 2",
		"Battle / Boss / Tempo 0",
		"Battle / Boss / Tempo 1",
		"Battle / Boss / Tempo 2",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d loop items, got %d", len(want), len(got))
	}
	for i, label := range want {
		if got[i].Label != label {
			t.Fatalf("unexpected loop label at %d: got %q want %q", i, got[i].Label, label)
		}
	}
}

func TestSearchStateIsScopedPerTab(t *testing.T) {
	m := newModel(&fakeAudioRuntime{available: true})

	next, _ := m.Update(tea.KeyPressMsg{Text: "/"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "h"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "a"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "c"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "k"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(model)

	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "/"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "b"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "o"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(model)

	if m.tabs[tabSFX].query != "hack" {
		t.Fatalf("expected sfx query preserved, got %q", m.tabs[tabSFX].query)
	}
	if m.tabs[tabLoops].query != "boss" {
		t.Fatalf("expected loop query preserved, got %q", m.tabs[tabLoops].query)
	}

	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = next.(model)
	if m.tab != tabSFX {
		t.Fatalf("expected to return to sfx tab")
	}
	if m.currentTab().query != "hack" {
		t.Fatalf("expected active query to restore to hack, got %q", m.currentTab().query)
	}
}

func TestSFXSelectionIsManualAndSeedAdjustable(t *testing.T) {
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newModel(audioRuntime)

	next, _ := m.Update(tea.KeyPressMsg{Text: "j"})
	m = next.(model)
	if len(audioRuntime.played) != 0 {
		t.Fatalf("expected selection movement to stay silent")
	}

	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(model)

	if len(audioRuntime.played) != 1 {
		t.Fatalf("expected one sfx playback, got %d", len(audioRuntime.played))
	}
	if audioRuntime.seeds[0] != 2 {
		t.Fatalf("expected adjusted seed 2, got %d", audioRuntime.seeds[0])
	}
}

func TestLoopPlaybackAndStop(t *testing.T) {
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newModel(audioRuntime)

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(model)

	if len(audioRuntime.loops) != 1 || audioRuntime.loops[0] != music.LoopBoot {
		t.Fatalf("expected boot loop playback, got %+v", audioRuntime.loops)
	}
	if len(audioRuntime.reactive) != 1 {
		t.Fatalf("expected one reactive state call, got %d", len(audioRuntime.reactive))
	}

	next, _ = m.Update(tea.KeyPressMsg{Text: "s"})
	m = next.(model)
	if audioRuntime.stopCalls != 1 {
		t.Fatalf("expected one stop call, got %d", audioRuntime.stopCalls)
	}
}
