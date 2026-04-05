package music

import "testing"

func TestLoopReaderWrapsSeamlessly(t *testing.T) {
	r := &loopReader{pcm: []byte{1, 2, 3}}
	buf := make([]byte, 8)

	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if n != len(buf) {
		t.Fatalf("expected full buffer read, got %d", n)
	}

	want := []byte{1, 2, 3, 1, 2, 3, 1, 2}
	for i, b := range want {
		if buf[i] != b {
			t.Fatalf("unexpected byte at %d: got %d want %d", i, buf[i], b)
		}
	}
}
