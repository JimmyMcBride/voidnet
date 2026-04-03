package audio

import (
	"errors"
	"sync"
	"testing"
	"time"

	"voidnet/internal/audio/music"
)

type fakeMusic struct {
	mutedCalls []bool
	startCalls []music.LoopID
	stopCalls  int
	startErr   error
	closeErr   error
	available  bool
}

func (f *fakeMusic) StartLoop(loop music.LoopID) error {
	f.startCalls = append(f.startCalls, loop)
	return f.startErr
}

func (f *fakeMusic) StopLoop() {
	f.stopCalls++
}

func (f *fakeMusic) SetMuted(v bool) {
	f.mutedCalls = append(f.mutedCalls, v)
}

func (f *fakeMusic) Available() bool {
	return f.available
}

func (f *fakeMusic) Close() error {
	return f.closeErr
}

type testBackend struct {
	mu    sync.Mutex
	calls int
	ch    chan struct{}
}

func (b *testBackend) PlayPCM(samples []int16, sampleRate int) error {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	if b.ch != nil {
		select {
		case b.ch <- struct{}{}:
		default:
		}
	}
	return nil
}

func (b *testBackend) Available() bool { return true }
func (b *testBackend) Close() error    { return nil }

func (b *testBackend) Count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func TestCombinedRuntimeStartsConfiguredLoop(t *testing.T) {
	loop := music.LoopBoot
	mr := &fakeMusic{available: true}
	rt, err := NewRuntime(Options{
		SFX:       NewNoopRuntime(),
		AutoStart: &loop,
		MusicFactory: func(opts music.Options) (*music.Runtime, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	combined, ok := rt.(*combinedRuntime)
	if !ok {
		t.Fatalf("expected combined runtime")
	}
	combined.music = mr
	if err := combined.music.StartLoop(loop); err != nil {
		t.Fatalf("start loop: %v", err)
	}
	if len(mr.startCalls) != 1 || mr.startCalls[0] != music.LoopBoot {
		t.Fatalf("expected boot loop start call, got %+v", mr.startCalls)
	}
}

func TestCombinedRuntimePropagatesMusicFactoryError(t *testing.T) {
	want := errors.New("music init failed")
	_, err := NewRuntime(Options{
		SFX: NewNoopRuntime(),
		MusicFactory: func(opts music.Options) (*music.Runtime, error) {
			return nil, want
		},
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected music factory error, got %v", err)
	}
}

func TestCombinedRuntimeMutePropagatesToMusicAndSFX(t *testing.T) {
	backend := &testBackend{}
	sfxRuntime := newSFXRuntimeWithBackend(func(event Event, seed int64) ([]int16, int, error) {
		return []int16{1, 2, 3}, 44100, nil
	}, backend)
	mr := &fakeMusic{available: true}
	rt := &combinedRuntime{sfx: sfxRuntime, music: mr}

	rt.SetMuted(true)
	if !rt.Muted() {
		t.Fatalf("expected combined runtime to store mute state")
	}
	if !sfxRuntime.Muted() {
		t.Fatalf("expected sfx runtime to receive mute state")
	}
	if len(mr.mutedCalls) != 1 || !mr.mutedCalls[0] {
		t.Fatalf("expected music runtime to receive mute state, got %+v", mr.mutedCalls)
	}
}

func TestRuntimePlaysQueuedEvent(t *testing.T) {
	backend := &testBackend{ch: make(chan struct{}, 1)}
	rt := newSFXRuntimeWithBackend(func(event Event, seed int64) ([]int16, int, error) {
		return []int16{1, 2, 3}, 44100, nil
	}, backend)
	defer rt.Close()

	rt.Play(EventSelect, 1)

	select {
	case <-backend.ch:
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("expected queued event to be played")
	}
}

func TestRuntimeMuteSuppressesPlayback(t *testing.T) {
	backend := &testBackend{ch: make(chan struct{}, 1)}
	rt := newSFXRuntimeWithBackend(func(event Event, seed int64) ([]int16, int, error) {
		return []int16{1, 2, 3}, 44100, nil
	}, backend)
	defer rt.Close()

	rt.SetMuted(true)
	rt.Play(EventSelect, 1)

	select {
	case <-backend.ch:
		t.Fatalf("expected muted runtime to suppress playback")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestNoopRuntimeIsUnavailableAndSafe(t *testing.T) {
	rt := NewNoopRuntime()
	if rt.Available() {
		t.Fatalf("expected noop runtime to report unavailable")
	}
	rt.SetMuted(true)
	rt.Play(EventSelect, 1)
	if !rt.Muted() {
		t.Fatalf("expected noop runtime mute state to persist")
	}
}
