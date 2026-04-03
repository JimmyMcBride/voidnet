package sfx

import "fmt"

type WaveType int

const (
	WaveSquare WaveType = iota
	WaveSaw
	WaveSine
	WaveNoise
)

func (w WaveType) String() string {
	switch w {
	case WaveSquare:
		return "square"
	case WaveSaw:
		return "saw"
	case WaveSine:
		return "sine"
	case WaveNoise:
		return "noise"
	default:
		return "unknown"
	}
}

type Preset string

const (
	DefaultSampleRate = 44100
	MaxSampleRate     = 192000

	PresetCursorMove      Preset = "cursor_move"
	PresetSelect          Preset = "select"
	PresetScan            Preset = "scan"
	PresetHackStart       Preset = "hack_start"
	PresetHackSuccess     Preset = "hack_success"
	PresetHackFail        Preset = "hack_fail"
	PresetDaemonAppears   Preset = "daemon_appears"
	PresetDaemonCaptured  Preset = "daemon_captured"
	PresetCorruptionBurst Preset = "corruption_burst"
	PresetPatchRestore    Preset = "patch_restore"
	PresetBackfire        Preset = "backfire"
	PresetCrash           Preset = "crash"
	PresetAlert           Preset = "alert"
	PresetLevelClear      Preset = "level_clear"
	PresetUnlock          Preset = "unlock"
	PresetSystemBoot      Preset = "system_boot"
	PresetGlitchStinger   Preset = "glitch_stinger"
	PresetRunVictory      Preset = "run_victory"
	PresetRunDefeat       Preset = "run_defeat"
)

var allPresets = []Preset{
	PresetCursorMove,
	PresetSelect,
	PresetScan,
	PresetHackStart,
	PresetHackSuccess,
	PresetHackFail,
	PresetDaemonAppears,
	PresetDaemonCaptured,
	PresetCorruptionBurst,
	PresetPatchRestore,
	PresetBackfire,
	PresetCrash,
	PresetAlert,
	PresetLevelClear,
	PresetUnlock,
	PresetSystemBoot,
	PresetGlitchStinger,
	PresetRunVictory,
	PresetRunDefeat,
}

func AllPresets() []Preset {
	out := make([]Preset, len(allPresets))
	copy(out, allPresets)
	return out
}

type Params struct {
	WaveType   WaveType
	SampleRate int
	Duration   float64

	BaseFreq      float64
	FreqRamp      float64
	FreqDeltaRamp float64

	DutyCycle float64
	DutyRamp  float64

	AttackTime   float64
	SustainTime  float64
	DecayTime    float64
	SustainPunch float64

	VibratoDepth float64
	VibratoSpeed float64

	LPFCutoff float64
	LPFRamp   float64
	HPFCutoff float64
	HPFRamp   float64

	PhaserOffset float64
	PhaserRamp   float64

	BitCrush int
	Volume   float64
	Seed     int64
}

func (p Preset) Parse() (Preset, error) {
	for _, candidate := range allPresets {
		if candidate == p {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unknown preset %q", p)
}
