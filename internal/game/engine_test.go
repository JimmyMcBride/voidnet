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
	if got := []string{a.Abilities[0].EffectID, a.Abilities[1].EffectID}; got[0] != "spike" || got[1] != "patch" {
		t.Fatalf("expected firewall starter to use fixed kit, got %+v", a.Abilities)
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

func TestGeneratedDaemonsUseFixedArchetypeKits(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 77, true)

	expected := map[string][2]string{
		"nullpointer":  {"spike", "corrupt"},
		"memoryleaker": {"spike", "leak"},
		"firewall":     {"spike", "patch"},
		"scheduler":    {"spike", "delay"},
	}
	for archetypeID, kit := range expected {
		for i := 0; i < 40; i++ {
			daemon := engine.generateDaemon(archetypeID, 0, false, "", Stats{})
			if daemon.Abilities[0].EffectID != kit[0] || daemon.Abilities[1].EffectID != kit[1] {
				t.Fatalf("daemon %s generated with wrong fixed kit: %+v", archetypeID, daemon.Abilities)
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
	expectedRecovery := max(8, int(math.Ceil(float64(expectedMax)*0.25)))
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

func TestStarterReceivesStabilizedForFirstCombatOnly(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 9, true)
	if _, err := engine.ChooseStarter("firewall"); err != nil {
		t.Fatalf("starter failed: %v", err)
	}

	if _, err := engine.ChooseNode("n1"); err != nil {
		t.Fatalf("first node failed: %v", err)
	}
	if got := engine.ActiveDaemon().Statuses["stabilized"]; got != 2 {
		t.Fatalf("expected first combat boost to apply stabilized(2), got %d", got)
	}

	engine.Run.Combat = nil
	engine.Run.Phase = PhaseNodeSelect
	delete(engine.ActiveDaemon().Statuses, "stabilized")
	engine.Run.PositionNodeID = "n1"
	engine.revealChildren("n1")

	if _, err := engine.ChooseNode(engine.currentChoicesForNodes()[0]); err != nil {
		t.Fatalf("second node failed: %v", err)
	}
	if _, ok := engine.ActiveDaemon().Statuses["stabilized"]; ok {
		t.Fatalf("expected first-combat boost to stop after the opener")
	}
}

func TestDelayedSkipsNextTurnOnce(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 44, true)
	engine.Run.Roster = []Daemon{{
		ID:           "player",
		Name:         "Firewall",
		ArchetypeID:  "firewall",
		MaxIntegrity: 48,
		Integrity:    48,
		Speed:        12,
		Stability:    12,
		TraitID:      "encrypted",
		Abilities:    []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "patch", ModifierID: "single"}},
		Statuses:     map[string]int{},
	}}
	engine.Run.ActiveIndex = 0
	engine.Run.Combat = &CombatState{
		NodeID:   "n1",
		NodeType: NodeStandard,
		Enemy: Daemon{
			ID:           "enemy",
			Name:         "Scheduler",
			ArchetypeID:  "scheduler",
			MaxIntegrity: 42,
			Integrity:    42,
			Speed:        8,
			Stability:    9,
			TraitID:      "persistent",
			Abilities:    []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "delay", ModifierID: "single"}},
			Statuses:     map[string]int{"delayed": 1},
		},
		Queue: []Actor{ActorPlayer, ActorEnemy},
		Round: 1,
	}
	engine.Run.Phase = PhaseCombat

	lines, err := engine.UseAbility(0)
	if err != nil {
		t.Fatalf("use ability failed: %v", err)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Enemy Scheduler lost the turn to Delayed.") {
		t.Fatalf("expected delayed skip to be logged, got %+v", lines)
	}
	if _, ok := engine.Run.Combat.Enemy.Statuses["delayed"]; ok {
		t.Fatalf("expected delayed to clear after the skipped turn")
	}
	if !engine.IsPlayerTurn() {
		t.Fatalf("expected turn flow to continue after the forced skip")
	}
}

func TestContinueRoutesThroughMaintenance(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 51, true)
	engine.Run.Phase = PhaseReward
	engine.Run.ActiveIndex = 0
	engine.Run.Roster = []Daemon{{ID: "a", Name: "Firewall", ArchetypeID: "firewall", MaxIntegrity: 48, Integrity: 30, Speed: 8, Stability: 12, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "patch", ModifierID: "single"}}, TraitID: "encrypted", Statuses: map[string]int{}}}

	if _, err := engine.Continue(); err != nil {
		t.Fatalf("continue failed: %v", err)
	}
	if engine.Run.Phase != PhaseMaintenance {
		t.Fatalf("expected reward continue to route to maintenance, got %s", engine.Run.Phase)
	}

	engine.Run.Phase = PhaseReward
	engine.Run.ActiveIndex = -1
	engine.Run.Roster = append(engine.Run.Roster, Daemon{ID: "b", Name: "Scheduler", ArchetypeID: "scheduler", MaxIntegrity: 42, Integrity: 20, Speed: 15, Stability: 9, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "delay", ModifierID: "single"}}, TraitID: "persistent", Statuses: map[string]int{}})
	if _, err := engine.Continue(); err != nil {
		t.Fatalf("continue after loss failed: %v", err)
	}
	if engine.Run.Phase != PhaseSelectActive || engine.Run.SelectActiveMode != SelectActiveAfterLoss {
		t.Fatalf("expected post-loss reward continue to route through active selection, got phase=%s mode=%s", engine.Run.Phase, engine.Run.SelectActiveMode)
	}
}

func TestChooseMaintenanceAppliesConfiguredEffects(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 61, true)
	engine.Run.Roster = []Daemon{
		{ID: "a", Name: "Firewall", ArchetypeID: "firewall", MaxIntegrity: 50, Integrity: 10, Speed: 8, Stability: 12, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "patch", ModifierID: "single"}}, TraitID: "encrypted", Statuses: map[string]int{"corrupted": 2}},
		{ID: "b", Name: "Scheduler", ArchetypeID: "scheduler", MaxIntegrity: 42, Integrity: 21, Speed: 15, Stability: 9, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "delay", ModifierID: "single"}}, TraitID: "persistent", Statuses: map[string]int{}},
	}
	engine.Run.ActiveIndex = 0
	engine.Run.Phase = PhaseMaintenance

	lines, err := engine.ChooseMaintenance("repair")
	if err != nil {
		t.Fatalf("repair failed: %v", err)
	}
	if engine.Run.Phase != PhaseNodeSelect {
		t.Fatalf("expected repair to return to node select, got %s", engine.Run.Phase)
	}
	if engine.Run.Roster[0].Integrity != 25 {
		t.Fatalf("expected repair to restore 15 integrity, got %d", engine.Run.Roster[0].Integrity)
	}
	if _, ok := engine.Run.Roster[0].Statuses["corrupted"]; ok {
		t.Fatalf("expected repair to cleanse one negative status")
	}
	if !strings.Contains(strings.Join(lines, "\n"), "restored 15 Integrity") {
		t.Fatalf("expected repair log, got %+v", lines)
	}

	engine.Run.ActiveIndex = 0
	engine.Run.Phase = PhaseMaintenance
	engine.Run.Roster[0].Integrity = 40
	lines, err = engine.ChooseMaintenance("fortify")
	if err != nil {
		t.Fatalf("fortify failed: %v", err)
	}
	if engine.Run.Phase != PhaseNodeSelect || engine.Run.Roster[0].Integrity != 45 || engine.Run.Roster[0].Statuses["stabilized"] != 2 {
		t.Fatalf("expected fortify to heal and grant stabilized, got phase=%s daemon=%+v", engine.Run.Phase, engine.Run.Roster[0])
	}
	if !strings.Contains(strings.Join(lines, "\n"), "gained Stabilized") {
		t.Fatalf("expected fortify log, got %+v", lines)
	}

	engine.Run.ActiveIndex = 0
	engine.Run.Phase = PhaseMaintenance
	lines, err = engine.ChooseMaintenance("rotate")
	if err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	if engine.Run.Phase != PhaseSelectActive || engine.Run.SelectActiveMode != SelectActiveRotateLead {
		t.Fatalf("expected rotate to route through select active, got phase=%s mode=%s", engine.Run.Phase, engine.Run.SelectActiveMode)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Choose a reserve daemon") {
		t.Fatalf("expected rotate guidance log, got %+v", lines)
	}

	lines, err = engine.ChooseActive(1)
	if err != nil {
		t.Fatalf("choose active during rotate failed: %v", err)
	}
	if engine.Run.Phase != PhaseNodeSelect || engine.Run.ActiveIndex != 1 {
		t.Fatalf("expected rotated daemon to become active and return to node select, got phase=%s active=%d", engine.Run.Phase, engine.Run.ActiveIndex)
	}
	if engine.Run.Roster[1].Integrity != 30 {
		t.Fatalf("expected rotated daemon to heal by 20%%, got %d", engine.Run.Roster[1].Integrity)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Scheduler restored 9 Integrity during maintenance.") {
		t.Fatalf("expected rotate heal log, got %+v", lines)
	}
}
