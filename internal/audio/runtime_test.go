package audio

import (
	"errors"
	"testing"

	"voidnet/internal/audio/music"
)

type fakeSFX struct {
	mutedCalls []bool
	playCalls  []Event
	closeErr   error
}

func (f *fakeSFX) SetMuted(v bool) { f.mutedCalls = append(f.mutedCalls, v) }
func (f *fakeSFX) Play(e Event) error {
	f.playCalls = append(f.playCalls, e)
	return nil
}
func (f *fakeSFX) Close() error { return f.closeErr }

type fakeMusic struct {
	mutedCalls []bool
	startCalls []music.LoopID
	stopCalls  int
	startErr   error
	closeErr   error
}

func (f *fakeMusic) StartLoop(loop music.LoopID) error {
	f.startCalls = append(f.startCalls, loop)
	return f.startErr
}
func (f *fakeMusic) StopLoop()       { f.stopCalls++ }
func (f *fakeMusic) SetMuted(v bool) { f.mutedCalls = append(f.mutedCalls, v) }
func (f *fakeMusic) Available() bool { return true }
func (f *fakeMusic) Close() error    { return f.closeErr }

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

func TestNewRuntimePropagatesMusicFactoryError(t *testing.T) {
	want := errors.New("music init failed")
	_, err := NewRuntime(Options{
		MusicFactory: func(opts music.Options) (*music.Runtime, error) {
			return nil, want
		},
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected music factory error, got %v", err)
	}
}

func TestCloseReturnsMusicAndSFXErrors(t *testing.T) {
	musicErr := errors.New("music close failed")
	sfxErr := errors.New("sfx close failed")
	r := &Runtime{
		sfx:   &fakeSFX{closeErr: sfxErr},
		music: &fakeMusic{closeErr: musicErr},
	}
	err := r.Close()
	if !errors.Is(err, musicErr) {
		t.Fatalf("expected joined music close error, got %v", err)
	}
	if !errors.Is(err, sfxErr) {
		t.Fatalf("expected joined sfx close error, got %v", err)
	}
}
