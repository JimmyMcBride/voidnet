package music

import "fmt"

type Cache struct {
	loops map[LoopID][]byte
}

func NewCache() *Cache {
	return &Cache{loops: make(map[LoopID][]byte)}
}

func (c *Cache) Get(loop LoopID) ([]byte, error) {
	if pcm, ok := c.loops[loop]; ok {
		return pcm, nil
	}
	pattern, ok := PatternForLoop(loop)
	if !ok {
		return nil, fmt.Errorf("unknown music loop %q", loop)
	}
	pcm, err := RenderLoop(pattern)
	if err != nil {
		return nil, err
	}
	c.loops[loop] = pcm
	return pcm, nil
}
