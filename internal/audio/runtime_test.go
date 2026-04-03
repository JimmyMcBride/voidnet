package audio

import (
	"testing"

	"voidnet/internal/audio/music"
)

type fakeSFX struct {
	mutedCalls []bool
	playCalls  []Event
}

func (f *fakeSFX) SetMuted(v bool) { f.mutedCalls = append(f.mutedCalls, v) }
func (f *fakeSFX) Play(e Event) error {
	f.playCalls = append(f.playCalls, e)
	return nil
}
func (f *fakeSFX) Close() error { return nil }

type fakeMusic struct {
	mutedCalls []bool
	startCalls []music.LoopID
	stopCalls  int
}

func (f *fakeMusic) StartLoop(loop music.LoopID) error {
	f.startCalls = append(f.startCalls, loop)
	return nil
}
func (f *fakeMusic) StopLoop()       { f.stopCalls++ }
func (f *fakeMusic) SetMuted(v bool) { f.mutedCalls = append(f.mutedCalls, v) }
func (f *fakeMusic) Available() bool { return true }
func (f *fakeMusic) Close() error    { return nil }

func TestStartMusicLoopDoesNotUseSFXQueue(t *testing.T) {
	sfx := &fakeSFX{}
	mr := &fakeMusic{}
	r, _ := NewRuntime(Options{
		SFX: sfx,
		MusicFactory: func(opts music.Options) (*music.Runtime, error) {
			return nil, nil
		},
	})
	r.music = mr
	if err := r.StartMusicLoop(music.LoopBoot); err != nil {
		t.Fatalf("start music: %v", err)
	}
	if len(sfx.playCalls) != 0 {
		t.Fatalf("expected no sfx queue usage when starting music")
	}
	if len(mr.startCalls) != 1 {
		t.Fatalf("expected music start call")
	}
}

func TestSFXStillPlaysWhileMusicActive(t *testing.T) {
	sfx := &fakeSFX{}
	mr := &fakeMusic{}
	r, _ := NewRuntime(Options{SFX: sfx})
	r.music = mr
	_ = r.StartMusicLoop(music.LoopBoot)
	if err := r.PlaySFX(EventSelect); err != nil {
		t.Fatalf("play sfx: %v", err)
	}
	if len(sfx.playCalls) != 1 || sfx.playCalls[0] != EventSelect {
		t.Fatalf("expected queued sfx play while music active")
	}
}

func TestGlobalMutePropagatesToSFXAndMusic(t *testing.T) {
	sfx := &fakeSFX{}
	mr := &fakeMusic{}
	r, _ := NewRuntime(Options{SFX: sfx})
	r.music = mr
	r.SetMuted(true)
	r.SetMuted(false)
	if len(sfx.mutedCalls) < 2 {
		t.Fatalf("expected mute calls to sfx")
	}
	if len(mr.mutedCalls) != 2 {
		t.Fatalf("expected mute calls to music")
	}
}
