package otox

import (
	"sync"

	"github.com/ebitengine/oto/v3"
)

const (
	SampleRate   = 44100
	ChannelCount = 1
	Format       = oto.FormatSignedInt16LE
)

var (
	sharedOnce  sync.Once
	sharedCtx   *oto.Context
	sharedReady chan struct{}
	sharedErr   error
)

// SharedContext returns the process-wide oto context. Oto v3 does not support
// creating multiple contexts, so music and SFX must share one.
func SharedContext() (*oto.Context, chan struct{}, error) {
	sharedOnce.Do(func() {
		sharedCtx, sharedReady, sharedErr = oto.NewContext(&oto.NewContextOptions{
			SampleRate:   SampleRate,
			ChannelCount: ChannelCount,
			Format:       Format,
		})
	})
	return sharedCtx, sharedReady, sharedErr
}
