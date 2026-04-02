package ui

import (
	"fmt"
	"strings"
	"testing"

	"voidnet/internal/app"
	"voidnet/internal/content"
	"voidnet/internal/meta"
)

func newCombatTestModel(t *testing.T, seed int64) model {
	t.Helper()

	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, seed, true)
	model := newModel(session)
	model.width = 100
	model.height = 40
	model.syncBarWidth()
	model.applyChoice("starter:firewall")
	model.applyChoice("node:n1")
	if model.scene.Combat == nil {
		t.Fatalf("expected combat scene for test setup")
	}
	return model
}

func TestApplyChoiceResetsSelectionToFirstOption(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)
	model := newModel(session)
	model.selectedIndex = 1

	model.applyChoice("starter:firewall")

	if model.selectedIndex != 0 {
		t.Fatalf("expected selection to reset to first option, got %d", model.selectedIndex)
	}

	choice, ok := selectedChoice(model.scene, model.selectedIndex)
	if !ok {
		t.Fatalf("expected a selectable option after scene transition")
	}
	if choice.ID != "node:n1" && choice.ID != "node:n2" {
		t.Fatalf("expected first node option to be selected, got %q", choice.ID)
	}
}

func TestCombatPlaybackDefersEnemyFirstDamageUntilImpactBeat(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 2, true)
	model := newModel(session)

	model.applyChoice("starter:firewall")
	model.applyChoice("node:n1")

	if model.scene.Combat == nil {
		t.Fatalf("expected combat scene after node entry")
	}
	if model.playback == nil {
		t.Fatalf("expected playback to start for combat entry")
	}
	if model.scene.Combat.Player.IntegrityCurrent != model.scene.Combat.Player.IntegrityMax {
		t.Fatalf("expected player HP to remain full before playback impact, got %d/%d", model.scene.Combat.Player.IntegrityCurrent, model.scene.Combat.Player.IntegrityMax)
	}
	if len(model.playbackLines) == 0 || !strings.HasPrefix(model.playbackLines[0], "Entered ") {
		t.Fatalf("expected first playback beat to announce combat entry, got %+v", model.playbackLines)
	}

	model.advancePlayback()
	model.advancePlayback()
	if model.playbackActor != actorEnemy || len(model.playbackLines) == 0 || model.playbackLines[0] != "Enemy turn. Acting first." {
		t.Fatalf("expected enemy turn banner after entry beats, got actor=%q lines=%+v", model.playbackActor, model.playbackLines)
	}
	if model.scene.Combat.Player.IntegrityCurrent != model.scene.Combat.Player.IntegrityMax {
		t.Fatalf("expected no damage before enemy impact, got %d/%d", model.scene.Combat.Player.IntegrityCurrent, model.scene.Combat.Player.IntegrityMax)
	}

	model.advancePlayback()
	model.advancePlayback()
	if model.scene.Combat.Player.IntegrityCurrent != model.scene.Combat.Player.IntegrityMax {
		t.Fatalf("expected no damage before impact beat, got %d/%d", model.scene.Combat.Player.IntegrityCurrent, model.scene.Combat.Player.IntegrityMax)
	}

	model.advancePlayback()
	if model.scene.Combat.Player.IntegrityCurrent != 44 {
		t.Fatalf("expected enemy impact beat to land damage, got %d", model.scene.Combat.Player.IntegrityCurrent)
	}
}

func TestFastForwardPlaybackAppliesResolvedEnemySequence(t *testing.T) {
	model := newCombatTestModel(t, 2)
	model.advancePlayback()
	model.advancePlayback()

	if model.playback == nil {
		t.Fatalf("expected enemy playback to be active before fast-forward")
	}

	model.fastForwardPlayback()

	if model.playback != nil {
		t.Fatalf("expected fast-forward to finish the current sequence")
	}
	if model.scene.Combat == nil {
		t.Fatalf("expected combat to remain active after enemy opener")
	}
	if model.scene.Combat.Player.IntegrityCurrent != 44 {
		t.Fatalf("expected fast-forward to apply resolved enemy damage, got %d", model.scene.Combat.Player.IntegrityCurrent)
	}
	if !model.scene.Combat.PlayerTurn {
		t.Fatalf("expected control to return to the player after fast-forwarded enemy turn")
	}
}

func TestCombatRenderShowsSeparateLogCard(t *testing.T) {
	model := newCombatTestModel(t, 2)

	view := model.render()

	if !strings.Contains(view, "COMBAT LOG") {
		t.Fatalf("expected dedicated combat log card, got:\n%s", view)
	}
	if strings.Contains(view, "Last resolution:") {
		t.Fatalf("expected encounter card to stop rendering last resolution text")
	}
	if strings.Contains(view, "Live signal") {
		t.Fatalf("expected live signal block to be removed from encounter card")
	}
}

func TestCombatLogAccumulatesDuringPlayback(t *testing.T) {
	model := newCombatTestModel(t, 2)

	if len(model.combatLogLines) != 1 || !strings.HasPrefix(model.combatLogLines[0], "Entered ") {
		t.Fatalf("expected combat log to start with entry beat, got %+v", model.combatLogLines)
	}

	model.advancePlayback()
	model.advancePlayback()

	if len(model.combatLogLines) < 3 {
		t.Fatalf("expected combat log to accumulate revealed beats, got %+v", model.combatLogLines)
	}
	if model.combatLogLines[2] != "Enemy turn. Acting first." {
		t.Fatalf("expected enemy turn banner in combat log, got %+v", model.combatLogLines)
	}
}

func TestCombatLogScrollControlsAndAutoFollow(t *testing.T) {
	model := newCombatTestModel(t, 2)
	model.combatLogLines = nil
	for i := 1; i <= 30; i++ {
		model.combatLogLines = append(model.combatLogLines, fmt.Sprintf("log line %02d", i))
	}
	model.combatLogAutoFollow = true
	model.clampCombatLogScroll()

	contentHeight := max(1, model.combatLayoutMetrics(model.panelWidth()).logBody-1)
	bottom := max(0, len(model.combatLogRows(model.panelWidth()))-contentHeight)
	if model.combatLogStart(model.panelWidth(), contentHeight) != bottom {
		t.Fatalf("expected combat log to start at bottom while auto-following")
	}

	if !model.handleCombatLogKey("ctrl+u") {
		t.Fatalf("expected ctrl+u to be handled by combat log")
	}
	if model.combatLogAutoFollow {
		t.Fatalf("expected ctrl+u to disable auto-follow")
	}
	if model.combatLogScroll >= bottom {
		t.Fatalf("expected ctrl+u to move above the bottom, got %d", model.combatLogScroll)
	}

	model.handleCombatLogKey("g")
	if !model.combatLogGPrefix {
		t.Fatalf("expected first g to arm the gg prefix")
	}
	model.handleCombatLogKey("g")
	if model.combatLogGPrefix {
		t.Fatalf("expected gg to consume the prefix")
	}
	if model.combatLogScroll != 0 {
		t.Fatalf("expected gg to jump to top, got %d", model.combatLogScroll)
	}

	model.handleCombatLogKey("G")
	if !model.combatLogAutoFollow {
		t.Fatalf("expected G to restore auto-follow")
	}
	if model.combatLogStart(model.panelWidth(), contentHeight) != bottom {
		t.Fatalf("expected G to jump back to bottom")
	}

	model.appendCombatLog([]string{"new tail line"})
	rows := model.combatLogRows(model.panelWidth())
	newBottom := max(0, len(rows)-contentHeight)
	if model.combatLogStart(model.panelWidth(), contentHeight) != newBottom {
		t.Fatalf("expected new log lines to keep the viewport pinned to bottom")
	}
}

func TestCombatLogPreservedAcrossInspectBack(t *testing.T) {
	model := newCombatTestModel(t, 2)
	model.advancePlayback()
	before := append([]string(nil), model.combatLogLines...)

	model.applyChoice("inspect")
	if model.scene.Kind != "inspect" {
		t.Fatalf("expected inspect scene, got %q", model.scene.Kind)
	}
	if got := strings.Join(model.combatLogLines, "\n"); got != strings.Join(before, "\n") {
		t.Fatalf("expected inspect to preserve combat log, got %+v want %+v", model.combatLogLines, before)
	}

	model.applyChoice("inspect:back")
	if model.scene.Combat == nil {
		t.Fatalf("expected return to combat scene after backing out of inspect")
	}
	if got := strings.Join(model.combatLogLines, "\n"); got != strings.Join(before, "\n") {
		t.Fatalf("expected inspect back to preserve combat log, got %+v want %+v", model.combatLogLines, before)
	}
}

func TestCombatRenderFitsViewport(t *testing.T) {
	model := newCombatTestModel(t, 2)
	model.width = 100
	model.height = 36
	model.syncBarWidth()

	view := model.render()
	if got := len(strings.Split(view, "\n")); got > model.height {
		t.Fatalf("expected combat render to fit within viewport height %d, got %d lines\n%s", model.height, got, view)
	}
}

func TestNodeGlyphUsesCurrentAndTypeMarkers(t *testing.T) {
	tests := []struct {
		name     string
		node     app.NodeMapNode
		focused  string
		expected string
	}{
		{
			name:     "focused overrides type",
			node:     app.NodeMapNode{ID: "n1", Type: "Standard"},
			focused:  "n1",
			expected: "[>]",
		},
		{
			name:     "focused overrides current marker",
			node:     app.NodeMapNode{ID: "start", Type: "Start", Current: true},
			focused:  "start",
			expected: "[>]",
		},
		{
			name:     "current node keeps current marker",
			node:     app.NodeMapNode{ID: "start", Type: "Start", Current: true},
			expected: "[@]",
		},
		{
			name:     "standard node shows s",
			node:     app.NodeMapNode{ID: "n1", Type: "Standard", Visible: true},
			expected: "[s]",
		},
		{
			name:     "corrupted node shows c",
			node:     app.NodeMapNode{ID: "n2", Type: "Corrupted", Visible: true},
			expected: "[c]",
		},
		{
			name:     "boss node shows b",
			node:     app.NodeMapNode{ID: "boss", Type: "Boss", Visible: true},
			expected: "[b]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := nodeGlyph(tc.node, tc.focused); got != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

func TestRenderNodeMapHighlightsFocusedMarker(t *testing.T) {
	lines := renderNodeMap(app.NodeMapView{
		Nodes: []app.NodeMapNode{
			{ID: "start", Type: "Start", Depth: 0, Lane: 1, Current: true},
			{ID: "n1", Type: "Standard", Depth: 1, Lane: 1, Visible: true},
		},
		Edges: []app.NodeMapEdge{{From: "start", To: "n1"}},
	}, "n1")

	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, colorize(ansiBold+ansiGreen, "[>]")) {
		t.Fatalf("expected focused node marker to be colorized, got:\n%s", rendered)
	}
}
