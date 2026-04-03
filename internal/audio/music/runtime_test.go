package music

import "testing"

type fakePlayer struct {
	startCalls int
	stopCalls  int
	closeCalls int
	lastPCM    []byte
}

func (f *fakePlayer) StartLoopPCM(pcm []byte) error {
	f.startCalls++
	f.lastPCM = append([]byte(nil), pcm...)
	return nil
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
