package music

import (
	"errors"
	"io"
	"sync"

	"github.com/ebitengine/oto/v3"

	"voidnet/internal/audio/otox"
)

var errAudioUnavailable = errors.New("audio backend unavailable")

type otoPlayer struct {
	ctx    *oto.Context
	ready  chan struct{}
	mu     sync.Mutex
	player *oto.Player
	reader *loopReader
}

type loopReader struct {
	mu  sync.Mutex
	pcm []byte
	pos int
}

func newOtoPlayer() (player, error) {
	ctx, ready, err := otox.SharedContext()
	if err != nil {
		return nil, errAudioUnavailable
	}
	return &otoPlayer{ctx: ctx, ready: ready}, nil
}

func (p *otoPlayer) StartLoopPCM(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	if err := p.waitReady(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reader == nil {
		p.reader = &loopReader{}
	}
	p.reader.SetPCM(pcm)
	if p.player == nil {
		p.player = p.ctx.NewPlayer(p.reader)
		if err := p.player.Err(); err != nil {
			_ = p.player.Close()
			p.player = nil
			return err
		}
	} else {
		p.player.Reset()
	}
	p.player.Play()
	return nil
}

func (p *otoPlayer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.player == nil {
		return nil
	}
	p.player.Pause()
	p.player.Reset()
	if p.reader != nil {
		p.reader.Rewind()
	}
	return p.player.Err()
}

func (p *otoPlayer) Close() error {
	if err := p.Stop(); err != nil {
		return err
	}
	if p.ctx != nil {
		return p.ctx.Err()
	}
	return nil
}

func (p *otoPlayer) waitReady() error {
	if p.ready != nil {
		<-p.ready
		p.ready = nil
	}
	if p.ctx == nil {
		return errAudioUnavailable
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (r *loopReader) Read(buf []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pcm) == 0 {
		return 0, io.EOF
	}
	for i := range buf {
		buf[i] = r.pcm[r.pos]
		r.pos++
		if r.pos >= len(r.pcm) {
			r.pos = 0
		}
	}
	return len(buf), nil
}

func (r *loopReader) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pcm) == 0 {
		r.pos = 0
		return 0, nil
	}
	base := 0
	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = r.pos
	case io.SeekEnd:
		base = len(r.pcm)
	default:
		return 0, errors.New("invalid whence")
	}
	pos := base + int(offset)
	pos %= len(r.pcm)
	if pos < 0 {
		pos += len(r.pcm)
	}
	r.pos = pos
	return int64(r.pos), nil
}

func (r *loopReader) SetPCM(pcm []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pcm = append(r.pcm[:0], pcm...)
	r.pos = 0
}

func (r *loopReader) Rewind() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pos = 0
}

func defaultPlayerFactory() (player, error) {
	return newOtoPlayer()
}
