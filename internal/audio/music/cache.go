package music

import "fmt"

type Cache struct {
	loops map[string][]byte
}

func NewCache() *Cache {
	return &Cache{loops: make(map[string][]byte)}
}

func (c *Cache) Get(loop LoopID) ([]byte, error) {
	return c.GetVariant(loop, ReactiveState{})
}

func (c *Cache) GetVariant(loop LoopID, state ReactiveState) ([]byte, error) {
	key := string(loop) + "|" + state.cacheKey()
	if pcm, ok := c.loops[key]; ok {
		return pcm, nil
	}
	pattern, ok := patternForLoopState(loop, state)
	if !ok {
		return nil, fmt.Errorf("unknown music loop %q", loop)
	}
	pcm, err := RenderLoop(pattern)
	if err != nil {
		return nil, err
	}
	c.loops[key] = pcm
	return pcm, nil
}
