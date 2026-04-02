package sfx

import (
	"fmt"
	"math"
	"math/rand"
)

func GeneratePreset(preset Preset, seed int64) ([]int16, int, error) {
	params, err := presetParams(preset, seed)
	if err != nil {
		return nil, 0, err
	}
	samples, err := Generate(params)
	if err != nil {
		return nil, 0, err
	}
	return samples, params.SampleRate, nil
}

func GenerateMany(preset Preset, baseSeed int64, count int) ([][]int16, int, error) {
	if count <= 0 {
		return nil, 0, fmt.Errorf("count must be > 0")
	}
	out := make([][]int16, 0, count)
	sampleRate := 0
	for i := range count {
		samples, rate, err := GeneratePreset(preset, baseSeed+int64(i))
		if err != nil {
			return nil, 0, err
		}
		if sampleRate == 0 {
			sampleRate = rate
		}
		out = append(out, samples)
	}
	return out, sampleRate, nil
}

func Generate(params Params) ([]int16, error) {
	params = clampParams(params)
	totalSamples := int(math.Ceil(params.Duration * float64(params.SampleRate)))
	if totalSamples <= 0 {
		return nil, fmt.Errorf("duration too short")
	}

	rng := rand.New(rand.NewSource(params.Seed))
	phaserBuffer := make([]float64, 2048)
	phaserPos := 0
	phase := 0.0
	freq := params.BaseFreq
	freqDelta := params.FreqRamp
	duty := params.DutyCycle
	lpf := 0.0
	lpfLast := 0.0
	hpf := 0.0
	holdCounter := 0
	holdValue := 0.0
	phaserOffset := params.PhaserOffset

	floatSamples := make([]float64, totalSamples)
	for i := 0; i < totalSamples; i++ {
		t := float64(i) / float64(params.SampleRate)
		freq += freqDelta
		freqDelta += params.FreqDeltaRamp
		if freq < 20 {
			freq = 20
		}
		if freq > 18000 {
			freq = 18000
		}
		duty = clamp(duty+params.DutyRamp, 0.02, 0.98)

		vib := 1.0
		if params.VibratoDepth > 0 && params.VibratoSpeed > 0 {
			vib += math.Sin(2*math.Pi*params.VibratoSpeed*t) * params.VibratoDepth
		}
		phase += (freq * vib) / float64(params.SampleRate)
		phase -= math.Floor(phase)

		raw := osc(params.WaveType, phase, duty, rng)

		if params.BitCrush > 1 {
			if holdCounter == 0 {
				holdValue = raw
				holdCounter = params.BitCrush
			}
			raw = holdValue
			holdCounter--
		}

		lpfCut := clamp(params.LPFCutoff+params.LPFRamp*t, 0.001, 1)
		lpf += (raw - lpf) * lpfCut
		lpfLast += (lpf - lpfLast) * 0.5
		s := lpfLast

		hpfCut := clamp(params.HPFCutoff+params.HPFRamp*t, 0, 0.995)
		hpf += s - lpfLast
		s -= hpf * hpfCut

		phaserOffset += params.PhaserRamp
		idx := (phaserPos - int(phaserOffset) + len(phaserBuffer)) % len(phaserBuffer)
		s += phaserBuffer[idx] * 0.4
		phaserBuffer[phaserPos] = s
		phaserPos = (phaserPos + 1) % len(phaserBuffer)

		s *= envelope(params, t)
		floatSamples[i] = s * params.Volume
	}

	normalize(floatSamples)
	pcm := make([]int16, len(floatSamples))
	for i, sample := range floatSamples {
		clamped := clamp(sample, -1, 1)
		pcm[i] = int16(clamped * 32767)
	}
	return pcm, nil
}

func ToFloat32PCM(samples []int16) []float32 {
	out := make([]float32, len(samples))
	for i, sample := range samples {
		out[i] = float32(sample) / 32767
	}
	return out
}

func osc(w WaveType, phase, duty float64, rng *rand.Rand) float64 {
	switch w {
	case WaveSquare:
		if phase < duty {
			return 1
		}
		return -1
	case WaveSaw:
		return phase*2 - 1
	case WaveSine:
		return math.Sin(2 * math.Pi * phase)
	case WaveNoise:
		return rng.Float64()*2 - 1
	default:
		return 0
	}
}

func envelope(params Params, t float64) float64 {
	if t < params.AttackTime {
		if params.AttackTime <= 0 {
			return 1
		}
		return t / params.AttackTime
	}
	t -= params.AttackTime
	if t < params.SustainTime {
		if params.SustainTime <= 0 {
			return 1
		}
		punch := params.SustainPunch * (1 - t/params.SustainTime)
		return 1 + punch
	}
	t -= params.SustainTime
	if t < params.DecayTime {
		if params.DecayTime <= 0 {
			return 0
		}
		return 1 - (t / params.DecayTime)
	}
	return 0
}

func normalize(samples []float64) {
	peak := 0.0
	for _, s := range samples {
		a := math.Abs(s)
		if a > peak {
			peak = a
		}
	}
	if peak <= 1e-6 {
		return
	}
	if peak <= 1 {
		return
	}
	scale := 1 / peak
	for i := range samples {
		samples[i] *= scale
	}
}

func clampParams(p Params) Params {
	if p.SampleRate <= 0 {
		p.SampleRate = 44100
	}
	if p.Duration <= 0 {
		p.Duration = p.AttackTime + p.SustainTime + p.DecayTime
	}
	p.Duration = clamp(p.Duration, 0.02, 3)
	p.BaseFreq = clamp(p.BaseFreq, 20, 18000)
	p.DutyCycle = clamp(p.DutyCycle, 0.02, 0.98)
	p.AttackTime = clamp(p.AttackTime, 0, 2)
	p.SustainTime = clamp(p.SustainTime, 0.01, 2)
	p.DecayTime = clamp(p.DecayTime, 0.01, 2)
	p.SustainPunch = clamp(p.SustainPunch, 0, 1)
	p.VibratoDepth = clamp(p.VibratoDepth, 0, 1)
	p.VibratoSpeed = clamp(p.VibratoSpeed, 0, 64)
	p.LPFCutoff = clamp(p.LPFCutoff, 0.001, 1)
	p.HPFCutoff = clamp(p.HPFCutoff, 0, 0.995)
	p.Volume = clamp(p.Volume, 0, 1)
	p.PhaserOffset = clamp(p.PhaserOffset, -1024, 1024)
	p.PhaserRamp = clamp(p.PhaserRamp, -5, 5)
	if p.BitCrush < 1 {
		p.BitCrush = 1
	}
	return p
}

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
