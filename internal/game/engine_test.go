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

func TestLeakingTicksOnlyAtEndOfAffectedDaemonTurn(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 71, true)
	engine.Run.Roster = []Daemon{{
		ID:           "player",
		Name:         "Firewall",
		ArchetypeID:  "firewall",
		MaxIntegrity: 50,
		Integrity:    50,
		Speed:        12,
		Stability:    12,
		TraitID:      "encrypted",
		Abilities: []Ability{
			{EffectID: "spike", ModifierID: "single"},
			{EffectID: "patch", ModifierID: "single"},
		},
		Statuses: map[string]int{},
	}}
	engine.Run.ActiveIndex = 0
	engine.Run.Combat = &CombatState{
		NodeID:   "n1",
		NodeType: NodeStandard,
		Enemy: Daemon{
			ID:           "enemy",
			Name:         "MemoryLeaker",
			ArchetypeID:  "memoryleaker",
			MaxIntegrity: 44,
			Integrity:    44,
			Speed:        10,
			Stability:    9,
			TraitID:      "persistent",
			Abilities: []Ability{
				{EffectID: "spike", ModifierID: "single"},
				{EffectID: "leak", ModifierID: "single"},
			},
			Statuses: map[string]int{},
		},
		Queue: []Actor{ActorPlayer, ActorEnemy},
		Round: 1,
	}
	engine.Run.Phase = PhaseCombat

	engine.Run.Combat.Enemy.Statuses["leaking"] = 2

	playerLines := engine.resolveAbility(ActorPlayer, Ability{EffectID: "spike", ModifierID: "single"})
	if strings.Contains(strings.Join(playerLines, "\n"), "Enemy MemoryLeaker suffered 3 damage from Leaking.") {
		t.Fatalf("expected enemy leaking to wait until enemy turn end, got %+v", playerLines)
	}
	if engine.Run.Combat.Enemy.Integrity != 35 {
		t.Fatalf("expected only direct spike damage on player turn, got %d", engine.Run.Combat.Enemy.Integrity)
	}

	engine.Run.Combat.Queue = []Actor{ActorEnemy}
	enemyLines := engine.resolveAbility(ActorEnemy, Ability{EffectID: "spike", ModifierID: "single"})
	if !strings.Contains(strings.Join(enemyLines, "\n"), "Enemy MemoryLeaker suffered 3 damage from Leaking.") {
		t.Fatalf("expected enemy leaking to tick at end of enemy turn, got %+v", enemyLines)
	}
	if engine.Run.Combat.Enemy.Integrity != 32 {
		t.Fatalf("expected leaking damage to land on enemy turn end, got %d", engine.Run.Combat.Enemy.Integrity)
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

func TestBattleWinBanksMaintenanceChargeUpToOne(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 60, true)
	engine.Run.Roster = []Daemon{
		{ID: "a", Name: "Firewall", ArchetypeID: "firewall", MaxIntegrity: 50, Integrity: 30, Speed: 8, Stability: 12, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "patch", ModifierID: "single"}}, TraitID: "encrypted", Statuses: map[string]int{}},
	}
	engine.Run.ActiveIndex = 0
	engine.Run.PositionNodeID = "start"
	engine.Run.Phase = PhaseNodeSelect
	node := engine.Run.Nodes["n1"]
	node.Visible = true

	if _, err := engine.ChooseNode("n1"); err != nil {
		t.Fatalf("choose node failed: %v", err)
	}
	engine.Run.MaintenanceCharge = 0
	engine.finishCombatWin(false, nil, []string{"Node complete."})
	if engine.Run.MaintenanceCharge != 1 {
		t.Fatalf("expected win to bank one maintenance charge, got %d", engine.Run.MaintenanceCharge)
	}
	if !strings.Contains(strings.Join(engine.Run.PendingRewardLines, "\n"), "Maintenance charge ready (1/1).") {
		t.Fatalf("expected maintenance charge log, got %+v", engine.Run.PendingRewardLines)
	}

	engine.Run.MaintenanceCharge = 1
	engine.Run.Phase = PhaseCombat
	engine.Run.Combat = &CombatState{
		NodeID:   "n2",
		NodeType: NodeStandard,
		Enemy: Daemon{
			ID:           "enemy2",
			Name:         "NullPointer",
			ArchetypeID:  "nullpointer",
			MaxIntegrity: 1,
			Integrity:    1,
			Speed:        1,
			Stability:    1,
			Abilities:    []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "corrupt", ModifierID: "single"}},
			TraitID:      "persistent",
			Statuses:     map[string]int{},
		},
		Queue: []Actor{ActorPlayer},
		Round: 1,
	}
	engine.finishCombatWin(false, nil, []string{"Second node complete."})
	if engine.Run.MaintenanceCharge != 1 {
		t.Fatalf("expected maintenance charge to cap at one, got %d", engine.Run.MaintenanceCharge)
	}
	if !strings.Contains(strings.Join(engine.Run.PendingRewardLines, "\n"), "Maintenance charge ready (1/1).") {
		t.Fatalf("expected capped maintenance charge log, got %+v", engine.Run.PendingRewardLines)
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
		{ID: "b", Name: "Scheduler", ArchetypeID: "scheduler", MaxIntegrity: 42, Integrity: 21, Speed: 15, Stability: 9, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "delay", ModifierID: "single"}}, TraitID: "persistent", Statuses: map[string]int{"corrupted": 2}},
	}
	engine.Run.ActiveIndex = 0
	engine.Run.Phase = PhaseMaintenance
	engine.Run.MaintenanceCharge = 1

	lines, err := engine.ChooseMaintenance("repair")
	if err != nil {
		t.Fatalf("repair failed: %v", err)
	}
	if engine.Run.Phase != PhaseSelectActive || engine.Run.SelectActiveMode != SelectActiveRepair {
		t.Fatalf("expected repair to route to target selection, got phase=%s mode=%s", engine.Run.Phase, engine.Run.SelectActiveMode)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Choose a daemon to repair.") {
		t.Fatalf("expected repair targeting prompt, got %+v", lines)
	}

	lines, err = engine.ChooseActive(1)
	if err != nil {
		t.Fatalf("repair target failed: %v", err)
	}
	if engine.Run.Roster[0].Integrity != 10 {
		t.Fatalf("expected untargeted daemon to remain unchanged, got %d", engine.Run.Roster[0].Integrity)
	}
	if engine.Run.Roster[1].Integrity != 34 {
		t.Fatalf("expected repair target to restore 13 integrity, got %d", engine.Run.Roster[1].Integrity)
	}
	if engine.Run.MaintenanceCharge != 0 {
		t.Fatalf("expected repair to consume maintenance charge, got %d", engine.Run.MaintenanceCharge)
	}
	if _, ok := engine.Run.Roster[1].Statuses["corrupted"]; ok {
		t.Fatalf("expected repair to cleanse one negative status from the target")
	}
	if engine.Run.Phase != PhaseMaintenance {
		t.Fatalf("expected repair target resolution to return to maintenance, got %s", engine.Run.Phase)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "restored 13 Health") {
		t.Fatalf("expected repair log, got %+v", lines)
	}

	engine.Run.ActiveIndex = 0
	engine.Run.Phase = PhaseMaintenance
	engine.Run.Roster[0].Integrity = 40
	engine.Run.MaintenanceCharge = 1
	lines, err = engine.ChooseMaintenance("fortify")
	if err != nil {
		t.Fatalf("fortify failed: %v", err)
	}
	if engine.Run.Phase != PhaseSelectActive || engine.Run.SelectActiveMode != SelectActiveFortify {
		t.Fatalf("expected fortify to route to target selection, got phase=%s mode=%s", engine.Run.Phase, engine.Run.SelectActiveMode)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Choose a daemon to fortify.") {
		t.Fatalf("expected fortify targeting prompt, got %+v", lines)
	}

	lines, err = engine.ChooseActive(0)
	if err != nil {
		t.Fatalf("fortify target failed: %v", err)
	}
	if engine.Run.Phase != PhaseMaintenance || engine.Run.Roster[0].Integrity != 45 || engine.Run.Roster[0].Statuses["stabilized"] != 2 {
		t.Fatalf("expected fortify target to heal and grant stabilized, got phase=%s daemon=%+v", engine.Run.Phase, engine.Run.Roster[0])
	}
	if engine.Run.MaintenanceCharge != 0 {
		t.Fatalf("expected fortify to consume maintenance charge, got %d", engine.Run.MaintenanceCharge)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "gained Stabilized") {
		t.Fatalf("expected fortify log, got %+v", lines)
	}

	engine.Run.Phase = PhaseMaintenance
	engine.Run.MaintenanceCharge = 1
	if _, err := engine.ChooseMaintenance("repair"); err != nil {
		t.Fatalf("repair retarget start failed: %v", err)
	}
	lines, err = engine.CancelMaintenanceTargeting()
	if err != nil {
		t.Fatalf("cancel maintenance targeting failed: %v", err)
	}
	if engine.Run.Phase != PhaseMaintenance || engine.Run.SelectActiveMode != SelectActiveNone {
		t.Fatalf("expected maintenance cancel to return to maintenance, got phase=%s mode=%s", engine.Run.Phase, engine.Run.SelectActiveMode)
	}
	if len(lines) != 0 {
		t.Fatalf("expected maintenance cancel to be silent, got %+v", lines)
	}

	engine.Run.ActiveIndex = 0
	engine.Run.Phase = PhaseNodeSelect
	lines, err = engine.BeginRotateLead()
	if err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	if engine.Run.Phase != PhaseSelectActive || engine.Run.SelectActiveMode != SelectActiveRotateLead {
		t.Fatalf("expected rotate to route through select active, got phase=%s mode=%s", engine.Run.Phase, engine.Run.SelectActiveMode)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Choose a reserve daemon") {
		t.Fatalf("expected rotate guidance log, got %+v", lines)
	}

	lines, err = engine.CancelRotateLead()
	if err != nil {
		t.Fatalf("cancel rotate failed: %v", err)
	}
	if engine.Run.Phase != PhaseNodeSelect || engine.Run.SelectActiveMode != SelectActiveNone {
		t.Fatalf("expected rotate cancel to return to node select, got phase=%s mode=%s", engine.Run.Phase, engine.Run.SelectActiveMode)
	}
	if len(lines) != 0 {
		t.Fatalf("expected rotate cancel to be silent, got %+v", lines)
	}

	lines, err = engine.BeginRotateLead()
	if err != nil {
		t.Fatalf("rotate restart failed: %v", err)
	}

	lines, err = engine.ChooseActive(1)
	if err != nil {
		t.Fatalf("choose active during rotate failed: %v", err)
	}
	if engine.Run.Phase != PhaseNodeSelect || engine.Run.ActiveIndex != 1 {
		t.Fatalf("expected rotated daemon to become active and return to node select, got phase=%s active=%d", engine.Run.Phase, engine.Run.ActiveIndex)
	}
	if engine.Run.Roster[1].Integrity != 42 {
		t.Fatalf("expected rotated daemon to heal up to max integrity, got %d", engine.Run.Roster[1].Integrity)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Scheduler restored 8 Health during maintenance.") {
		t.Fatalf("expected rotate heal log, got %+v", lines)
	}

	engine.Run.Phase = PhaseMaintenance
	engine.Run.MaintenanceCharge = 0
	if _, err := engine.ChooseMaintenance("repair"); err == nil {
		t.Fatalf("expected repair without a stored charge to fail")
	}
}

func TestMergeFlowConsumesBranchAndAddsThirdAbility(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 61, true)
	engine.Run.Roster = []Daemon{
		{ID: "a", Name: "Firewall", ArchetypeID: "firewall", MaxIntegrity: 50, Integrity: 30, Speed: 8, Stability: 12, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "patch", ModifierID: "single"}}, TraitID: "encrypted", Statuses: map[string]int{}},
		{ID: "b", Name: "Scheduler", ArchetypeID: "scheduler", MaxIntegrity: 42, Integrity: 36, Speed: 15, Stability: 9, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "delay", ModifierID: "intensify"}}, TraitID: "persistent", Statuses: map[string]int{}},
	}
	engine.Run.ActiveIndex = 0
	engine.Run.Phase = PhaseNodeSelect

	if !engine.CanBeginMerge() {
		t.Fatalf("expected merge pair to be available")
	}
	if _, err := engine.BeginMerge(); err != nil {
		t.Fatalf("begin merge failed: %v", err)
	}
	if engine.Run.Phase != PhaseSelectActive || engine.Run.SelectActiveMode != SelectActiveMergeBase {
		t.Fatalf("expected merge to start at base selection, got phase=%s mode=%s", engine.Run.Phase, engine.Run.SelectActiveMode)
	}
	if _, err := engine.ChooseActive(0); err != nil {
		t.Fatalf("choose base failed: %v", err)
	}
	if engine.Run.SelectActiveMode != SelectActiveMergeFork {
		t.Fatalf("expected fork selection, got mode=%s", engine.Run.SelectActiveMode)
	}
	if _, err := engine.ChooseActive(1); err != nil {
		t.Fatalf("choose fork failed: %v", err)
	}
	if engine.Run.Phase != PhaseMergeConfirm {
		t.Fatalf("expected merge confirm phase, got %s", engine.Run.Phase)
	}

	lines, err := engine.ConfirmMerge()
	if err != nil {
		t.Fatalf("confirm merge failed: %v", err)
	}
	if engine.Run.Phase != PhaseMergeResult {
		t.Fatalf("expected merge to route through merge result, got %s", engine.Run.Phase)
	}
	if len(engine.Run.Roster) != 1 {
		t.Fatalf("expected fork to be consumed, got roster %+v", engine.Run.Roster)
	}
	base := engine.Run.Roster[0]
	if base.MergeLevel != 1 {
		t.Fatalf("expected merged base level 1, got %+v", base)
	}
	if len(base.Abilities) != 3 {
		t.Fatalf("expected third skill slot after merge, got %+v", base.Abilities)
	}
	if base.Abilities[2].EffectID != "delay" {
		t.Fatalf("expected fork special move to become slot 3, got %+v", base.Abilities[2])
	}
	if base.Speed != 10 {
		t.Fatalf("expected scheduler merge to add +2 speed, got %+v", base)
	}
	if base.Integrity != 43 {
		t.Fatalf("expected merge heal to restore 13 health after stat gain, got %+v", base)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Firewall +1 gained Delay + ") {
		t.Fatalf("expected merge result lines to reveal gained skill, got %+v", lines)
	}
	if engine.Run.PendingMergeResult == nil || engine.Run.PendingMergeResult.ResultName != "Firewall +1" {
		t.Fatalf("expected structured merge result to be populated, got %+v", engine.Run.PendingMergeResult)
	}
	if _, err := engine.Continue(); err != nil {
		t.Fatalf("continue from merge result failed: %v", err)
	}
	if engine.Run.Phase != PhaseNodeSelect {
		t.Fatalf("expected continue to return to node select, got %s", engine.Run.Phase)
	}
}

func TestMergeRestrictionsRejectSameArchetypeAndPlusOne(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 61, true)
	engine.Run.Roster = []Daemon{
		{ID: "a", Name: "Firewall", MergeLevel: 1, ArchetypeID: "firewall", MaxIntegrity: 50, Integrity: 50, Speed: 8, Stability: 12, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "patch", ModifierID: "single"}, {EffectID: "delay", ModifierID: "single"}}, TraitID: "encrypted", Statuses: map[string]int{}},
		{ID: "b", Name: "Firewall", ArchetypeID: "firewall", MaxIntegrity: 48, Integrity: 48, Speed: 8, Stability: 12, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "patch", ModifierID: "single"}}, TraitID: "encrypted", Statuses: map[string]int{}},
		{ID: "c", Name: "Scheduler", ArchetypeID: "scheduler", MaxIntegrity: 42, Integrity: 42, Speed: 15, Stability: 9, Abilities: []Ability{{EffectID: "spike", ModifierID: "single"}, {EffectID: "delay", ModifierID: "single"}}, TraitID: "persistent", Statuses: map[string]int{}},
	}

	if engine.canMergeAsBase(0) {
		t.Fatalf("expected +1 daemon to be ineligible as base")
	}
	if engine.canMergePair(1, 0) {
		t.Fatalf("expected +1 daemon to be ineligible as fork")
	}
	if engine.canMergePair(1, 1) {
		t.Fatalf("expected same daemon merge to be ineligible")
	}
	if engine.canMergePair(1, 0) {
		t.Fatalf("expected same-archetype or +1 merge to be rejected")
	}
	if !engine.canMergePair(1, 2) {
		t.Fatalf("expected different unmerged archetypes to be mergeable")
	}
}
