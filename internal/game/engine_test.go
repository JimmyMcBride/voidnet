package game

import (
	"math"
	"strings"
	"testing"

	"voidnet/internal/content"
	"voidnet/internal/meta"
)

func TestDeterministicStarterAndGraph(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	stateA := meta.DefaultState()
	stateB := meta.DefaultState()

	engineA := New(reg, &stateA, 12345, true)
	engineB := New(reg, &stateB, 12345, true)

	if _, err := engineA.ChooseStarter("firewall"); err != nil {
		t.Fatalf("engineA starter failed: %v", err)
	}
	if _, err := engineB.ChooseStarter("firewall"); err != nil {
		t.Fatalf("engineB starter failed: %v", err)
	}

	if engineA.Run.NodeOrder[0] != engineB.Run.NodeOrder[0] || engineA.Run.Nodes["n1"].Label != engineB.Run.Nodes["n1"].Label {
		t.Fatalf("expected identical graph generation for same seed")
	}

	a := engineA.ActiveDaemon()
	b := engineB.ActiveDaemon()
	if a.Name != b.Name || a.Abilities[0] != b.Abilities[0] || a.TraitID != b.TraitID {
		t.Fatalf("expected identical starter generation for same seed")
	}
}

func TestGraphAlwaysProvidesTwoNonBossFightsBeforeBoss(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	for seed := int64(1); seed <= 40; seed++ {
		state := meta.DefaultState()
		engine := New(reg, &state, seed, true)

		if got := len(engine.Run.NodeOrder); got != 4 && got != 5 {
			t.Fatalf("seed %d: expected 4 or 5 nodes, got %d", seed, got)
		}

		boss := engine.Run.Nodes["boss"]
		if boss == nil || boss.Difficulty != 3 {
			t.Fatalf("seed %d: expected boss difficulty 3, got %+v", seed, boss)
		}

		for _, nodeID := range engine.currentChoicesForNodesFrom("start") {
			first := engine.Run.Nodes[nodeID]
			if first == nil || first.Difficulty != 1 {
				t.Fatalf("seed %d: expected first layer difficulty 1, got %+v", seed, first)
			}
			if len(first.Children) != 1 {
				t.Fatalf("seed %d: expected one child from %s, got %v", seed, nodeID, first.Children)
			}

			second := engine.Run.Nodes[first.Children[0]]
			if second == nil || second.ID == "boss" || second.Difficulty != 2 {
				t.Fatalf("seed %d: expected second layer difficulty 2 before boss, got %+v", seed, second)
			}
			if len(second.Children) != 1 || second.Children[0] != "boss" {
				t.Fatalf("seed %d: expected %s to lead directly to boss, got %v", seed, second.ID, second.Children)
			}
		}
	}
}

func TestCaptureChanceUsesRebalancedValues(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 55, true)
	engine.Run.Combat = &CombatState{
		Enemy: Daemon{
			MaxIntegrity: 100,
			Integrity:    50,
			Stability:    10,
			Statuses:     map[string]int{},
		},
	}

	if chance, eligible := engine.captureChance(); !eligible || chance != 49 {
		t.Fatalf("expected rebalanced mid-health capture chance, got eligible=%v chance=%d", eligible, chance)
	}

	engine.Run.Combat.Enemy.Integrity = 25
	if chance, eligible := engine.captureChance(); !eligible || chance != 79 {
		t.Fatalf("expected rebalanced low-health capture chance, got eligible=%v chance=%d", eligible, chance)
	}

	engine.Run.Combat.Enemy.Statuses["corrupted"] = 2
	if chance, eligible := engine.captureChance(); !eligible || chance != 89 {
		t.Fatalf("expected corrupted bonus to apply, got eligible=%v chance=%d", eligible, chance)
	}
}

func TestCaptureChanceDetailsIncludeStabilityBreakdown(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 55, true)
	engine.Run.Combat = &CombatState{
		Enemy: Daemon{
			MaxIntegrity: 100,
			Integrity:    25,
			Stability:    10,
			Statuses: map[string]int{
				"corrupted": 2,
				"delayed":   2,
			},
		},
	}

	chance, eligible, terms := engine.captureChanceDetails()
	if !eligible || chance != 94 {
		t.Fatalf("expected deterministic capture chance details, got eligible=%v chance=%d", eligible, chance)
	}

	line := formatChanceBreakdown("Isolation chance", chance, 54, terms)
	if !strings.Contains(line, "base 80") || !strings.Contains(line, "corrupted +10") || !strings.Contains(line, "negative statuses +5") || !strings.Contains(line, "target Stability -1") {
		t.Fatalf("expected capture breakdown terms in line, got %q", line)
	}
}

func TestResolveAbilityIncludesAccuracyBreakdownTerms(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 77, true)
	engine.Run.Roster = []Daemon{{
		ID:           "player",
		Name:         "Firewall",
		ArchetypeID:  "firewall",
		MaxIntegrity: 50,
		Integrity:    50,
		Speed:        8,
		Stability:    12,
		TraitID:      "persistent",
		Abilities:    []Ability{{EffectID: "spike", ModifierID: "intensify"}},
		Statuses:     map[string]int{"corrupted": 2},
	}}
	engine.Run.ActiveIndex = 0
	engine.Run.Combat = &CombatState{
		NodeID:   "n1",
		NodeType: NodeStandard,
		Enemy: Daemon{
			ID:           "enemy",
			Name:         "NullPointer",
			ArchetypeID:  "nullpointer",
			MaxIntegrity: 40,
			Integrity:    40,
			Speed:        10,
			Stability:    20,
			TraitID:      "encrypted",
			Abilities:    []Ability{{EffectID: "spike", ModifierID: "single"}},
			Statuses:     map[string]int{},
		},
		Queue: []Actor{ActorPlayer, ActorEnemy},
		Round: 1,
	}

	lines := engine.resolveAbility(ActorPlayer, Ability{EffectID: "spike", ModifierID: "intensify"})
	if len(lines) < 2 {
		t.Fatalf("expected resolveAbility to emit roll output, got %+v", lines)
	}
	rollLine := lines[1]
	if !strings.Contains(rollLine, "base 90") || !strings.Contains(rollLine, "modifier -10") || !strings.Contains(rollLine, "status -10") || !strings.Contains(rollLine, "stability diff -4") || !strings.Contains(rollLine, "target resistance -10") {
		t.Fatalf("expected roll breakdown terms in line, got %q", rollLine)
	}
}

func TestGeneratedDaemonAlwaysHasOffensiveAbility(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 77, true)

	for archetypeID := range reg.Archetypes {
		for i := 0; i < 40; i++ {
			daemon := engine.generateDaemon(archetypeID, 0, false, "", Stats{})
			hasOffense := false
			for _, ability := range daemon.Abilities {
				effect := reg.Effects[ability.EffectID]
				if effect.Kind == "damage" || effect.Kind == "hybrid" {
					hasOffense = true
				}
			}
			if !hasOffense {
				t.Fatalf("daemon %s generated without an offensive ability: %+v", archetypeID, daemon.Abilities)
			}
		}
	}
}

func TestEnemyAvoidsPatchAtHighIntegrityWhenOffenseExists(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 99, true)
	if _, err := engine.ChooseStarter("firewall"); err != nil {
		t.Fatalf("starter failed: %v", err)
	}

	engine.Run.Combat = &CombatState{
		NodeID:   "n1",
		NodeType: NodeStandard,
		Enemy: Daemon{
			Name:         "Firewall",
			ArchetypeID:  "firewall",
			MaxIntegrity: 50,
			Integrity:    50,
			Speed:        8,
			Stability:    10,
			TraitID:      "volatile",
			Abilities: []Ability{
				{EffectID: "patch", ModifierID: "single"},
				{EffectID: "spike", ModifierID: "single"},
			},
			Statuses: map[string]int{},
		},
		Queue: []Actor{ActorEnemy, ActorPlayer},
		Round: 1,
	}

	choice := engine.pickEnemyAbility()
	if choice.EffectID == "patch" {
		t.Fatalf("expected enemy to avoid patch at full integrity, chose %+v", choice)
	}
}

func TestNonBossEnemiesUseRestrictedModifiers(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	for _, difficulty := range []int{1, 2, 3} {
		for seed := int64(1); seed <= 40; seed++ {
			state := meta.DefaultState()
			engine := New(reg, &state, seed, true)
			enemy := engine.generateEnemyForNode(&Node{ID: "sim", Type: NodeStandard, Difficulty: difficulty})

			for _, ability := range enemy.Abilities {
				if ability.ModifierID == "unstable" {
					t.Fatalf("difficulty %d seed %d: expected non-boss enemies to avoid unstable, got %+v", difficulty, seed, enemy.Abilities)
				}
				if difficulty <= 2 && ability.ModifierID != "single" {
					t.Fatalf("difficulty %d seed %d: expected early non-boss enemies to use single only, got %+v", difficulty, seed, enemy.Abilities)
				}
			}
		}
	}
}

func TestBossUsesRebalancedConfig(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	if reg.Data.Boss.Modifier != "intensify" || reg.Data.Boss.IntegrityBonus != 10 || reg.Data.Boss.SpeedBonus != 2 || reg.Data.Boss.StabilityBonus != 3 {
		t.Fatalf("unexpected boss config: %+v", reg.Data.Boss)
	}

	state := meta.DefaultState()
	engine := New(reg, &state, 7, true)
	boss := engine.generateEnemyForNode(&Node{ID: "boss", Type: NodeBoss, Difficulty: 3})
	for _, ability := range boss.Abilities {
		if ability.ModifierID != "intensify" {
			t.Fatalf("expected boss abilities to use intensify, got %+v", boss.Abilities)
		}
	}
}

func TestFinishCombatWinRestoresIntegrityAfterGrowth(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 11, true)
	if _, err := engine.ChooseStarter("firewall"); err != nil {
		t.Fatalf("starter failed: %v", err)
	}

	active := engine.ActiveDaemon()
	beforeMax := active.MaxIntegrity
	active.Integrity = 1
	engine.Run.Nodes["test"] = &Node{ID: "test", Label: "test", Children: nil}
	engine.Run.Combat = &CombatState{NodeID: "test"}

	engine.finishCombatWin(false, nil, nil)

	expectedMax := beforeMax + reg.Archetypes[active.ArchetypeID].Growth.Integrity
	expectedAfterGrowth := min(expectedMax, 1+reg.Archetypes[active.ArchetypeID].Growth.Integrity)
	expectedRecovery := max(6, int(math.Ceil(float64(expectedMax)*0.20)))
	expectedIntegrity := min(expectedMax, expectedAfterGrowth+expectedRecovery)

	if active.MaxIntegrity != expectedMax || active.Integrity != expectedIntegrity {
		t.Fatalf("expected growth then recovery to apply, got max=%d integrity=%d want max=%d integrity=%d", active.MaxIntegrity, active.Integrity, expectedMax, expectedIntegrity)
	}
}

func TestFinishCombatWinRecoveryCapsAtMaxIntegrity(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 19, true)
	if _, err := engine.ChooseStarter("firewall"); err != nil {
		t.Fatalf("starter failed: %v", err)
	}

	active := engine.ActiveDaemon()
	active.Integrity = active.MaxIntegrity - 1
	engine.Run.Nodes["test"] = &Node{ID: "test", Label: "test", Children: nil}
	engine.Run.Combat = &CombatState{NodeID: "test"}

	engine.finishCombatWin(false, nil, nil)

	if active.Integrity != active.MaxIntegrity {
		t.Fatalf("expected victory recovery to cap at max integrity, got %d/%d", active.Integrity, active.MaxIntegrity)
	}
}
