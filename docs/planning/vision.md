## Vision Document — *Voidnet*

---

## 1. Core Fantasy

A terminal-native RPG where the player acts as an **Operator** infiltrating a corrupted network:

* Capture rogue AI daemons
* Rewrite system behavior through combat and merging
* Shape the philosophy of the network (control vs freedom vs balance)

**Player Experience Goal:**

> “I built something uniquely mine—and pulled off insane combos I didn’t expect, but earned.”

---

## 2. Pillars (Non-Negotiables)

### 1. Controlled Chaos

* RNG exists, but within **tight, readable boundaries**
* Players influence outcomes, but never fully control them

### 2. Build Expression

* Daemons = modular systems
* Merging, traits, and abilities enable **custom builds**

### 3. Terminal Clarity

* Everything is readable via text
* No hidden complexity; depth emerges from interaction

### 4. Strategic Layering

* Micro: combat decisions
* Macro: node pathing + merging + philosophy choices

---

## 3. Core Gameplay Loop

```text
Scan → Choose Node → Enter → Encounter → Battle → Capture → Reward → Maintenance Console / Node Map → Progress
```

Expanded:

1. **Scan Network**

   * Reveal nearby nodes (partial visibility)
   * Some daemons hidden unless upgraded scanning

2. **Choose Path**

   * Branching node structure (Slay-the-Spire style)
   * Limited foresight → strategic risk

3. **Encounter**

   * Single-daemon duel (MVP)
   * Later: multi-daemon battles

4. **Battle**

   * Turn-based (Speed-driven)
   * Abilities = Effect + Modifier
   * Status-driven system

5. **Capture**

   * Replace or expand your roster

6. **Reward**

   * Firmware patches (merge modifiers)
   * Scan upgrades
   * Narrative logs

7. **Maintenance Console**

   * Winning a battle banks 1 maintenance charge (cap 1)
   * Spend it on repairs or fortification whenever you want before the next node
   * Choose the maintenance action first, then target any daemon in your roster
   * Swap between the node map and maintenance console freely

8. **Progress**

   * Reach boss → make a **philosophical decision**

---

## 4. Daemon System

### Structure

```text
Archetype + Stats + Abilities (2-3) + Trait (1) + Merge Level
```

### Stats

* **Health** → durability before a daemon crashes
* **Speed** → Turn order
* **Stability** → Status resistance

---

### Archetypes (MVP)

1. **NullPointer**

   * High damage, fragile
   * Theme: crashes, faults

2. **MemoryLeaker**

   * Damage over time
   * Theme: degradation

3. **Firewall**

   * Defensive/control
   * Theme: protection

4. **Scheduler**

   * Speed/turn manipulation
   * Theme: execution control

---

### Abilities System

Each ability:

```text
Effect + Modifier
```

#### Effects (MVP)

* Corrupt (status)
* Leak (DoT)
* Spike (burst)
* Patch (heal/cleanse)
* Delay (turn manipulation)

#### Modifiers (MVP)

* Single
* Intensify (stronger + risk)
* Unstable (chance to backfire)

---

### Traits (MVP Examples)

* Volatile → effect on death
* Persistent → resist status
* Overclocked → speed ↑ stability ↓
* Encrypted → reduce incoming effects
* Glitched → unpredictable bonuses/penalties

---

## 5. Procedural System (Controlled)

Generation Flow:

1. Pick Archetype
2. Assign stat ranges
3. Generate 2 abilities
4. Assign 1 trait

**Key Principle:**

> No chaos without boundaries.

---

## 6. Merge / Evolution System

### Current Merge Rule: `+1`

* Merging is available from the node map
* Choose a `base` daemon and a `fork` daemon
* The fork is consumed
* The base becomes `Name +1`
* The base keeps:
  * its archetype
  * its trait
  * its first two abilities
* The fork contributes:
  * its archetype move as a new slot 3 ability
  * a random modifier weighted toward the fork's current modifiers
  * a `+2` stat bonus in the fork archetype's merge-focus stat
* The merge also restores Health to the base daemon

### Constraints

* `+1` daemons cannot merge again in the current version
* Base and fork must be different archetypes
* Same-archetype merges are not allowed

### Why It Exists

* Gives the player a real macro build choice before adding deeper merge trees
* Adds a meaningful third ability slot without exploding complexity
* Reinforces “build something uniquely mine” through visible roster evolution

### Deferred / Future

* `+2` merges
* Trait inheritance
* Mutation outcomes
* Rare merge items / override tools

---

## 7. Combat Vision (MVP → Endgame)

### MVP

* 1v1 daemon battles
* Speed-based turn order
* Status interactions matter

---

### Endgame Vision

* Multi-daemon parties (2–3 active)
* Mid-battle swapping
* Combo chains:

Example:

* Delay → act twice → Spike + Unstable → Volatile death chain

---

### “Oh Snap” Moments Come From:

* Trait triggers
* Unstable modifiers succeeding
* Unexpected synergy chains

---

## 8. Node System (Meta Progression)

### Structure

* Branching node map (like Slay the Spire)
* Limited visibility:

  * See current + adjacent nodes only
  * Future hidden unless upgraded

---

### Node Types

* Combat Node
* Elite Node (harder, better rewards)
* Resource Node (items, patches)
* Log Node (story fragments)
* Boss Node (end of layer)

---

### Strategic Layer

* Risk vs reward decisions
* Path planning with incomplete information

---

## 9. Narrative Vision

### Premise

You are an Operator entering a corrupted network originally designed to:

> Protect free, decentralized information

Now:

* Controlled by a powerful entity
* Daemons corrupted into enforcers

---

### Core Conflict

* Freedom vs Control
* Order vs Chaos
* Idealism vs Reality

---

### The Operator (Story Device)

* A **previous Operator** (not you)
* Similar ideals
* Failed

---

### Narrative Delivery

* Log fragments (primary)
* Optional discoveries
* Daemon memory fragments (secondary)

---

### Early Logs

* Hopeful tone
* Belief in open systems
* Warning signs of corruption

---

### Mid Logs

* Increasing frustration
* Moral compromise
* Attempts to “fix” the system

---

### Late Logs

* Loss of control
* System becoming oppressive
* Regret

---

## 10. Philosophical Layer (Replayability)

Optional narrative fragments exploring:

* Privacy vs transparency
* AI autonomy vs control
* Decentralization vs safety
* Power vs responsibility

These do NOT alter main plot—but deepen engagement.

---

## 11. Decision System (Run-Defining)

After each boss:

You choose a **philosophical direction**

---

### Example Axes

#### Control

* More predictable systems
* Reduced randomness
* Stronger stability

#### Chaos

* Higher variance
* More powerful outcomes possible
* More risk

#### Balance

* Hybrid approach
* Adaptive systems

---

### Gameplay Impact

* Affects:

  * Trait behavior
  * Ability outcomes
  * Encounter difficulty
  * Merge probabilities

---

## 12. Endings

### Determined By:

* Accumulated decisions across the run

---

### 1. Control Ending

* Stable, locked-down network
* Safe but restrictive

---

### 2. Chaos Ending

* Open, unstable network
* Free but dangerous

---

### 3. Balance Ending

* Self-regulating system
* Adaptive equilibrium

---

### Final Choice

* Become the core (control the system)
* Or release it (let it evolve)

---

## 13. Long-Term Vision

### Expand Toward:

* Multi-daemon combat
* Deep merge trees
* Persistent network states
* Advanced node manipulation
* Player-defined builds at scale

---

## 14. MVP Discipline (Critical)

DO NOT ADD:

* Evolution trees (yet)
* Large ability pools
* Complex stat systems
* Party swapping

---

### MVP Focus

* 1 daemon vs 1 daemon
* Tight ability system
* Clean node progression
* First-layer story

---

## 15. Guiding Principle

> Build a small, tight system that *feels* deep—then expand.
