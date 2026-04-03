# Internal Runtime SFX + Music Audio System

Voidnet includes an internal SFX runtime generator under `internal/audio/sfx` and a separate looping music subsystem under `internal/audio/music`.

## SFX runtime vs music runtime

- **SFX runtime** (`internal/audio/sfx`) is deterministic one-shot synthesis for UI/gameplay cues.
- **Music runtime** (`internal/audio/music`) is a long-lived loop player that renders a procedural pattern once and reuses cached PCM.
- The split ensures background music does not serialize or starve one-shot SFX activity.

## Why music is not routed through the SFX queue

The one-shot SFX path is intentionally preserved for short, discrete sounds. Looping music is handled in its own runtime/player lifecycle so it can start/stop/mute independently and avoid queue contention with gameplay cues.

## Structure

- `internal/audio/events.go` — semantic event-level API for gameplay systems.
- `internal/audio/runtime.go` — top-level audio façade that coordinates global mute and delegates separately to SFX + music runtimes.
- `internal/audio/sfx/types.go` — waveform, preset, and synthesis params definitions.
- `internal/audio/sfx/presets.go` — Voidnet gameplay preset families with deterministic seeded variation.
- `internal/audio/sfx/generator.go` — PCM synthesis pipeline (waveform + envelope + pitch + modulation/effects).
- `internal/audio/sfx/wav.go` — minimal WAV writer for local debugging/auditioning.
- `internal/audio/music/types.go` — loop, track, and note definitions.
- `internal/audio/music/patterns.go` — built-in procedural music loop definitions.
- `internal/audio/music/render.go` — deterministic pattern-to-PCM renderer.
- `internal/audio/music/cache.go` — lazy loop cache to avoid repeat renders.
- `internal/audio/music/runtime.go` — loop playback runtime + mute/start/stop lifecycle.
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

## Music loop v1 capabilities

- one built-in procedural loop (`music.LoopBoot`)
- deterministic offline render to PCM bytes
- background looping playback path owned by a dedicated music runtime
- global mute propagation from top-level runtime to both SFX and music
- safe fallback when audio backend is unavailable (music methods become no-op safe)

## Determinism

A seed is part of SFX generation. Identical inputs (preset + seed + params) produce identical PCM output.

Music loops are deterministic for identical loop definitions and render parameters.

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

## Non-goals

The v1 music runtime is intentionally **not**:

- an adaptive soundtrack engine
- a sequencer
- a stem-switching system
- a composition/content authoring pipeline

## Known limits

- one loop active at a time
- simple note tracks/voices only
- no dynamic transitions or crossfades
- overlap details depend on backend player capabilities if separate players are used

## Notes and tradeoffs

- SFX synthesis remains tuned for short effects, not long ambience or composition.
- Music is rendered offline then looped, which keeps behavior deterministic and testable.
- API is internal-first and can evolve if a richer backend/mixer is introduced later.
