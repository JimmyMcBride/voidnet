package sfx

import (
	"fmt"
	"math/rand"
)

func GenerateParamsForDebug(preset Preset, seed int64) (Params, error) {
	return presetParams(preset, seed)
}

func presetParams(preset Preset, seed int64) (Params, error) {
	if _, err := preset.Parse(); err != nil {
		return Params{}, err
	}
	rng := rand.New(rand.NewSource(seed))
	j := func(scale float64) float64 { return (rng.Float64()*2 - 1) * scale }

	base := Params{
		WaveType:     WaveSquare,
		SampleRate:   44100,
		Duration:     0.2,
		BaseFreq:     440,
		DutyCycle:    0.5,
		AttackTime:   0.005,
		SustainTime:  0.08,
		DecayTime:    0.08,
		SustainPunch: 0,
		LPFCutoff:    1.0,
		HPFCutoff:    0,
		BitCrush:     1,
		Volume:       0.8,
		Seed:         seed,
	}

	switch preset {
	case PresetCursorMove:
		base.WaveType = WaveSquare
		base.Duration = 0.06
		base.BaseFreq = 1100 + j(80)
		base.DutyCycle = 0.3 + j(0.05)
		base.DecayTime = 0.04
		base.Volume = 0.35
	case PresetSelect:
		base.WaveType = WaveSine
		base.Duration = 0.1
		base.BaseFreq = 860 + j(120)
		base.FreqRamp = 0.02
		base.SustainTime = 0.05
		base.DecayTime = 0.05
		base.SustainPunch = 0.2
		base.Volume = 0.45
	case PresetScan:
		base.WaveType = WaveSaw
		base.Duration = 0.35
		base.BaseFreq = 450 + j(40)
		base.FreqRamp = 0.01
		base.VibratoDepth = 0.08
		base.VibratoSpeed = 8 + rng.Float64()*5
		base.LPFCutoff = 0.5
		base.LPFRamp = 0.15
		base.HPFCutoff = 0.05
		base.BitCrush = 2
		base.Volume = 0.6
	case PresetHackStart:
		base.WaveType = WaveSaw
		base.Duration = 0.3
		base.BaseFreq = 320 + j(30)
		base.FreqRamp = 0.045
		base.FreqDeltaRamp = 0.00002
		base.SustainTime = 0.12
		base.DecayTime = 0.15
		base.LPFCutoff = 0.7
		base.Volume = 0.65
	case PresetHackSuccess:
		base.WaveType = WaveSine
		base.Duration = 0.28
		base.BaseFreq = 520 + j(60)
		base.FreqRamp = 0.06
		base.SustainPunch = 0.35
		base.VibratoDepth = 0.04
		base.VibratoSpeed = 10 + rng.Float64()*4
		base.Volume = 0.75
	case PresetHackFail:
		base.WaveType = WaveSquare
		base.Duration = 0.22
		base.BaseFreq = 420 + j(70)
		base.FreqRamp = -0.08
		base.DutyCycle = 0.15
		base.HPFCutoff = 0.08
		base.BitCrush = 3
		base.Volume = 0.7
	case PresetDaemonAppears:
		base.WaveType = WaveNoise
		base.Duration = 0.4
		base.BaseFreq = 260 + j(50)
		base.FreqRamp = 0.015
		base.VibratoDepth = 0.13
		base.VibratoSpeed = 6 + rng.Float64()*3
		base.LPFCutoff = 0.35
		base.HPFCutoff = 0.2
		base.PhaserOffset = 60 + j(20)
		base.PhaserRamp = 0.08
		base.BitCrush = 2
		base.Volume = 0.72
	case PresetDaemonCaptured:
		base.WaveType = WaveSquare
		base.Duration = 0.3
		base.BaseFreq = 350 + j(40)
		base.FreqRamp = 0.075
		base.FreqDeltaRamp = -0.00003
		base.SustainPunch = 0.4
		base.LPFCutoff = 0.8
		base.PhaserOffset = 40
		base.BitCrush = 2
		base.Volume = 0.82
	case PresetCorruptionBurst:
		base.WaveType = WaveNoise
		base.Duration = 0.28
		base.BaseFreq = 150 + j(25)
		base.FreqRamp = -0.03
		base.SustainTime = 0.09
		base.DecayTime = 0.18
		base.HPFCutoff = 0.18
		base.LPFCutoff = 0.4
		base.PhaserOffset = -90
		base.PhaserRamp = -0.04
		base.BitCrush = 5
		base.Volume = 0.9
	case PresetAlert:
		base.WaveType = WaveSquare
		base.Duration = 0.2
		base.BaseFreq = 980 + j(50)
		base.FreqRamp = -0.01
		base.DutyCycle = 0.2
		base.VibratoDepth = 0.05
		base.VibratoSpeed = 15
		base.HPFCutoff = 0.1
		base.Volume = 0.7
	case PresetLevelClear:
		base.WaveType = WaveSine
		base.Duration = 0.4
		base.BaseFreq = 420 + j(30)
		base.FreqRamp = 0.035
		base.FreqDeltaRamp = -0.00001
		base.SustainTime = 0.22
		base.DecayTime = 0.18
		base.SustainPunch = 0.25
		base.VibratoDepth = 0.02
		base.VibratoSpeed = 7
		base.Volume = 0.85
	case PresetSystemBoot:
		base.WaveType = WaveSaw
		base.Duration = 0.45
		base.BaseFreq = 180 + j(20)
		base.FreqRamp = 0.02
		base.FreqDeltaRamp = 0.00001
		base.AttackTime = 0.02
		base.SustainTime = 0.25
		base.DecayTime = 0.18
		base.LPFCutoff = 0.6
		base.LPFRamp = 0.1
		base.Volume = 0.78
	case PresetGlitchStinger:
		base.WaveType = WaveNoise
		base.Duration = 0.24
		base.BaseFreq = 720 + j(180)
		base.FreqRamp = -0.11
		base.FreqDeltaRamp = 0.00009
		base.VibratoDepth = 0.18
		base.VibratoSpeed = 25
		base.HPFCutoff = 0.25
		base.LPFCutoff = 0.55
		base.PhaserOffset = 120 + j(50)
		base.PhaserRamp = -0.12
		base.BitCrush = 6
		base.Volume = 0.9
	default:
		return Params{}, fmt.Errorf("unsupported preset %q", preset)
	}

	return clampParams(base), nil
}
