# Voidnet — MVP PRD (Build-Ready)

---

## 1. Overview

**Game Type:** Terminal-native roguelike creature-collection RPG
**Platform:** CLI with Go TUI
**Language:** Go (single binary)

### Core Hook

> Capture rogue AI daemons and use them to hack, rewrite, and break a corrupted network from the inside.

---

## 2. Core Loop

```text
Start Run
→ Select Node
→ Scan
→ Encounter
→ Battle
→ Capture (optional)
→ Reward (if survive)
→ Maintenance Console / Node Map
→ Next Node
→ Boss
→ Win or Die → Reset
```

---

## 3. UI / Input System (Go TUI)

### Requirements

* No free-text commands
* Menu-driven selection only
* Arrow keys + enter
* Always-valid inputs (no user errors possible)
* Runtime SFX should reinforce key actions and be muteable with `m`

---

### Example Screen Flow

#### Node Selection

```text
Select Node:

[1] data.cache (Standard)
[2] auth.layer (Corrupted)
[3] ??? (Unknown)

> _
```

---

#### Combat

```text
Enemy: NullPointer [Corrupted]
Health: 28 | Status: Corrupted

Your Daemon: Firewall [Encrypted]
Health: 52

Choose Action:
[1] Patch + Single
[2] Delay + Single
[3] Isolate
[4] Inspect
```

---

## 4. World Structure

### Node Graph

* Small branching graph per run (4–5 nodes total, always 2 non-boss fights before the boss)

### Node Types

* Standard
* Corrupted
* Boss (final)

### Rules

* 1 encounter per node
* Hidden difficulty
* Node completes after battle

---

## 5. Daemon System

### Structure

```text
Archetype + Stats + 2-3 Abilities + 1 Trait + Merge Level
```

---

### Stats

* **Health** (HP)
* **Speed** (turn order)
* **Stability** (status resistance)

---

### Archetypes (MVP)

* NullPointer (offense)
* MemoryLeaker (DoT)
* Firewall (defense)
* Scheduler (speed)

---

### Abilities

Each = **Effect + Modifier**

#### Effects

* Corrupt
* Leak
* Spike
* Patch
* Delay

#### Modifiers

* Single
* Intensify
* Unstable

---

### Traits

* Volatile
* Persistent
* Overclocked
* Encrypted
* Glitched

---

### Generation Flow

```text
1. Select archetype
2. Assign stat ranges
3. Assign fixed core attack + archetype move
4. Assign 1 trait
```

---

## 6. Combat System

### Structure

* 1v1 daemon battle
* Turn-based
* Speed determines order each round

---

### Actions

* Ability 1
* Ability 2
* Ability 3 (merged daemons only)
* Isolate
* Inspect

---

### Status Effects

* Corrupted → lower success rate
* Leaking → damage over time
* Delayed → skip next turn
* Stabilized → resist effects

### Between-Fight Management

* Winning a battle grants 1 maintenance charge, capped at 1
* Maintenance can be opened from the node map at any time before the next node
* The maintenance screen shows the full roster state
* Repair Daemon and Fortify Daemon each spend 1 maintenance charge
* After choosing repair or fortify, the player selects which daemon in the roster receives the effect
* Rotate Lead is free roster management from the node map
* Merge Daemons is a visible node-map action
* Merge flow: choose base → choose fork → confirm
* `+1` merges are free, consume the fork, and give the base daemon a third skill slot
* Base and fork must be different archetypes, and neither daemon can already be `+1`
* Players can move between the node map and maintenance console freely before entering the next node

### Merge Outcome (`+1` MVP)

* The base daemon keeps its archetype, trait, and first two abilities
* The fork contributes its archetype move as slot 3
* Slot 3 gets a random modifier weighted toward modifiers already present on the fork
* The base daemon heals for 25% max Health, minimum 8
* The base daemon gains `+2` in the stat associated with the fork archetype:
  * Firewall fork → Max Health
  * MemoryLeaker fork → Stability
  * NullPointer fork → Speed
  * Scheduler fork → Speed
* `+2` merges are out of scope for the current version

---

### Ability Resolution

```text
Success Chance =
Base Chance
± Stability interaction
± Status effects
± Modifiers
```

---

### Win / Loss

* Win: enemy defeated or captured
* Loss: daemon dies

---

### Roguelike Rule

* Last daemon dies → run reset

---

### Node Risk Rule

* If daemon dies:

  * Lose capture
  * Lose stat gains from node

---

## 7. Capture System (Isolation)

### Eligibility

* Must be ≤ 50% Health

---

### Base Chances

| Health | Chance |
| --------- | ------ |
| 25–50%    | 40%    |
| ≤25%      | 75%    |

---

### Modifiers

* Corrupted: +10%
* Other status: +5% each
* Rarity resistance (future-ready)

---

### Final Formula

```text
Final Chance =
Base + Status Bonuses - Resistance
Clamp 5%–95%
```

---

### Failure

* No penalty
* No side effects

---

## 8. Progression

### In-Run

* Daemons gain small stat increases after victory
* Capture expands your options
* Risk: death removes node rewards

---

### Between Runs (Meta)

Unlock pool expands:

* Archetypes
* Traits
* Ability modifiers

---

### Starter System

* Unlockable starters
* Choose at run start

---

## 9. Boss System

### Boss Node

* Final node
* Stronger daemon
* Higher Stability
* Strong modifier

### Rules

* Not capturable
* Ends run on defeat

---

## 10. Narrative (Light MVP)

### Log System

* 1 short log per run or node

Example:

```text
[LOG_01]
"we built this system to be free…"
```

Purpose:

* Establish tone
* Seed future narrative system

---

## 11. System Hooks (Future-Proofing)

These exist as placeholders only:

* Reward slots (patches, upgrades)
* System state indicators
* Capture resistance scaling
* Expanded node types
* Merge system integration

---

## 12. Explicit Non-Goals (MVP)

* Multi-daemon party
* Switching
* Evolution / merging
* Items / inventory
* Shops / safe nodes
* Philosophy system
* Endings
* Save system
* Complex UI animations

---

## 13. Build Order (Concrete)

### Step 1 — Core Engine

* Daemon struct
* Ability system
* Status system

### Step 2 — Combat

* Turn loop
* Action handling
* Resolution system

### Step 3 — World

* Node graph generation
* Run loop

### Step 4 — Capture

* Isolation logic
* UI integration

### Step 5 — TUI

* Menus
* Navigation
* Clean output formatting

### Step 6 — Progression

* Stat growth
* Unlock pool

### Step 7 — Boss + Logs

* Boss logic
* Narrative output
