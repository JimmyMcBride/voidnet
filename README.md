# Voidnet

Voidnet is a terminal-native roguelike creature-collection RPG built in Go.

You play as an Operator infiltrating a corrupted network, capturing rogue AI daemons, fighting through branching nodes, and assembling weird little build synergies out of traits, modifiers, and merges.

## Install

### macOS / Linux

```sh
curl -fsSL https://voidnet.jimmymcbride.dev/install.sh | sh
```

### Windows PowerShell

```powershell
irm https://voidnet.jimmymcbride.dev/install.ps1 | iex
```

After install:

```sh
voidnet -version
voidnet
```

Notes:

- Linux builds currently need `libasound.so.2` available at runtime.
- The hosted installer pulls the latest tagged release from GitHub Releases.
- You can pin a Unix install to a specific version:

```sh
curl -fsSL https://voidnet.jimmymcbride.dev/install.sh | VERSION=v0.1.0 sh
```

## What Kind Of Game Is This?

Voidnet is a single-binary CLI game with a Go TUI. The current version is built around short, readable runs:

- Move through a small branching node map.
- Enter one encounter per node.
- Fight one enemy daemon at a time in turn-based combat.
- Capture enemies with `Isolate` instead of only defeating them.
- Spend a maintenance charge between fights to repair or fortify your roster.
- Rotate your lead daemon or merge two daemons from the node map.
- Reach the boss, win the run, or lose your last daemon and reset.

The core fantasy is not “type commands into a fake shell.” It is a menu-driven tactics game that happens to live inside a terminal.

## How A Run Works

The current run loop looks like this:

```text
Scan -> Choose Node -> Enter -> Encounter -> Battle -> Capture / Reward -> Maintenance / Node Map -> Boss
```

In practice:

- You start with a small daemon roster and one unlocked starter.
- Each node gives you a fight and a chance to improve your roster state.
- Winning a battle banks one maintenance charge, capped at one.
- The node map is where you manage your squad between encounters.
- Boss fights are tougher, and boss encounters cannot be isolated.

## Current Game Basics

### Archetypes

The current playable archetypes are:

- `Firewall`: defensive, high integrity
- `NullPointer`: offense, faster and more fragile
- `MemoryLeaker`: damage-over-time and status pressure
- `Scheduler`: speed and turn disruption

### Abilities

Abilities are built from:

- an `Effect`
- a `Modifier`

Current effects:

- `Corrupt`
- `Leak`
- `Spike`
- `Patch`
- `Delay`

Current modifiers:

- `Single`
- `Intensify`
- `Unstable`

### Traits

Current traits include:

- `Volatile`
- `Persistent`
- `Overclocked`
- `Encrypted`
- `Glitched`

Traits shape stat lines, resistance, or combat swings, and they are part of what makes captured daemons worth keeping around.

### Isolate

`Isolate` is the capture action.

- It does not work on bosses.
- It becomes available once the enemy is at 50% Health or lower.
- It gets much stronger at 25% Health or lower.
- Status effects can improve your odds.
- Target Stability pushes back against capture chance.

### Maintenance

The maintenance console lives between fights.

- `Repair Daemon` spends 1 charge to restore Health and clear one negative status.
- `Fortify Daemon` spends 1 charge to restore a smaller amount of Health and grant `Stabilized`.
- `Rotate Lead` is free and lets you pivot to another daemon before the next node.

### Merge

The current merge rule is `+1`.

- Pick a base daemon and a fork daemon.
- The fork is consumed.
- The base keeps its core identity and becomes `Name +1`.
- It gains the fork's archetype move as a third ability.
- It heals on merge and gains a small stat bonus based on the fork archetype.
- Same-archetype merges are not allowed, and `+1` daemons cannot merge again in the current version.

## Controls

Voidnet is menu-driven. You do not need to type free-form commands.

- `up/down` or `j/k`: move selection
- `enter`: confirm the current action
- `space` or `enter`: accelerate active combat/merge sequences
- `x`: open maintenance from the node map
- `r`: rotate lead from the node map
- `i`: open command detail where available
- `c`: open the command sheet
- `m`: toggle audio
- `f`: toggle playback fast-forward during combat

## Built For The Terminal

Some real things that are fun about how Voidnet is built:

- It is a Go TUI, not a wrapper around shell commands.
- The shipped game is a single binary.
- Core content is data-driven from [`internal/content/data/game.json`](./internal/content/data/game.json).
- Daemon generation and combat feedback lean on deterministic seeded behavior, which makes runs debuggable and repeatable.
- The game has procedural SFX and looping music instead of shipped audio files.
- Audio is semantic inside the game code: gameplay emits events like `Scan`, `Crash`, or `DaemonCaptured`, and the runtime decides how to play them.
- Music has reactive battle themes and tempo intensity, but it stays deterministic and cacheable.
- Progression is saved in the user config directory under `voidnet/meta.json`.

## Local Development

Run the game locally:

```sh
go run ./cmd/voidnet
```

Run tests:

```sh
go test ./...
```

Check the current build version string:

```sh
go run ./cmd/voidnet -version
```

Related internal docs:

- [`docs/internal/release-install.md`](./docs/internal/release-install.md) for release and installer hosting
- [`docs/internal/sfx-runtime.md`](./docs/internal/sfx-runtime.md) for the runtime audio system
- [`docs/planning/vision.md`](./docs/planning/vision.md) and [`docs/planning/prd.md`](./docs/planning/prd.md) for product direction
