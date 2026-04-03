package music

func PatternForLoop(id LoopID) (Pattern, bool) {
	switch id {
	case LoopBoot:
		return bootPattern(), true
	default:
		return Pattern{}, false
	}
}

func bootPattern() Pattern {
	return Pattern{
		BPM:          110,
		BeatsPerLoop: 8,
		SampleRate:   44100,
		Tracks: []Track{
			{
				Waveform: WaveSine,
				Gain:     0.35,
				Notes: []Note{
					{StartBeat: 0, Beats: 1, MIDI: 38, Velocity: 0.9},
					{StartBeat: 1, Beats: 1, MIDI: 38, Velocity: 0.85},
					{StartBeat: 2, Beats: 1, MIDI: 41, Velocity: 0.9},
					{StartBeat: 3, Beats: 1, MIDI: 38, Velocity: 0.85},
					{StartBeat: 4, Beats: 1, MIDI: 43, Velocity: 0.95},
					{StartBeat: 5, Beats: 1, MIDI: 41, Velocity: 0.9},
					{StartBeat: 6, Beats: 1, MIDI: 38, Velocity: 0.85},
					{StartBeat: 7, Beats: 1, MIDI: 36, Velocity: 0.9},
				},
			},
			{
				Waveform: WaveSquare,
				Gain:     0.2,
				Notes: []Note{
					{StartBeat: 0, Beats: 0.5, MIDI: 62, Velocity: 0.7},
					{StartBeat: 0.5, Beats: 0.5, MIDI: 65, Velocity: 0.7},
					{StartBeat: 1.0, Beats: 0.5, MIDI: 69, Velocity: 0.72},
					{StartBeat: 1.5, Beats: 0.5, MIDI: 74, Velocity: 0.72},
					{StartBeat: 2.0, Beats: 0.5, MIDI: 62, Velocity: 0.7},
					{StartBeat: 2.5, Beats: 0.5, MIDI: 65, Velocity: 0.7},
					{StartBeat: 3.0, Beats: 0.5, MIDI: 67, Velocity: 0.72},
					{StartBeat: 3.5, Beats: 0.5, MIDI: 69, Velocity: 0.72},
					{StartBeat: 4.0, Beats: 0.5, MIDI: 65, Velocity: 0.7},
					{StartBeat: 4.5, Beats: 0.5, MIDI: 69, Velocity: 0.7},
					{StartBeat: 5.0, Beats: 0.5, MIDI: 72, Velocity: 0.74},
					{StartBeat: 5.5, Beats: 0.5, MIDI: 77, Velocity: 0.74},
					{StartBeat: 6.0, Beats: 0.5, MIDI: 64, Velocity: 0.7},
					{StartBeat: 6.5, Beats: 0.5, MIDI: 67, Velocity: 0.7},
					{StartBeat: 7.0, Beats: 0.5, MIDI: 69, Velocity: 0.74},
					{StartBeat: 7.5, Beats: 0.5, MIDI: 72, Velocity: 0.74},
				},
			},
		},
	}
}
