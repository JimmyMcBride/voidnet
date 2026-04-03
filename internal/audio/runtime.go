package audio

import (
	"errors"

	"voidnet/internal/audio/music"
)

type sfxRuntime interface {
	SetMuted(bool)
	Play(Event) error
	Close() error
}

type musicRuntime interface {
	StartLoop(music.LoopID) error
	StopLoop()
	SetMuted(bool)
	Available() bool
	Close() error
}

type Options struct {
	Muted        bool
	SFX          sfxRuntime
	MusicFactory func(music.Options) (*music.Runtime, error)
	AutoStart    *music.LoopID
}

type Runtime struct {
	sfx   sfxRuntime
	music musicRuntime
	muted bool
}

func NewRuntime(opts Options) (*Runtime, error) {
	r := &Runtime{sfx: opts.SFX, muted: opts.Muted}
	if r.sfx != nil {
		r.sfx.SetMuted(opts.Muted)
	}

	factory := opts.MusicFactory
	if factory == nil {
		factory = music.NewRuntime
	}
	mr, err := factory(music.Options{Muted: opts.Muted})
	if err != nil {
		return nil, err
	}
	if mr != nil {
		if opts.AutoStart != nil {
			if err := mr.StartLoop(*opts.AutoStart); err != nil {
				return nil, errors.Join(err, mr.Close())
			}
		}
		r.music = mr
	}
	return r, nil
}

func (r *Runtime) PlaySFX(event Event) error {
	if r.sfx == nil {
		return nil
	}
	return r.sfx.Play(event)
}

func (r *Runtime) StartMusicLoop(loop music.LoopID) error {
	if r.music == nil {
		return nil
	}
	return r.music.StartLoop(loop)
}

func (r *Runtime) StopMusicLoop() {
	if r.music != nil {
		r.music.StopLoop()
	}
}

func (r *Runtime) SetMuted(muted bool) {
	r.muted = muted
	if r.sfx != nil {
		r.sfx.SetMuted(muted)
	}
	if r.music != nil {
		r.music.SetMuted(muted)
	}
}

func (r *Runtime) Muted() bool {
	return r.muted
}

func (r *Runtime) MusicAvailable() bool {
	if r.music == nil {
		return false
	}
	return r.music.Available()
}

func (r *Runtime) Close() error {
	var err error
	if r.music != nil {
		err = errors.Join(err, r.music.Close())
	}
	if r.sfx != nil {
		err = errors.Join(err, r.sfx.Close())
	}
	return err
}
