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
