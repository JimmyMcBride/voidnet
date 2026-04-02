# Internal Runtime SFX System

Voidnet now includes an internal SFX runtime generator under `internal/audio/sfx`.

## Structure

- `internal/audio/events.go` — semantic event-level API for gameplay systems.
- `internal/audio/sfx/types.go` — waveform, preset, and synthesis params definitions.
- `internal/audio/sfx/presets.go` — Voidnet gameplay preset families with deterministic seeded variation.
- `internal/audio/sfx/generator.go` — PCM synthesis pipeline (waveform + envelope + pitch + modulation/effects).
- `internal/audio/sfx/wav.go` — minimal WAV writer for local debugging/auditioning.
- `cmd/sfxdemo/main.go` — local CLI to audition and export generated WAV files.

## Gameplay usage

Use semantic events from `internal/audio` (preferred):

```go
samples, sampleRate, err := audio.GenerateEventSFX(audio.EventHackSuccess, seed)
```

or call SFX presets directly:

```go
samples, sampleRate, err := sfx.GeneratePreset(sfx.PresetHackSuccess, seed)
```

Both return in-memory mono `[]int16` PCM for runtime playback integration.

## Determinism

A seed is part of generation. Identical inputs (preset + seed + params) produce identical PCM output.

Use this for:

- deterministic gameplay feedback
- reproducible debugging
- synchronized effects in future systems

## Auditioning in development

Generate one WAV:

```bash
go run ./cmd/sfxdemo -preset hack_success -seed 42 -out ./tmp/hack_success.wav
```

Generate multiple deterministic variations:

```bash
go run ./cmd/sfxdemo -preset daemon_captured -seed 100 -count 4 -out ./tmp/daemon_captured.wav
```

Optional sample-rate override:

```bash
go run ./cmd/sfxdemo -preset alert -sample-rate 22050 -out ./tmp/alert_22k.wav
```

## Adding a new preset

1. Add a constant in `internal/audio/sfx/types.go`.
2. Add it to `allPresets`.
3. Add tuning logic in `presetParams` in `internal/audio/sfx/presets.go`.
4. Optionally map it as a gameplay event in `internal/audio/events.go`.
5. Add/update tests to ensure generation remains deterministic and non-empty.

## Notes and tradeoffs

- This is tuned for short game SFX, not music or long ambience.
- Effects are intentionally lightweight (filters/phaser/bitcrush) to keep the code readable and fast.
- API is internal-first and can be refactored into a reusable package later if needed.
