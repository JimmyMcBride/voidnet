package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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

func settleCombatPlayback(m *model) {
	for m.playback != nil {
		_ = m.fastForwardPlayback()
	}
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

func TestFormatCombatLogLineStylesTurnActionAndRoll(t *testing.T) {
	turn := formatCombatLogLine("Enemy turn. Acting first.")
	if !strings.Contains(turn, colorize(ansiBold+ansiYellow, "[TURN]")) {
		t.Fatalf("expected turn tag styling, got %q", turn)
	}
	if !strings.Contains(turn, colorize(ansiBold+ansiRed, "Enemy turn.")) {
		t.Fatalf("expected enemy turn emphasis, got %q", turn)
	}

	action := formatCombatLogLine("Enemy NullPointer used Spike + Single.")
	if !strings.Contains(action, colorize(ansiBold+ansiYellow, "[ACT ]")) {
		t.Fatalf("expected action tag styling, got %q", action)
	}
	if !strings.Contains(action, colorize(ansiBold+ansiRed, "Enemy NullPointer")) {
		t.Fatalf("expected actor emphasis in action line, got %q", action)
	}
	if !strings.Contains(action, colorize(ansiBold+ansiYellow, "Spike")) {
		t.Fatalf("expected effect emphasis in action line, got %q", action)
	}

	roll := formatCombatLogLine("Success chance 66% (base 82, modifier -10, status -10, stability diff -4, target resistance -10). Roll 78.")
	if !strings.Contains(roll, colorize(ansiDim+ansiYellow, "[ROLL]")) {
		t.Fatalf("expected roll tag styling, got %q", roll)
	}
	if !strings.Contains(roll, colorize(ansiBold+ansiYellow, "66%")) || !strings.Contains(roll, colorize(ansiBold+ansiYellow, "78")) {
		t.Fatalf("expected chance and roll emphasis, got %q", roll)
	}
	if !strings.Contains(roll, colorize(ansiCyan, "stability diff")) || !strings.Contains(roll, colorize(ansiCyan, "target resistance")) {
		t.Fatalf("expected stability-related terms to be highlighted, got %q", roll)
	}
}

func TestFormatCombatLogLineStylesStatusAndRewards(t *testing.T) {
	status := formatCombatLogLine("Your Firewall is now Corrupted.")
	if !strings.Contains(status, colorize(ansiBold+ansiCyan, "Your Firewall")) {
		t.Fatalf("expected player actor emphasis, got %q", status)
	}
	if !strings.Contains(status, colorize(ansiBold+ansiYellow, "Corrupted")) {
		t.Fatalf("expected negative status emphasis, got %q", status)
	}

	gain := formatCombatLogLine("Firewall gained +5 Integrity, +0 Speed, +1 Stability.")
	if !strings.Contains(gain, colorize(ansiBold+ansiGreen, "[GAIN]")) {
		t.Fatalf("expected gain tag styling, got %q", gain)
	}
	if !strings.Contains(gain, colorize(ansiBold+ansiGreen, "+5")) || !strings.Contains(gain, colorize(ansiBold+ansiGreen, "+1")) {
		t.Fatalf("expected highlighted stat gains, got %q", gain)
	}

	lore := formatCombatLogLine("[LOG_01] \"we built this system to be free...\"")
	if !strings.Contains(lore, colorize(ansiDim+ansiCyan, "[LOG ]")) {
		t.Fatalf("expected lore tag styling, got %q", lore)
	}
}

func TestFormatCombatLogLineFallsBackCleanly(t *testing.T) {
	line := "Unclassified combat note."
	formatted := formatCombatLogLine(line)
	if formatted != line {
		t.Fatalf("expected fallback lines to remain readable, got %q", formatted)
	}
}

func TestCombatLogRowsWrapStyledLinesByVisibleWidth(t *testing.T) {
	model := newCombatTestModel(t, 2)
	model.combatLogLines = []string{"Success chance 66% (base 82, modifier -10, status -10, stability diff -4, target resistance -10). Roll 78."}

	rows := model.combatLogRows(20)
	if len(rows) < 2 {
		t.Fatalf("expected styled log line to wrap in narrow viewport, got %+v", rows)
	}
	for _, row := range rows {
		if visibleWidth(row) > 20 {
			t.Fatalf("expected wrapped row width <= 20, got %d for %q", visibleWidth(row), row)
		}
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

func TestCombatMenuShowsSelectedChoicePreview(t *testing.T) {
	model := newCombatTestModel(t, 2)
	settleCombatPlayback(&model)

	lines := model.renderCombatMenuLines()
	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "Preview:") {
		t.Fatalf("expected combat menu preview block, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Damage | 90% base accuracy | 9 base power") {
		t.Fatalf("expected selected ability preview text, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Press i for full command detail.") {
		t.Fatalf("expected modal hint in combat menu, got:\n%s", rendered)
	}
}

func TestCombatChoiceModalOpensAndCloses(t *testing.T) {
	m := newCombatTestModel(t, 2)
	settleCombatPlayback(&m)

	m.openChoiceModal()
	if m.activeModal == nil {
		t.Fatalf("expected choice modal to open for selected combat ability")
	}
	if m.activeModal.Title != "Spike + Single" {
		t.Fatalf("unexpected modal title %q", m.activeModal.Title)
	}

	rendered := m.render()
	if !strings.Contains(rendered, "Close with i, Esc, or q.") {
		t.Fatalf("expected modal overlay close hint, got:\n%s", rendered)
	}

	updated, _ := m.Update(tea.KeyPressMsg{Text: "i"})
	next := updated.(model)
	if next.activeModal != nil {
		t.Fatalf("expected modal to close on i")
	}
}

func TestCombatChoiceModalStaysLockedDuringPlayback(t *testing.T) {
	model := newCombatTestModel(t, 2)
	model.playback = &playbackSequence{}

	model.openChoiceModal()

	if model.activeModal != nil {
		t.Fatalf("expected modal open to be blocked during playback")
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

func TestRenderLogoCentersBannerAndAddsSideAccents(t *testing.T) {
	scene := app.Scene{Title: "Select Node"}

	logo := renderLogo(scene, 74)
	lines := strings.Split(logo, "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 logo lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], ".::") || !strings.Contains(lines[0], "::.") {
		t.Fatalf("expected decorative accents on top logo line, got %q", lines[0])
	}
	if !strings.Contains(lines[2], "[[ ") || !strings.Contains(lines[2], " ]]") {
		t.Fatalf("expected mirrored midline accents, got %q", lines[2])
	}

	const banner = "_    __      _     __           __"
	leftPad := strings.Index(lines[0], banner)
	if leftPad < 10 {
		t.Fatalf("expected banner to be centered with visible left padding, got %q", lines[0])
	}
}

func TestDecorateLinesStylesTraceView(t *testing.T) {
	scene := app.Scene{
		Kind: "inspect",
		Lines: []string{
			"Your Firewall 41/47 INT | SPD 7 | STB 11 | Volatile",
			"Abilities: Spike + Unstable, Corrupt + Intensify",
			"Trait: Volatile - deals 6 damage on crash",
			"Active status: Corrupted(2): -10 accuracy",
			"",
			"Stat key:",
			"STB Stability: resists hostile effects and lowers isolation chance against this daemon.",
			"Corrupted: -10 accuracy",
		},
	}

	lines := decorateLines(scene)
	rendered := strings.Join(lines, "\n")

	if !strings.Contains(rendered, colorize(ansiBold+ansiCyan, "Your ")) {
		t.Fatalf("expected player trace summary emphasis, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, colorize(ansiCyan, "Abilities: ")) {
		t.Fatalf("expected ability label styling, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, colorize(ansiGreen, "Trait: ")) {
		t.Fatalf("expected trait label styling, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, colorize(ansiBold+ansiRed, "-10 accuracy")) {
		t.Fatalf("expected negative status mechanics to be highlighted, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, colorize(ansiDim+ansiCyan, "····················")) {
		t.Fatalf("expected decorative divider in trace view, got:\n%s", rendered)
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
