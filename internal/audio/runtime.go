package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ebitengine/oto/v3"

	"voidnet/internal/audio/music"
	"voidnet/internal/audio/sfx"
)

type Runtime interface {
	Play(event Event, seed int64)
	SetMuted(muted bool)
	Muted() bool
	Available() bool
	Close() error
}

type Options struct {
	Muted        bool
	SFX          Runtime
	MusicFactory func(music.Options) (*music.Runtime, error)
	AutoStart    *music.LoopID
}

type combinedRuntime struct {
	sfx   Runtime
	music musicRuntime
	muted bool
}

type musicRuntime interface {
	StartLoop(music.LoopID) error
	StopLoop()
	SetMuted(bool)
	Available() bool
	Close() error
}

type GeneratorFunc func(event Event, seed int64) ([]int16, int, error)

type playbackRequest struct {
	event Event
	seed  int64
}

type audioBackend interface {
	PlayPCM(samples []int16, sampleRate int) error
	Available() bool
	Close() error
}

type sfxPlaybackRuntime struct {
	generate  GeneratorFunc
	backend   audioBackend
	queue     chan playbackRequest
	muted     atomic.Bool
	closed    atomic.Bool
	closeOnce sync.Once
	wg        sync.WaitGroup
}

type noopRuntime struct {
	muted     atomic.Bool
	available bool
}

type otoBackend struct {
	ctx   *oto.Context
	ready chan struct{}
}

func NewRuntime(opts Options) (Runtime, error) {
	sfxRuntime := opts.SFX
	if sfxRuntime == nil {
		sfxRuntime = newSFXRuntime()
	}
	sfxRuntime.SetMuted(opts.Muted)

	factory := opts.MusicFactory
	if factory == nil {
		factory = music.NewRuntime
	}
	musicRuntime, err := factory(music.Options{Muted: opts.Muted})
	if err != nil {
		return nil, err
	}
	if musicRuntime != nil && opts.AutoStart != nil {
		if err := musicRuntime.StartLoop(*opts.AutoStart); err != nil {
			return nil, errors.Join(err, musicRuntime.Close(), sfxRuntime.Close())
		}
	}

	return &combinedRuntime{
		sfx:   sfxRuntime,
		music: musicRuntime,
		muted: opts.Muted,
	}, nil
}

func NewNoopRuntime() Runtime {
	return &noopRuntime{}
}

func (r *combinedRuntime) Play(event Event, seed int64) {
	if r == nil || r.sfx == nil {
		return
	}
	r.sfx.Play(event, seed)
}

func (r *combinedRuntime) SetMuted(muted bool) {
	r.muted = muted
	if r.sfx != nil {
		r.sfx.SetMuted(muted)
	}
	if r.music != nil {
		r.music.SetMuted(muted)
	}
}

func (r *combinedRuntime) Muted() bool {
	if r == nil {
		return false
	}
	return r.muted
}

func (r *combinedRuntime) Available() bool {
	if r == nil {
		return false
	}
	if r.sfx != nil && r.sfx.Available() {
		return true
	}
	return r.music != nil && r.music.Available()
}

func (r *combinedRuntime) Close() error {
	if r == nil {
		return nil
	}
	var err error
	if r.music != nil {
		err = errors.Join(err, r.music.Close())
	}
	if r.sfx != nil {
		err = errors.Join(err, r.sfx.Close())
	}
	return err
}

func newSFXRuntime() Runtime {
	backend, err := newOtoBackend()
	if err != nil {
		return &noopRuntime{}
	}
	return newSFXRuntimeWithBackend(GenerateEventSFX, backend)
}

func newSFXRuntimeWithBackend(generate GeneratorFunc, backend audioBackend) Runtime {
	r := &sfxPlaybackRuntime{
		generate: generate,
		backend:  backend,
		queue:    make(chan playbackRequest, 32),
	}
	r.wg.Add(1)
	go r.loop()
	return r
}

func (r *sfxPlaybackRuntime) loop() {
	defer r.wg.Done()
	for req := range r.queue {
		if r.muted.Load() {
			continue
		}
		samples, sampleRate, err := r.generate(req.event, req.seed)
		if err != nil {
			continue
		}
		_ = r.backend.PlayPCM(samples, sampleRate)
	}
}

func (r *sfxPlaybackRuntime) Play(event Event, seed int64) {
	if event == "" || r.closed.Load() {
		return
	}
	select {
	case r.queue <- playbackRequest{event: event, seed: seed}:
	default:
	}
}

func (r *sfxPlaybackRuntime) SetMuted(muted bool) {
	r.muted.Store(muted)
}

func (r *sfxPlaybackRuntime) Muted() bool {
	return r.muted.Load()
}

func (r *sfxPlaybackRuntime) Available() bool {
	return r.backend != nil && r.backend.Available()
}

func (r *sfxPlaybackRuntime) Close() error {
	r.closeOnce.Do(func() {
		r.closed.Store(true)
		close(r.queue)
		r.wg.Wait()
	})
	if r.backend != nil {
		return r.backend.Close()
	}
	return nil
}

func (n *noopRuntime) Play(event Event, seed int64) {}

func (n *noopRuntime) SetMuted(muted bool) {
	n.muted.Store(muted)
}

func (n *noopRuntime) Muted() bool {
	return n.muted.Load()
}

func (n *noopRuntime) Available() bool {
	return n.available
}

func (n *noopRuntime) Close() error {
	return nil
}

func newOtoBackend() (audioBackend, error) {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sfx.DefaultSampleRate,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return nil, err
	}
	return &otoBackend{ctx: ctx, ready: ready}, nil
}

func (b *otoBackend) waitReady() error {
	if b.ready != nil {
		<-b.ready
		b.ready = nil
	}
	if err := b.ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (b *otoBackend) PlayPCM(samples []int16, sampleRate int) error {
	if sampleRate != sfx.DefaultSampleRate {
		return fmt.Errorf("unsupported sample rate %d", sampleRate)
	}
	if err := b.waitReady(); err != nil {
		return err
	}

	player := b.ctx.NewPlayer(bytes.NewReader(int16PCMBytes(samples)))
	player.Play()
	for player.IsPlaying() || player.BufferedSize() > 0 {
		if err := player.Err(); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	return player.Err()
}

func (b *otoBackend) Available() bool {
	return b != nil && b.ctx != nil
}

func (b *otoBackend) Close() error {
	return nil
}

func int16PCMBytes(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(sample))
	}
	return buf
}
