package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"voidnet/internal/audio/sfx"
)

func main() {
	var (
		presetFlag     = flag.String("preset", string(sfx.PresetHackSuccess), "preset name")
		outFlag        = flag.String("out", "", "output .wav file (required when count=1)")
		seedFlag       = flag.Int64("seed", 1, "seed for deterministic generation")
		countFlag      = flag.Int("count", 1, "number of variations to generate")
		sampleRateFlag = flag.Int("sample-rate", 0, "override sample rate")
	)
	flag.Parse()

	preset, err := sfx.Preset(*presetFlag).Parse()
	if err != nil {
		exitErr(err)
	}
	if *countFlag <= 0 {
		exitErr(fmt.Errorf("count must be > 0"))
	}
	if *countFlag == 1 && *outFlag == "" {
		exitErr(fmt.Errorf("-out is required when count=1"))
	}

	for i := range *countFlag {
		seed := *seedFlag + int64(i)
		params, err := presetParamsForDemo(preset, seed, *sampleRateFlag)
		if err != nil {
			exitErr(err)
		}
		samples, err := sfx.Generate(params)
		if err != nil {
			exitErr(err)
		}
		outPath := chooseOutPath(*outFlag, preset, i, *countFlag)
		if outPath == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			exitErr(err)
		}
		if err := sfx.WriteWAV(outPath, params.SampleRate, samples); err != nil {
			exitErr(err)
		}
		fmt.Printf("wrote %s (preset=%s seed=%d samples=%d sr=%d)\n", outPath, preset, seed, len(samples), params.SampleRate)
	}
}

func presetParamsForDemo(preset sfx.Preset, seed int64, sampleRate int) (sfx.Params, error) {
	params, err := sfx.GenerateParamsForDebug(preset, seed)
	if err != nil {
		return sfx.Params{}, err
	}
	if sampleRate < 0 {
		return sfx.Params{}, fmt.Errorf("sample rate must be >= 0")
	}
	if sampleRate > sfx.MaxSampleRate {
		return sfx.Params{}, fmt.Errorf("sample rate %d exceeds max %d", sampleRate, sfx.MaxSampleRate)
	}
	if sampleRate > 0 {
		params.SampleRate = sampleRate
	}
	return params, nil
}

func chooseOutPath(out string, preset sfx.Preset, idx, count int) string {
	if count == 1 {
		return out
	}
	if out == "" {
		return filepath.Join("tmp", fmt.Sprintf("%s_%02d.wav", preset, idx+1))
	}
	ext := filepath.Ext(out)
	base := out[:len(out)-len(ext)]
	return fmt.Sprintf("%s_%02d%s", base, idx+1, ext)
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "sfxdemo:", err)
	os.Exit(1)
}
