package music

import "sync"

type player interface {
	StartLoopPCM(pcm []byte) error
	Stop() error
	Close() error
}

type playerFactory func() (player, error)

type Options struct {
	Muted   bool
	Factory playerFactory
}

type Runtime struct {
	mu        sync.Mutex
	available bool
	muted     bool
	cache     *Cache
	player    player
	active    LoopID
}

func NewRuntime(opts Options) (*Runtime, error) {
	r := &Runtime{muted: opts.Muted, cache: NewCache()}
	factory := opts.Factory
	if factory == nil {
		factory = defaultPlayerFactory
	}
	p, err := factory()
	if err != nil {
		return r, nil
	}
	r.player = p
	r.available = true
	return r, nil
}

func (r *Runtime) StartLoop(loop LoopID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.available || r.muted || r.player == nil {
		r.active = loop
		return nil
	}
	pcm, err := r.cache.Get(loop)
	if err != nil {
		return err
	}
	_ = r.player.Stop()
	if err := r.player.StartLoopPCM(pcm); err != nil {
		return err
	}
	r.active = loop
	return nil
}

func (r *Runtime) StopLoop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = ""
	if !r.available || r.player == nil {
		return
	}
	_ = r.player.Stop()
}

func (r *Runtime) SetMuted(muted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.muted == muted {
		return
	}
	r.muted = muted
	if !r.available || r.player == nil {
		return
	}
	if muted {
		_ = r.player.Stop()
		return
	}
	if r.active == "" {
		return
	}
	pcm, err := r.cache.Get(r.active)
	if err != nil {
		return
	}
	_ = r.player.StartLoopPCM(pcm)
}

func (r *Runtime) Available() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.available
}

func (r *Runtime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.player == nil {
		return nil
	}
	_ = r.player.Stop()
	err := r.player.Close()
	r.player = nil
	r.available = false
	return err
}

func defaultPlayerFactory() (player, error) {
	return nil, errAudioUnavailable
}
