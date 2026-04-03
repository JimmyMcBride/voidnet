# Internal Runtime SFX + Music Audio System

Voidnet includes a deterministic SFX generator under `internal/audio/sfx`, a runtime playback layer under `internal/audio`, and a separate looping music subsystem under `internal/audio/music`.

## SFX runtime vs music runtime

- **SFX runtime** (`internal/audio/sfx`) is deterministic one-shot synthesis for UI/gameplay cues.
- **Music runtime** (`internal/audio/music`) is a long-lived loop player that renders a procedural pattern once and reuses cached PCM.
- The split ensures background music does not serialize or starve one-shot SFX activity.

## Structure

- `internal/audio/events.go` — semantic event-level API for gameplay systems.
- `internal/audio/runtime.go` — top-level audio façade that coordinates global mute and delegates separately to SFX + music runtimes.
- `internal/audio/runtime.go` — top-level audio façade that coordinates queued one-shot SFX playback, global mute, and the separate music runtime.
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

For live playback inside the game:

```go
bootLoop := music.LoopBoot
rt, err := audio.NewRuntime(audio.Options{AutoStart: &bootLoop})
if err != nil {
	panic(err)
}
defer rt.Close()

rt.Play(audio.EventHackSuccess, seed)
rt.SetMuted(true)
```

`NewRuntime()` silently falls back to a no-op runtime when the host audio backend cannot be initialized, so gameplay must not depend on sound being available.

## Runtime behavior

- One-shot SFX playback is queued and serialized for short terminal-native cues.
- Music loops run through a separate runtime so they do not contend with the SFX queue.
- Runtime audio is enabled by default when available.
- The TUI exposes a global `m` mute toggle and shows `audio=on`, `audio=muted`, or `audio=unavailable` in the footer.
- Gameplay systems should emit semantic events such as `EventScan`, `EventDaemonAppears`, or `EventCrash`; the UI/runtime decides when to actually play them.

## Current semantic events

- `SystemBoot` — app launch and new-run reset
- `Select` — generic menu confirmation
- `Scan` — node entry and inspect readouts
- `HackStart` — player attack windup
- `HackSuccess` — successful hit
- `HackFail` — failed hit or failed isolate
- `DaemonAppears` — encounter reveal
- `DaemonCaptured` — successful isolate / recruit
- `CorruptionBurst` — corruption or leaking damage
- `PatchRestore` — healing or stabilization recovery
- `Backfire` — modifier self-damage
- `Crash` — daemon death / collapse
- `Alert` — hostile opener, boss warning, active-daemon loss
- `LevelClear` — cleared node with no unlock
- `Unlock` — reward that unlocks new meta content
- `GlitchStinger` — delay / glitch disruption
- `RunVictory` — winning the run
- `RunDefeat` — losing the run

## Music loop v1 capabilities

- one built-in procedural loop (`music.LoopBoot`)
- deterministic offline render to PCM bytes
- background looping playback path owned by a dedicated music runtime
- global mute propagation from the top-level runtime to both SFX and music
- safe fallback when a host audio backend is unavailable

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
