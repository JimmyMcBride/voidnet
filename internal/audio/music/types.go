package music

type LoopID string

const (
	LoopBoot    LoopID = "boot"
	LoopAmbient LoopID = "ambient"
	LoopBattle  LoopID = "battle"
	LoopVictory LoopID = "victory"
	LoopDefeat  LoopID = "defeat"
)

type Waveform int

const (
	WaveSine Waveform = iota
	WaveSquare
	WaveSaw
	WaveTriangle
)

type Note struct {
	StartBeat float64
	Beats     float64
	MIDI      int
	Velocity  float64
}

type Track struct {
	Waveform Waveform
	Gain     float64
	Notes    []Note
}

type Pattern struct {
	BPM          float64
	BeatsPerLoop int
	SampleRate   int
	Tracks       []Track
}

type BattleTheme string

const (
	BattleThemeStandard  BattleTheme = "standard"
	BattleThemeCorrupted BattleTheme = "corrupted"
	BattleThemeBoss      BattleTheme = "boss"
)

// ReactiveState controls which battle theme is rendered and how aggressively
// its tempo is pushed. It remains intentionally constrained.
type ReactiveState struct {
	BattleTheme BattleTheme
	Intensity   int
}

func (s ReactiveState) normalized() ReactiveState {
	if s.Intensity < 0 {
		s.Intensity = 0
	}
	if s.Intensity > 2 {
		s.Intensity = 2
	}
	switch s.BattleTheme {
	case BattleThemeStandard, BattleThemeCorrupted, BattleThemeBoss:
	default:
		s.BattleTheme = ""
	}
	return s
}

func (s ReactiveState) cacheKey() string {
	s = s.normalized()
	return string(s.BattleTheme) + "|" + string(rune('0'+s.Intensity))
}
