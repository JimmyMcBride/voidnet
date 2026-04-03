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
→ Maintenance
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
Integrity: 28 | Status: Corrupted

Your Daemon: Firewall [Encrypted]
Integrity: 52

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
Archetype + Stats + 2 Abilities + 1 Trait
```

---

### Stats

* **Integrity** (HP)
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
* Isolate
* Inspect

---

### Status Effects

* Corrupted → lower success rate
* Leaking → damage over time
* Delayed → skip next turn
* Stabilized → resist effects

### Between-Fight Management

* After surviving a non-boss node, choose one maintenance action
* Repair Active
* Fortify Link
* Rotate Lead

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

* Must be ≤ 50% Integrity

---

### Base Chances

| Integrity | Chance |
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
