package app

import (
	"strings"
	"testing"

	"voidnet/internal/content"
	"voidnet/internal/game"
	"voidnet/internal/meta"
)

func TestSnapshotBuildsStructuredCombatView(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	store := meta.NewStore(t.TempDir() + "/meta.json")
	session := NewSession(reg, store, state, 123, true)

	if _, _, err := session.Apply("starter:firewall"); err != nil {
		t.Fatalf("starter apply failed: %v", err)
	}

	scene := session.Snapshot()
	var nodeChoice string
	for _, choice := range scene.Choices {
		if len(choice.ID) > 5 && choice.ID[:5] == "node:" {
			nodeChoice = choice.ID
			break
		}
	}
	if nodeChoice == "" {
		t.Fatalf("expected a node choice in node select scene")
	}

	if _, _, err := session.Apply(nodeChoice); err != nil {
		t.Fatalf("node apply failed: %v", err)
	}

	scene = session.Snapshot()
	if scene.Combat == nil {
		t.Fatalf("expected combat payload to be populated")
	}
	if scene.Combat.Player.Label == "" || scene.Combat.Enemy.Label == "" {
		t.Fatalf("expected combat labels to be present: %+v", scene.Combat)
	}
	if scene.Combat.Player.IntegrityMax <= 0 || scene.Combat.Enemy.IntegrityMax <= 0 {
		t.Fatalf("expected combat integrity values to be populated: %+v", scene.Combat)
	}
}

func TestSnapshotBuildsNodeMapWithStableLanes(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	store := meta.NewStore(t.TempDir() + "/meta.json")
	session := NewSession(reg, store, state, 12345, true)

	if _, _, err := session.Apply("starter:firewall"); err != nil {
		t.Fatalf("starter apply failed: %v", err)
	}

	scene := session.Snapshot()
	if scene.NodeMap == nil {
		t.Fatalf("expected node map payload in node select scene")
	}

	nodesByDepth := map[int][]NodeMapNode{}
	for _, node := range scene.NodeMap.Nodes {
		nodesByDepth[node.Depth] = append(nodesByDepth[node.Depth], node)
	}

	if len(nodesByDepth[0]) != 1 || nodesByDepth[0][0].ID != "start" || nodesByDepth[0][0].Lane != 1 {
		t.Fatalf("expected centered start node, got %+v", nodesByDepth[0])
	}

	for depth, nodes := range nodesByDepth {
		if depth == 0 {
			continue
		}
		switch len(nodes) {
		case 1:
			if nodes[0].Lane != 1 {
				t.Fatalf("expected single node at depth %d to be centered, got %+v", depth, nodes)
			}
		case 2:
			lanes := map[int]bool{}
			for _, node := range nodes {
				lanes[node.Lane] = true
			}
			if !lanes[0] || !lanes[2] {
				t.Fatalf("expected split depth %d to use lanes 0 and 2, got %+v", depth, nodes)
			}
		}
	}
}

func TestChooseNodeShowsEnemyFirstCombatBeforeDamageLands(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	store := meta.NewStore(t.TempDir() + "/meta.json")
	session := NewSession(reg, store, state, 2, true)

	if _, _, err := session.Apply("starter:firewall"); err != nil {
		t.Fatalf("starter apply failed: %v", err)
	}

	scene, events, err := session.Apply("node:n1")
	if err != nil {
		t.Fatalf("node apply failed: %v", err)
	}
	if scene.Combat == nil {
		t.Fatalf("expected combat scene after entering node")
	}
	if scene.Combat.PlayerTurn {
		t.Fatalf("expected enemy-first opener for regression seed")
	}
	if scene.Combat.Player.IntegrityCurrent != scene.Combat.Player.IntegrityMax {
		t.Fatalf("expected player to enter combat uninjured, got %d/%d", scene.Combat.Player.IntegrityCurrent, scene.Combat.Player.IntegrityMax)
	}
	for _, event := range events {
		if strings.Contains(event.Message, " used ") {
			t.Fatalf("expected entry events only before enemy playback, got %+v", events)
		}
	}
}

func TestAdvanceEnemyTurnResolvesOneEnemyAction(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	store := meta.NewStore(t.TempDir() + "/meta.json")
	session := NewSession(reg, store, state, 2, true)

	if _, _, err := session.Apply("starter:firewall"); err != nil {
		t.Fatalf("starter apply failed: %v", err)
	}

	before, _, err := session.Apply("node:n1")
	if err != nil {
		t.Fatalf("node apply failed: %v", err)
	}
	if before.Combat == nil {
		t.Fatalf("expected combat scene after entering node")
	}

	after, events, err := session.AdvanceEnemyTurn()
	if err != nil {
		t.Fatalf("advance enemy turn failed: %v", err)
	}
	if after.Combat == nil {
		t.Fatalf("expected combat to remain active after one enemy action")
	}
	if !after.Combat.PlayerTurn {
		t.Fatalf("expected control to return to the player after one enemy action")
	}
	if after.Combat.Player.IntegrityCurrent >= before.Combat.Player.IntegrityCurrent {
		t.Fatalf("expected deterministic enemy opener to reduce player integrity, got before=%d after=%d", before.Combat.Player.IntegrityCurrent, after.Combat.Player.IntegrityCurrent)
	}
	if len(events) == 0 || !strings.Contains(events[0].Message, "Enemy") || !strings.Contains(events[0].Message, " used ") {
		t.Fatalf("expected enemy action log, got %+v", events)
	}
}

func TestCombatChoiceDetailsExplainStatusEffects(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	details := abilityDetails(game.Ability{EffectID: "corrupt", ModifierID: "single"}, reg)
	if details == nil {
		t.Fatalf("expected details for ability")
	}

	rendered := strings.Join(details.Lines, "\n")
	if !strings.Contains(rendered, "applies Corrupted for 3 turns (-10 accuracy)") {
		t.Fatalf("expected status effect explanation in ability details, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Hit chance is lower against targets with higher Stability.") {
		t.Fatalf("expected stability explanation in hostile ability details, got:\n%s", rendered)
	}
}

func TestIsolateDetailsExplainStability(t *testing.T) {
	details := isolateDetails()
	if details == nil {
		t.Fatalf("expected isolate details")
	}

	rendered := strings.Join(details.Lines, "\n")
	if !strings.Contains(rendered, "Higher target Stability lowers the capture chance.") {
		t.Fatalf("expected isolate details to explain Stability, got:\n%s", rendered)
	}
}

func TestInspectSceneIncludesStatGlossary(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	store := meta.NewStore(t.TempDir() + "/meta.json")
	session := NewSession(reg, store, state, 123, true)

	if _, _, err := session.Apply("starter:firewall"); err != nil {
		t.Fatalf("starter apply failed: %v", err)
	}
	if _, _, err := session.Apply("node:n1"); err != nil {
		t.Fatalf("node apply failed: %v", err)
	}

	scene, _, err := session.Apply("inspect")
	if err != nil {
		t.Fatalf("inspect apply failed: %v", err)
	}

	rendered := strings.Join(scene.Lines, "\n")
	if !strings.Contains(rendered, "Stat key:") {
		t.Fatalf("expected inspect scene to include stat glossary, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "STB Stability: resists hostile effects and lowers isolation chance against this daemon.") {
		t.Fatalf("expected inspect scene to explain Stability, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Status reference:") {
		t.Fatalf("expected inspect scene to include status reference, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Corrupted: -10 accuracy") {
		t.Fatalf("expected inspect scene to explain core status effects, got:\n%s", rendered)
	}
}

func TestInspectSceneExplainsTraitsAndActiveStatuses(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	store := meta.NewStore(t.TempDir() + "/meta.json")
	session := NewSession(reg, store, state, 123, true)

	if _, _, err := session.Apply("starter:firewall"); err != nil {
		t.Fatalf("starter apply failed: %v", err)
	}
	if _, _, err := session.Apply("node:n1"); err != nil {
		t.Fatalf("node apply failed: %v", err)
	}

	player := session.engine.ActiveDaemon()
	player.TraitID = "encrypted"
	player.Statuses = map[string]int{"corrupted": 2}
	session.engine.Run.Combat.Enemy.TraitID = "overclocked"
	session.engine.Run.Combat.Enemy.Statuses = map[string]int{"leaking": 3, "delayed": 1}

	scene, _, err := session.Apply("inspect")
	if err != nil {
		t.Fatalf("inspect apply failed: %v", err)
	}

	rendered := strings.Join(scene.Lines, "\n")
	if !strings.Contains(rendered, "Trait: Encrypted - 10 hostile-effect resistance") {
		t.Fatalf("expected inspect scene to explain trait mechanics, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Active status: Corrupted(2): -10 accuracy") {
		t.Fatalf("expected inspect scene to explain player active status, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Trait: Overclocked - +4 speed, -3 stability") {
		t.Fatalf("expected inspect scene to explain enemy trait mechanics, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Leaking(3): 3 damage at end of turn") || !strings.Contains(rendered, "Delayed(1): -4 speed") {
		t.Fatalf("expected inspect scene to explain enemy active statuses, got:\n%s", rendered)
	}
}
