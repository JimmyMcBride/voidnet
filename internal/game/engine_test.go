package game

import (
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

func TestCaptureThreshold(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}
	state := meta.DefaultState()
	engine := New(reg, &state, 55, true)
	if _, err := engine.ChooseStarter("firewall"); err != nil {
		t.Fatalf("starter failed: %v", err)
	}
	nodeID := engine.NodeChoices()[0]
	if _, err := engine.ChooseNode(nodeID); err != nil {
		t.Fatalf("node failed: %v", err)
	}

	engine.Run.Combat.Enemy.Integrity = engine.Run.Combat.Enemy.MaxIntegrity
	if _, eligible := engine.captureChance(); eligible {
		t.Fatalf("expected capture to be ineligible over 50%% integrity")
	}

	engine.Run.Combat.Enemy.Integrity = engine.Run.Combat.Enemy.MaxIntegrity / 4
	if chance, eligible := engine.captureChance(); !eligible || chance < 5 {
		t.Fatalf("expected capture to be eligible with a clamped chance, got eligible=%v chance=%d", eligible, chance)
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
