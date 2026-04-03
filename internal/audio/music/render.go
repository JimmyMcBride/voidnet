package music

import (
	"encoding/binary"
	"fmt"
	"math"
)

func RenderLoop(p Pattern) ([]byte, error) {
	samples, err := RenderLoopPCM(p)
	if err != nil {
		return nil, err
	}
	return EncodePCM16(samples), nil
}

func RenderLoopPCM(p Pattern) ([]float64, error) {
	if p.BPM <= 0 {
		return nil, fmt.Errorf("bpm must be > 0")
	}
	if p.BeatsPerLoop <= 0 {
		return nil, fmt.Errorf("beats per loop must be > 0")
	}
	if p.SampleRate <= 0 {
		return nil, fmt.Errorf("sample rate must be > 0")
	}
	secondsPerBeat := 60.0 / p.BPM
	durationSeconds := float64(p.BeatsPerLoop) * secondsPerBeat
	totalSamples := int(math.Round(durationSeconds * float64(p.SampleRate)))
	if totalSamples <= 0 {
		return nil, fmt.Errorf("pattern duration too short")
	}

	mix := make([]float64, totalSamples)
	for _, track := range p.Tracks {
		for _, note := range track.Notes {
			if note.Beats <= 0 {
				continue
			}
			start := int(math.Round(note.StartBeat * secondsPerBeat * float64(p.SampleRate)))
			if start >= totalSamples {
				continue
			}
			if start < 0 {
				start = 0
			}
			noteSamples := int(math.Round(note.Beats * secondsPerBeat * float64(p.SampleRate)))
			if noteSamples <= 0 {
				continue
			}
			end := start + noteSamples
			if end > totalSamples {
				end = totalSamples
			}

			freq := midiToHz(note.MIDI)
			velocity := clamp(note.Velocity, 0, 1)
			gain := clamp(track.Gain, 0, 1) * velocity
			for i := start; i < end; i++ {
				rel := i - start
				phase := 2 * math.Pi * freq * (float64(rel) / float64(p.SampleRate))
				amp := osc(track.Waveform, phase)
				env := noteEnvelope(rel, end-start)
				mix[i] += amp * gain * env
			}
		}
	}

	for i := range mix {
		mix[i] = clamp(mix[i], -1, 1)
	}
	return mix, nil
}

func EncodePCM16(samples []float64) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		v := int16(clamp(s, -1, 1) * 32767)
		binary.LittleEndian.PutUint16(out[i*2:], uint16(v))
	}
	return out
}

func midiToHz(midi int) float64 {
	return 440.0 * math.Pow(2.0, float64(midi-69)/12.0)
}

func noteEnvelope(i, total int) float64 {
	if total <= 1 {
		return 1
	}
	attack := int(float64(total) * 0.03)
	release := int(float64(total) * 0.1)
	if attack < 1 {
		attack = 1
	}
	if release < 1 {
		release = 1
	}
	if i < attack {
		return float64(i) / float64(attack)
	}
	releaseStart := total - release
	if i >= releaseStart {
		remaining := total - i
		if remaining < 0 {
			remaining = 0
		}
		return float64(remaining) / float64(release)
	}
	return 1
}

func osc(w Waveform, phase float64) float64 {
	switch w {
	case WaveSine:
		return math.Sin(phase)
	case WaveSquare:
		if math.Sin(phase) >= 0 {
			return 1
		}
		return -1
	case WaveSaw:
		n := phase / (2 * math.Pi)
		n -= math.Floor(n)
		return n*2 - 1
	case WaveTriangle:
		n := phase / (2 * math.Pi)
		n -= math.Floor(n)
		return 2*math.Abs(2*n-1) - 1
	default:
		return 0
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
