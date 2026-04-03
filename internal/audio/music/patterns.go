package music

func PatternForLoop(id LoopID) (Pattern, bool) {
	switch id {
	case LoopBoot:
		return bootPattern(), true
	case LoopAmbient:
		return ambientPattern(), true
	case LoopBattle:
		return battlePattern(), true
	case LoopVictory:
		return victoryPattern(), true
	case LoopDefeat:
		return defeatPattern(), true
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

// ambientPattern is used for navigation and management screens (node map, roster
// rotation, reward, starter select). Slow A-minor pulse with sparse triangle pads
// and a ghostly square arpeggio — tension without urgency.
func ambientPattern() Pattern {
	return Pattern{
		BPM:          85,
		BeatsPerLoop: 8,
		SampleRate:   44100,
		Tracks: []Track{
			// Sine bass — sustained Am foundation
			{
				Waveform: WaveSine,
				Gain:     0.30,
				Notes: []Note{
					{StartBeat: 0, Beats: 2, MIDI: 45, Velocity: 0.85}, // A2
					{StartBeat: 2, Beats: 1, MIDI: 48, Velocity: 0.80}, // C3
					{StartBeat: 3, Beats: 1, MIDI: 50, Velocity: 0.75}, // D3
					{StartBeat: 4, Beats: 2, MIDI: 45, Velocity: 0.85}, // A2
					{StartBeat: 6, Beats: 2, MIDI: 43, Velocity: 0.80}, // G2
				},
			},
			// Triangle pads — atmospheric, filtered highs
			{
				Waveform: WaveTriangle,
				Gain:     0.14,
				Notes: []Note{
					{StartBeat: 0, Beats: 1.5, MIDI: 69, Velocity: 0.50}, // A4
					{StartBeat: 2, Beats: 1.5, MIDI: 72, Velocity: 0.50}, // C5
					{StartBeat: 4, Beats: 1.5, MIDI: 67, Velocity: 0.48}, // G4
					{StartBeat: 6, Beats: 2.0, MIDI: 64, Velocity: 0.48}, // E4
				},
			},
			// Square arpeggio — sparse digital glitch accent
			{
				Waveform: WaveSquare,
				Gain:     0.09,
				Notes: []Note{
					{StartBeat: 0.5, Beats: 0.25, MIDI: 57, Velocity: 0.60}, // A3
					{StartBeat: 1.0, Beats: 0.25, MIDI: 60, Velocity: 0.60}, // C4
					{StartBeat: 1.5, Beats: 0.25, MIDI: 64, Velocity: 0.60}, // E4
					{StartBeat: 4.5, Beats: 0.25, MIDI: 55, Velocity: 0.58}, // G3
					{StartBeat: 5.0, Beats: 0.25, MIDI: 57, Velocity: 0.58}, // A3
					{StartBeat: 5.5, Beats: 0.25, MIDI: 60, Velocity: 0.58}, // C4
				},
			},
		},
	}
}

// battlePattern plays during combat and the inspect view. Fast, aggressive saw
// bass with a punching square lead — dark A-minor riff at high BPM.
func battlePattern() Pattern {
	return Pattern{
		BPM:          145,
		BeatsPerLoop: 8,
		SampleRate:   44100,
		Tracks: []Track{
			// Saw bass — driving, distorted low-end
			{
				Waveform: WaveSaw,
				Gain:     0.28,
				Notes: []Note{
					{StartBeat: 0.00, Beats: 0.50, MIDI: 33, Velocity: 0.95}, // A1
					{StartBeat: 0.50, Beats: 0.25, MIDI: 33, Velocity: 0.88},
					{StartBeat: 1.00, Beats: 0.50, MIDI: 36, Velocity: 0.95}, // C2
					{StartBeat: 1.50, Beats: 0.25, MIDI: 33, Velocity: 0.85},
					{StartBeat: 2.00, Beats: 0.50, MIDI: 35, Velocity: 0.92}, // B1
					{StartBeat: 2.50, Beats: 0.25, MIDI: 33, Velocity: 0.88},
					{StartBeat: 3.00, Beats: 0.50, MIDI: 36, Velocity: 0.90}, // C2
					{StartBeat: 3.50, Beats: 0.50, MIDI: 38, Velocity: 0.85}, // D2
					{StartBeat: 4.00, Beats: 0.50, MIDI: 33, Velocity: 0.95},
					{StartBeat: 4.50, Beats: 0.25, MIDI: 33, Velocity: 0.88},
					{StartBeat: 5.00, Beats: 0.50, MIDI: 36, Velocity: 0.95},
					{StartBeat: 5.50, Beats: 0.25, MIDI: 33, Velocity: 0.85},
					{StartBeat: 6.00, Beats: 0.50, MIDI: 40, Velocity: 0.92}, // E2
					{StartBeat: 6.50, Beats: 0.25, MIDI: 38, Velocity: 0.88},
					{StartBeat: 7.00, Beats: 0.50, MIDI: 36, Velocity: 0.90},
					{StartBeat: 7.50, Beats: 0.50, MIDI: 33, Velocity: 0.85},
				},
			},
			// Square lead — punchy Am melodic stabs
			{
				Waveform: WaveSquare,
				Gain:     0.18,
				Notes: []Note{
					{StartBeat: 0.00, Beats: 0.25, MIDI: 57, Velocity: 0.80}, // A3
					{StartBeat: 0.50, Beats: 0.25, MIDI: 60, Velocity: 0.80}, // C4
					{StartBeat: 1.00, Beats: 0.50, MIDI: 64, Velocity: 0.85}, // E4
					{StartBeat: 2.00, Beats: 0.25, MIDI: 62, Velocity: 0.78}, // D4
					{StartBeat: 2.50, Beats: 0.25, MIDI: 60, Velocity: 0.78}, // C4
					{StartBeat: 3.00, Beats: 1.00, MIDI: 57, Velocity: 0.85}, // A3
					{StartBeat: 4.00, Beats: 0.25, MIDI: 55, Velocity: 0.78}, // G3
					{StartBeat: 4.50, Beats: 0.25, MIDI: 57, Velocity: 0.80},
					{StartBeat: 5.00, Beats: 0.50, MIDI: 60, Velocity: 0.82},
					{StartBeat: 6.00, Beats: 0.25, MIDI: 64, Velocity: 0.80},
					{StartBeat: 6.50, Beats: 0.50, MIDI: 65, Velocity: 0.85}, // F4
					{StartBeat: 7.00, Beats: 1.00, MIDI: 64, Velocity: 0.80},
				},
			},
			// Triangle accent — high shimmer on off-beats
			{
				Waveform: WaveTriangle,
				Gain:     0.10,
				Notes: []Note{
					{StartBeat: 0.50, Beats: 0.25, MIDI: 76, Velocity: 0.55}, // E5
					{StartBeat: 1.50, Beats: 0.25, MIDI: 72, Velocity: 0.52},
					{StartBeat: 2.50, Beats: 0.25, MIDI: 69, Velocity: 0.55},
					{StartBeat: 3.50, Beats: 0.25, MIDI: 72, Velocity: 0.52},
					{StartBeat: 4.50, Beats: 0.25, MIDI: 76, Velocity: 0.55},
					{StartBeat: 5.50, Beats: 0.25, MIDI: 74, Velocity: 0.52},
					{StartBeat: 6.50, Beats: 0.25, MIDI: 72, Velocity: 0.55},
					{StartBeat: 7.50, Beats: 0.25, MIDI: 69, Velocity: 0.52},
				},
			},
		},
	}
}

// victoryPattern plays when the player wins a run. Bright C-major fanfare with
// rising sine melody and triangle arpeggio — triumphant but still digital.
func victoryPattern() Pattern {
	return Pattern{
		BPM:          120,
		BeatsPerLoop: 8,
		SampleRate:   44100,
		Tracks: []Track{
			// Sine melody — bold rising fanfare
			{
				Waveform: WaveSine,
				Gain:     0.32,
				Notes: []Note{
					{StartBeat: 0, Beats: 1.0, MIDI: 48, Velocity: 0.90}, // C3
					{StartBeat: 1, Beats: 0.5, MIDI: 55, Velocity: 0.85}, // G3
					{StartBeat: 1.5, Beats: 0.5, MIDI: 55, Velocity: 0.80},
					{StartBeat: 2, Beats: 1.0, MIDI: 60, Velocity: 0.90},   // C4
					{StartBeat: 3, Beats: 0.5, MIDI: 67, Velocity: 0.85},   // G4
					{StartBeat: 3.5, Beats: 0.5, MIDI: 64, Velocity: 0.80}, // E4
					{StartBeat: 4, Beats: 1.0, MIDI: 65, Velocity: 0.85},   // F4
					{StartBeat: 5, Beats: 1.0, MIDI: 67, Velocity: 0.90},   // G4
					{StartBeat: 6, Beats: 0.5, MIDI: 69, Velocity: 0.85},   // A4
					{StartBeat: 6.5, Beats: 0.5, MIDI: 67, Velocity: 0.80},
					{StartBeat: 7, Beats: 1.0, MIDI: 72, Velocity: 0.95}, // C5
				},
			},
			// Triangle arpeggio — sparkling digital shimmer
			{
				Waveform: WaveTriangle,
				Gain:     0.18,
				Notes: []Note{
					{StartBeat: 0.0, Beats: 0.5, MIDI: 60, Velocity: 0.68}, // C4
					{StartBeat: 0.5, Beats: 0.5, MIDI: 64, Velocity: 0.68}, // E4
					{StartBeat: 1.0, Beats: 0.5, MIDI: 67, Velocity: 0.68}, // G4
					{StartBeat: 1.5, Beats: 0.5, MIDI: 72, Velocity: 0.68}, // C5
					{StartBeat: 2.0, Beats: 0.5, MIDI: 60, Velocity: 0.68},
					{StartBeat: 2.5, Beats: 0.5, MIDI: 67, Velocity: 0.68},
					{StartBeat: 3.0, Beats: 0.5, MIDI: 72, Velocity: 0.70},
					{StartBeat: 3.5, Beats: 0.5, MIDI: 76, Velocity: 0.70}, // E5
					{StartBeat: 4.0, Beats: 0.5, MIDI: 65, Velocity: 0.68}, // F4
					{StartBeat: 4.5, Beats: 0.5, MIDI: 69, Velocity: 0.68}, // A4
					{StartBeat: 5.0, Beats: 0.5, MIDI: 72, Velocity: 0.68},
					{StartBeat: 5.5, Beats: 0.5, MIDI: 77, Velocity: 0.70}, // F5
					{StartBeat: 6.0, Beats: 0.5, MIDI: 69, Velocity: 0.68},
					{StartBeat: 6.5, Beats: 0.5, MIDI: 72, Velocity: 0.68},
					{StartBeat: 7.0, Beats: 1.0, MIDI: 76, Velocity: 0.72},
				},
			},
			// Square stabs — punchy rhythmic chords on the beat
			{
				Waveform: WaveSquare,
				Gain:     0.12,
				Notes: []Note{
					{StartBeat: 0, Beats: 0.25, MIDI: 48, Velocity: 0.75},
					{StartBeat: 2, Beats: 0.25, MIDI: 53, Velocity: 0.72}, // F3
					{StartBeat: 4, Beats: 0.25, MIDI: 53, Velocity: 0.72},
					{StartBeat: 6, Beats: 0.25, MIDI: 55, Velocity: 0.72}, // G3
				},
			},
		},
	}
}

// defeatPattern plays when the player loses a run. Slow, descending A-minor
// dirge — somber sine with a droning saw undertone and quiet triangle elegy.
func defeatPattern() Pattern {
	return Pattern{
		BPM:          68,
		BeatsPerLoop: 4,
		SampleRate:   44100,
		Tracks: []Track{
			// Sine melody — slow descending minor
			{
				Waveform: WaveSine,
				Gain:     0.28,
				Notes: []Note{
					{StartBeat: 0.0, Beats: 1.5, MIDI: 45, Velocity: 0.80}, // A2
					{StartBeat: 1.5, Beats: 1.5, MIDI: 43, Velocity: 0.75}, // G2
					{StartBeat: 3.0, Beats: 1.0, MIDI: 40, Velocity: 0.78}, // E2
				},
			},
			// Triangle elegy — high, mournful countermelody
			{
				Waveform: WaveTriangle,
				Gain:     0.14,
				Notes: []Note{
					{StartBeat: 0, Beats: 1.0, MIDI: 69, Velocity: 0.45}, // A4
					{StartBeat: 1, Beats: 1.0, MIDI: 67, Velocity: 0.42}, // G4
					{StartBeat: 2, Beats: 1.0, MIDI: 64, Velocity: 0.45}, // E4
					{StartBeat: 3, Beats: 1.0, MIDI: 62, Velocity: 0.40}, // D4
				},
			},
			// Saw drone — dissonant, system-down undertone
			{
				Waveform: WaveSaw,
				Gain:     0.07,
				Notes: []Note{
					{StartBeat: 0, Beats: 4.0, MIDI: 31, Velocity: 0.30}, // G1
				},
			},
		},
	}
}
