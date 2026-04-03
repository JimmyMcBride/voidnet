package music

import (
	"errors"
	"testing"
)

type fakePlayer struct {
	startCalls int
	stopCalls  int
	closeCalls int
	lastPCM    []byte
	startErr   error
}

func (f *fakePlayer) StartLoopPCM(pcm []byte) error {
	f.startCalls++
	f.lastPCM = append([]byte(nil), pcm...)
	return f.startErr
}
func (f *fakePlayer) Stop() error {
	f.stopCalls++
	return nil
}
func (f *fakePlayer) Close() error {
	f.closeCalls++
	return nil
}

func TestRuntimeUnavailableWhenFactoryFails(t *testing.T) {
	r, err := NewRuntime(Options{Factory: func() (player, error) { return nil, errAudioUnavailable }})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if r.Available() {
		t.Fatalf("expected unavailable runtime")
	}
}

func TestRuntimePropagatesUnexpectedFactoryError(t *testing.T) {
	want := errors.New("backend init failed")
	_, err := NewRuntime(Options{Factory: func() (player, error) { return nil, want }})
	if !errors.Is(err, want) {
		t.Fatalf("expected factory error, got %v", err)
	}
}

func TestRuntimeUnavailableWhenFactoryReturnsNilPlayer(t *testing.T) {
	r, err := NewRuntime(Options{Factory: func() (player, error) { return nil, nil }})
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if r.Available() {
		t.Fatalf("expected unavailable runtime for nil player")
	}
}

func TestStopLoopIdempotent(t *testing.T) {
	p := &fakePlayer{}
	r, _ := NewRuntime(Options{Factory: func() (player, error) { return p, nil }})
	r.StopLoop()
	r.StopLoop()
	if p.stopCalls != 2 {
		t.Fatalf("expected idempotent stop calls, got %d", p.stopCalls)
	}
}

func TestSetMutedStateAndResume(t *testing.T) {
	p := &fakePlayer{}
	r, _ := NewRuntime(Options{Factory: func() (player, error) { return p, nil }})
	if err := r.StartLoop(LoopBoot); err != nil {
		t.Fatalf("start loop: %v", err)
	}
	r.SetMuted(true)
	r.SetMuted(false)
	if p.startCalls < 2 {
		t.Fatalf("expected restart on unmute, starts=%d", p.startCalls)
	}
	if p.stopCalls < 1 {
		t.Fatalf("expected stop on mute, stops=%d", p.stopCalls)
	}
}

func TestRestartLoopSwapsActiveLoopCleanly(t *testing.T) {
	p := &fakePlayer{}
	r, _ := NewRuntime(Options{Factory: func() (player, error) { return p, nil }})
	if err := r.StartLoop(LoopBoot); err != nil {
		t.Fatalf("start loop 1: %v", err)
	}
	if err := r.StartLoop(LoopBoot); err != nil {
		t.Fatalf("start loop 2: %v", err)
	}
	if p.stopCalls < 2 {
		t.Fatalf("expected stop before each restart, got %d", p.stopCalls)
	}
	if p.startCalls != 2 {
		t.Fatalf("expected two starts, got %d", p.startCalls)
	}
}

func TestSetMutedKeepsRuntimeMutedWhenResumeFails(t *testing.T) {
	p := &fakePlayer{}
	r, _ := NewRuntime(Options{Factory: func() (player, error) { return p, nil }})
	if err := r.StartLoop(LoopBoot); err != nil {
		t.Fatalf("start loop: %v", err)
	}

	r.SetMuted(true)
	p.startErr = errors.New("resume failed")
	r.SetMuted(false)
	if !r.muted {
		t.Fatalf("expected runtime to stay muted after failed resume")
	}
	if p.startCalls != 2 {
		t.Fatalf("expected one initial start and one failed resume attempt, got %d", p.startCalls)
	}

	p.startErr = nil
	r.SetMuted(false)
	if r.muted {
		t.Fatalf("expected runtime to unmute after successful retry")
	}
	if p.startCalls != 3 {
		t.Fatalf("expected retry to start playback, got %d starts", p.startCalls)
	}
}

func TestCloseMarksUnavailableWithoutPlayer(t *testing.T) {
	r := &Runtime{available: true, cache: NewCache()}
	if err := r.Close(); err != nil {
		t.Fatalf("close runtime: %v", err)
	}
	if r.Available() {
		t.Fatalf("expected close to mark runtime unavailable")
	}
}
