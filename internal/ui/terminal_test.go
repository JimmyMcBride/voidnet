package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"voidnet/internal/app"
	"voidnet/internal/audio"
	"voidnet/internal/audio/music"
	"voidnet/internal/content"
	"voidnet/internal/meta"
)

type fakeAudioRuntime struct {
	played    []audio.Event
	uiPlayed  []audio.Event
	loops     []music.LoopID
	reactive  []music.ReactiveState
	muted     bool
	available bool
}

func (f *fakeAudioRuntime) Play(event audio.Event, seed int64) {
	if f.muted || event == "" {
		return
	}
	f.played = append(f.played, event)
}

func (f *fakeAudioRuntime) PlayUI(event audio.Event, seed int64) {
	if f.muted || event == "" {
		return
	}
	f.uiPlayed = append(f.uiPlayed, event)
}

func (f *fakeAudioRuntime) StartMusicLoop(loop music.LoopID) error {
	f.loops = append(f.loops, loop)
	return nil
}

func (f *fakeAudioRuntime) StopMusicLoop() {}

func (f *fakeAudioRuntime) SetMusicReactiveState(state music.ReactiveState) error {
	f.reactive = append(f.reactive, state)
	return nil
}

func (f *fakeAudioRuntime) SetMuted(muted bool) {
	f.muted = muted
}

func (f *fakeAudioRuntime) Muted() bool {
	return f.muted
}

func (f *fakeAudioRuntime) Available() bool {
	return f.available
}

func (f *fakeAudioRuntime) Close() error {
	return nil
}

func newCombatTestModel(t *testing.T, seed int64) model {
	return newCombatTestModelWithAudio(t, seed, audio.NewNoopRuntime())
}

func newCombatTestModelWithAudio(t *testing.T, seed int64, audioRuntime audio.Runtime) model {
	t.Helper()

	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, seed, true)
	model := newModelWithAudio(session, audioRuntime)
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

func advancePlaybackUntil(t *testing.T, m *model, max int, cond func(*model) bool) {
	t.Helper()
	for i := 0; i < max; i++ {
		if cond(m) {
			return
		}
		_ = m.advancePlayback()
	}
	t.Fatalf("condition not reached after %d playback steps", max)
}

func containsAudioEvent(events []audio.Event, target audio.Event) bool {
	for _, event := range events {
		if event == target {
			return true
		}
	}
	return false
}

func maintenanceTestScene() app.Scene {
	return app.Scene{
		Kind:  "maintenance",
		Title: "Maintenance",
		Lines: []string{
			"Choose repair or fortify, then select any daemon in your roster as the target.",
			"Active daemon: Firewall 55/55 HP | SPD 9 | STB 13 | Encrypted",
			"Maintenance charge: 1/1 ready.",
			"",
			"Roster telemetry:",
			"* Firewall 55/55 HP | SPD 9 | STB 13 | Encrypted",
			"  Scheduler 34/42 HP | SPD 15 | STB 9 | Persistent",
		},
		Choices: []app.Choice{
			{ID: "maintenance:repair", Label: "Repair Daemon", Enabled: true, Details: &app.ChoiceDetails{
				Title:   "Repair Daemon",
				Preview: "Pick any daemon, then restore 30% max Health, minimum 10, and cleanse one negative status.",
				Lines:   []string{"Repair preview"},
			}},
			{ID: "maintenance:fortify", Label: "Fortify Daemon", Enabled: true, Details: &app.ChoiceDetails{
				Title:   "Fortify Daemon",
				Preview: "Pick any daemon, then restore 10% max Health, minimum 4, and grant Stabilized for the next fight.",
				Lines:   []string{"Fortify preview"},
			}},
			{ID: "view:nodes", Label: "Back to Network Map", Enabled: true},
			{ID: "quit", Label: "Quit", Enabled: true},
		},
	}
}

func rewardTestScene() app.Scene {
	return app.Scene{
		Kind:  "reward",
		Title: "signal.root complete",
		Lines: []string{
			"Firewall gained +3 Max Health, +1 Speed, +1 Stability.",
			"Firewall restored 12 Health after the encounter.",
			"MemoryLeaker joined the roster.",
			"Maintenance charge ready (1/1).",
			"Unlocked modifier: Intensify.",
			"Trace residue folded back into the mesh.",
		},
		Choices: []app.Choice{
			{ID: "continue", Label: "Continue", Enabled: true},
			{ID: "quit", Label: "Quit", Enabled: true},
		},
	}
}

func newNodeSelectTestModel(t *testing.T) model {
	t.Helper()

	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)
	m := newModel(session)
	m.width = 100
	m.height = 36
	m.applyChoice("starter:firewall")
	if m.scene.Kind != "node_select" {
		t.Fatalf("expected node_select scene, got %q", m.scene.Kind)
	}
	return m
}

func newMergeResultTestModel(t *testing.T, audioRuntime audio.Runtime) model {
	t.Helper()

	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)
	m := newModelWithAudio(session, audioRuntime)
	m.width = 100
	m.height = 40
	m.scene = app.Scene{
		Kind:  "merge_result",
		Title: "Merge Complete",
		Lines: []string{
			"Base seed: Firewall",
			"Fork consumed: Scheduler",
			"Result daemon: Firewall +1",
		},
		Choices: []app.Choice{
			{ID: "continue", Label: "Back to Node Map", Enabled: true},
			{ID: "quit", Label: "Quit", Enabled: true},
		},
		Merge: &app.MergeView{
			ForkName:   "Scheduler",
			ResultName: "Firewall +1",
			Trait:      "Encrypted",
			FocusStat:  "Speed",
			StatBefore: 8,
			StatAfter:  10,
			Healed:     13,
			Ability:    "Delay + Intensify",
			Daemon: app.CombatantView{
				Label:            "Firewall +1",
				Trait:            "Encrypted",
				IntegrityCurrent: 43,
				IntegrityMax:     50,
				Statuses:         []string{"None"},
			},
		},
	}
	_ = m.beginMergeCutscene(m.scene)
	return m
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

func TestPlaybackBeatTimingUsesModerateProfile(t *testing.T) {
	prelude := buildPreludeBeats([]app.Event{
		{Message: "Entered signal.root.", Cue: audio.EventScan},
		{Message: "Encountered NullPointer.", Cue: audio.EventDaemonAppears},
	})
	if len(prelude) != 2 {
		t.Fatalf("expected 2 prelude beats, got %d", len(prelude))
	}
	if prelude[0].startDelay != playbackIntroDelay || prelude[0].holdDelay != playbackIntroDelay || prelude[0].phase != "LINK ESTABLISHED" {
		t.Fatalf("expected intro beats to use the slower entry timing and phase, got %+v", prelude[0])
	}
	if prelude[0].cue != audio.EventScan || prelude[1].cue != audio.EventDaemonAppears {
		t.Fatalf("expected prelude cues to preserve event routing, got %+v", prelude)
	}
	if !prelude[1].sequenceEnd {
		t.Fatalf("expected final prelude line to end the sequence, got %+v", prelude[1])
	}

	beats := buildActionBeats(actorEnemy, app.Scene{Combat: &app.CombatView{Round: 1}}, []app.Event{
		{Message: "Enemy NullPointer used Spike + Single."},
		{Message: "Success chance 66% (base 90, target resistance -10, stability diff -4). Roll 78."},
		{Message: "Your Firewall took 11 damage.", Cue: audio.EventHackSuccess},
	})
	if len(beats) != 4 {
		t.Fatalf("expected 4 action beats, got %d", len(beats))
	}
	if beats[0].startDelay != playbackTurnDelay || beats[0].holdDelay != playbackTurnDelay || beats[0].phase != "HOSTILE EXECUTION" {
		t.Fatalf("expected enemy turn beat to use slower turn timing, got %+v", beats[0])
	}
	if beats[0].cue != audio.EventAlert {
		t.Fatalf("expected enemy turn banner to trigger alert cue, got %+v", beats[0])
	}
	if beats[1].startDelay != playbackActionDelay || beats[1].holdDelay != playbackActionDelay || beats[1].phase != "ABILITY PRIMED" {
		t.Fatalf("expected action beat timing, got %+v", beats[1])
	}
	if beats[2].startDelay != playbackRollDelay || beats[2].holdDelay != playbackRollDelay || beats[2].phase != "RESOLUTION CHECK" {
		t.Fatalf("expected roll beat timing, got %+v", beats[2])
	}
	if beats[3].startDelay != playbackImpactDelay || beats[3].holdDelay != playbackSequenceDelay || beats[3].phase != "PAYLOAD LANDED" || !beats[3].applyScene {
		t.Fatalf("expected impact beat timing and scene application, got %+v", beats[3])
	}
	if beats[3].cue != audio.EventHackSuccess {
		t.Fatalf("expected impact beat to carry dominant cue, got %+v", beats[3])
	}
	if !beats[3].sequenceEnd {
		t.Fatalf("expected impact beat to end the action sequence, got %+v", beats[3])
	}
}

func TestCombatPlaybackDefersEnemyFirstDamageUntilImpactBeat(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 2, true)
	m := newModel(session)

	m.applyChoice("starter:firewall")
	m.applyChoice("node:n1")

	if m.scene.Combat == nil {
		t.Fatalf("expected combat scene after node entry")
	}
	if m.playback == nil {
		t.Fatalf("expected playback to start for combat entry")
	}
	if m.scene.Combat.Player.IntegrityCurrent != m.scene.Combat.Player.IntegrityMax {
		t.Fatalf("expected player HP to remain full before playback impact, got %d/%d", m.scene.Combat.Player.IntegrityCurrent, m.scene.Combat.Player.IntegrityMax)
	}

	_ = m.fastForwardPlayback()
	if m.playback == nil || m.playbackActor != actorEnemy || m.playbackPhase != "HOSTILE EXECUTION" {
		t.Fatalf("expected enemy sequence to start after entry fast-forward, got playback=%+v actor=%q phase=%q", m.playback, m.playbackActor, m.playbackPhase)
	}
	if m.scene.Combat.Player.IntegrityCurrent != m.scene.Combat.Player.IntegrityMax {
		t.Fatalf("expected no damage before enemy impact, got %d/%d", m.scene.Combat.Player.IntegrityCurrent, m.scene.Combat.Player.IntegrityMax)
	}

	advancePlaybackUntil(t, &m, 256, func(m *model) bool {
		current := currentPlaybackBeat(m.playback)
		return current != nil &&
			strings.HasPrefix(current.line, "Your Firewall took ") &&
			m.playback.stage == playbackStageStart
	})
	if m.scene.Combat.Player.IntegrityCurrent != m.scene.Combat.Player.IntegrityMax {
		t.Fatalf("expected no damage before impact beat, got %d/%d", m.scene.Combat.Player.IntegrityCurrent, m.scene.Combat.Player.IntegrityMax)
	}

	_ = m.advancePlayback()
	if m.scene.Combat.Player.IntegrityCurrent != 41 {
		t.Fatalf("expected enemy impact beat to land damage, got %d", m.scene.Combat.Player.IntegrityCurrent)
	}
}

func TestFastForwardPlaybackAppliesResolvedEnemySequence(t *testing.T) {
	model := newCombatTestModel(t, 2)
	_ = model.fastForwardPlayback()

	if model.playback == nil {
		t.Fatalf("expected enemy playback to be active before fast-forward")
	}

	_ = model.fastForwardPlayback()

	if model.playback != nil {
		t.Fatalf("expected fast-forward to finish the current sequence")
	}
	if model.scene.Combat == nil {
		t.Fatalf("expected combat to remain active after enemy opener")
	}
	if model.scene.Combat.Player.IntegrityCurrent != 41 {
		t.Fatalf("expected fast-forward to apply resolved enemy damage, got %d", model.scene.Combat.Player.IntegrityCurrent)
	}
	if !model.scene.Combat.PlayerTurn {
		t.Fatalf("expected control to return to the player after fast-forwarded enemy turn")
	}
}

func TestCombatEncounterShowsPlaybackPhaseSignal(t *testing.T) {
	model := newCombatTestModel(t, 2)

	lines := model.renderCombatEncounterLines(model.panelWidth(), true)
	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, colorize(ansiBold+ansiCyan, "LINK ESTABLISHED")) {
		t.Fatalf("expected entry playback phase signal, got:\n%s", rendered)
	}

	_ = model.fastForwardPlayback()
	lines = model.renderCombatEncounterLines(model.panelWidth(), true)
	rendered = strings.Join(lines, "\n")
	if !strings.Contains(rendered, colorize(ansiBold+ansiRed, "HOSTILE EXECUTION")) {
		t.Fatalf("expected enemy turn phase signal, got:\n%s", rendered)
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

	if len(model.combatLogLines) != 0 {
		t.Fatalf("expected combat log to start empty before typed playback commits lines, got %+v", model.combatLogLines)
	}

	model.advancePlayback()
	model.advancePlayback()
	model.advancePlayback()
	if len(model.combatLogLines) != 0 {
		t.Fatalf("expected current typed line to remain transient until finished, got %+v", model.combatLogLines)
	}
	_ = model.fastForwardPlayback()
	if len(model.combatLogLines) < 4 {
		t.Fatalf("expected fast-forwarded entry sequence to commit prelude lines and spacer, got %+v", model.combatLogLines)
	}
	if !strings.HasPrefix(model.combatLogLines[0], "Entered ") || !strings.HasPrefix(model.combatLogLines[1], "Encountered ") || !strings.Contains(model.combatLogLines[2], "gained Stabilized") || model.combatLogLines[3] != "" {
		t.Fatalf("expected committed prelude lines with spacer, got %+v", model.combatLogLines)
	}

	_ = model.fastForwardPlayback()
	if len(model.combatLogLines) < 8 {
		t.Fatalf("expected combat log to accumulate revealed beats, got %+v", model.combatLogLines)
	}
	if model.combatLogLines[4] != "Enemy turn. Acting first." {
		t.Fatalf("expected enemy turn banner in combat log, got %+v", model.combatLogLines)
	}
	if model.combatLogLines[len(model.combatLogLines)-1] != "" {
		t.Fatalf("expected action sequence to end with a blank spacer row, got %+v", model.combatLogLines)
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

	gain := formatCombatLogLine("Firewall gained +5 Max Health, +0 Speed, +1 Stability.")
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

func TestCombatLogCardHeightStaysFixedAsHistoryGrows(t *testing.T) {
	model := newCombatTestModel(t, 2)
	model.width = 100
	model.height = 36
	model.syncBarWidth()

	initial := model.combatLayoutMetrics(model.panelWidth()).logBody
	model.combatLogLines = []string{"Entered signal.root."}
	short := model.combatLayoutMetrics(model.panelWidth()).logBody
	model.combatLogLines = []string{}
	for i := 0; i < 40; i++ {
		model.combatLogLines = append(model.combatLogLines, fmt.Sprintf("log line %02d", i))
	}
	long := model.combatLayoutMetrics(model.panelWidth()).logBody

	if initial != short || short != long {
		t.Fatalf("expected combat log card height to stay fixed, got initial=%d short=%d long=%d", initial, short, long)
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

func TestMaintenanceMenuShowsSelectedChoicePreview(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)

	m := newModel(session)
	m.width = 100
	m.height = 36
	m.scene = maintenanceTestScene()

	view := m.render()
	if !strings.Contains(view, "Preview:") {
		t.Fatalf("expected maintenance command deck to show a preview block, got:\n%s", view)
	}
	if !strings.Contains(view, "Pick any daemon, then restore 30% max Health, minimum 10, and cleanse one negative status.") {
		t.Fatalf("expected maintenance preview text, got:\n%s", view)
	}
	if !strings.Contains(view, "Press i for full command detail.") {
		t.Fatalf("expected maintenance detail hint, got:\n%s", view)
	}
	if !strings.Contains(view, ":: repair // fortify // bank ::") {
		t.Fatalf("expected maintenance summary ornament, got:\n%s", view)
	}
	if !strings.Contains(view, ":: service note ::") {
		t.Fatalf("expected maintenance preview ornament, got:\n%s", view)
	}
	if !strings.Contains(view, "Roster telemetry:") || !strings.Contains(view, "55/55 HP") || !strings.Contains(view, "34/42 HP") {
		t.Fatalf("expected maintenance roster summary, got:\n%s", view)
	}
}

func TestRewardSceneUsesBeautifiedPayloadStyling(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)

	m := newModel(session)
	m.width = 100
	m.height = 36
	m.scene = rewardTestScene()

	view := m.render()
	if !strings.Contains(view, ":: uplink recovered - node cache unpacked ::") {
		t.Fatalf("expected reward ornament, got:\n%s", view)
	}
	if !strings.Contains(view, "// new access //") {
		t.Fatalf("expected unlock section accent, got:\n%s", view)
	}
	if !strings.Contains(view, "/* residual trace */") {
		t.Fatalf("expected lore section accent, got:\n%s", view)
	}
	if !strings.Contains(view, "Maintenance charge ") || !strings.Contains(view, "ready ") || !strings.Contains(view, "(1/1)") {
		t.Fatalf("expected maintenance charge text to remain visible, got:\n%s", view)
	}
	if !strings.Contains(view, "MemoryLeaker") || !strings.Contains(view, "joined the roster.") {
		t.Fatalf("expected roster payload text to remain visible, got:\n%s", view)
	}
}

func TestSpaceNoLongerSelectsMenusAndEnterStillDoes(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)
	m := newModel(session)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	afterSpace := updated.(model)
	if afterSpace.scene.Kind != "starter_select" {
		t.Fatalf("expected space to stop acting as menu confirm, got scene %q", afterSpace.scene.Kind)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	afterEnter := updated.(model)
	if afterEnter.scene.Kind != "node_select" {
		t.Fatalf("expected enter to remain the menu confirm key, got scene %q", afterEnter.scene.Kind)
	}
}

func TestSpaceAcceleratesCombatPlaybackOnly(t *testing.T) {
	m := newCombatTestModel(t, 2)
	_ = m.fastForwardPlayback()

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	next := updated.(model)
	if next.playback == nil {
		t.Fatalf("expected space to keep the enemy sequence active while accelerating it")
	}
	if !next.playback.fast {
		t.Fatalf("expected space to enable fast playback for the current sequence")
	}
	if next.scene.Combat == nil || next.scene.Combat.Player.IntegrityCurrent != next.scene.Combat.Player.IntegrityMax {
		t.Fatalf("expected space acceleration to preserve readability before impact, got %+v", next.scene.Combat)
	}
}

func TestTogglePlaybackFastForwardWithF(t *testing.T) {
	m := newCombatTestModel(t, 2)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "f"})
	next := updated.(model)
	if !next.playbackAutoFast {
		t.Fatalf("expected f to enable playback auto fast-forward")
	}
	if next.playback == nil || !next.playback.fast {
		t.Fatalf("expected current playback sequence to inherit auto fast-forward")
	}

	updated, _ = next.Update(tea.KeyPressMsg{Text: "f"})
	next = updated.(model)
	if next.playbackAutoFast {
		t.Fatalf("expected second f to disable playback auto fast-forward")
	}
	if next.playback != nil && next.playback.fast {
		t.Fatalf("expected current playback sequence to drop out of fast mode")
	}
}

func TestToggleAudioMuteState(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newModelWithAudio(session, audioRuntime)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "m"})
	next := updated.(model)
	if !next.audio.Muted() {
		t.Fatalf("expected m to mute runtime")
	}

	updated, _ = next.Update(tea.KeyPressMsg{Text: "m"})
	next = updated.(model)
	if next.audio.Muted() {
		t.Fatalf("expected second m to unmute runtime")
	}
}

func TestNodeEntryPlaybackUsesScanAndEncounterCues(t *testing.T) {
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newCombatTestModelWithAudio(t, 2, audioRuntime)
	advancePlaybackUntil(t, &m, 512, func(m *model) bool {
		return containsAudioEvent(audioRuntime.played, audio.EventScan) &&
			containsAudioEvent(audioRuntime.played, audio.EventDaemonAppears) &&
			containsAudioEvent(audioRuntime.played, audio.EventPatchRestore)
	})

	if len(audioRuntime.uiPlayed) == 0 || audioRuntime.uiPlayed[0] != audio.EventLogLine {
		t.Fatalf("expected combat entry playback to emit line-open UI cue, got %+v", audioRuntime.uiPlayed)
	}
	if len(audioRuntime.played) < 3 {
		t.Fatalf("expected combat entry playback to emit scan and encounter cues, got %+v", audioRuntime.played)
	}
	if !containsAudioEvent(audioRuntime.played, audio.EventScan) || !containsAudioEvent(audioRuntime.played, audio.EventDaemonAppears) || !containsAudioEvent(audioRuntime.played, audio.EventPatchRestore) {
		t.Fatalf("expected node entry to include scan, reveal, and starter fortify cues, got %+v", audioRuntime.played)
	}
}

func TestCombatPlaybackUsesTurnAndImpactCues(t *testing.T) {
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newCombatTestModelWithAudio(t, 2, audioRuntime)

	for m.playback != nil {
		_ = m.advancePlayback()
	}

	if len(audioRuntime.played) < 4 {
		t.Fatalf("expected combat playback cues, got %+v", audioRuntime.played)
	}
	if !containsAudioEvent(audioRuntime.uiPlayed, audio.EventLogLine) || !containsAudioEvent(audioRuntime.uiPlayed, audio.EventTypingPulse) {
		t.Fatalf("expected combat playback to emit UI typing cues, got %+v", audioRuntime.uiPlayed)
	}
	if !containsAudioEvent(audioRuntime.played, audio.EventDaemonAppears) {
		t.Fatalf("expected encounter reveal cue during prelude, got %+v", audioRuntime.played)
	}
	if !containsAudioEvent(audioRuntime.played, audio.EventAlert) {
		t.Fatalf("expected enemy turn banner alert cue, got %+v", audioRuntime.played)
	}
	if audioRuntime.played[len(audioRuntime.played)-1] != audio.EventHackSuccess {
		t.Fatalf("expected impact cue to land on damage result, got %+v", audioRuntime.played)
	}
}

func TestFastForwardPlaybackStillPlaysDominantRemainingCue(t *testing.T) {
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newCombatTestModelWithAudio(t, 2, audioRuntime)
	_ = m.fastForwardPlayback()

	before := len(audioRuntime.played)
	_ = m.fastForwardPlayback()

	if len(audioRuntime.played) != before+1 {
		t.Fatalf("expected fast-forward to emit one dominant cue, got %+v", audioRuntime.played)
	}
	if audioRuntime.played[len(audioRuntime.played)-1] != audio.EventHackSuccess {
		t.Fatalf("expected fast-forward to preserve impact cue, got %+v", audioRuntime.played)
	}
}

func TestRewardAndGameOverSceneEntryCues(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newModelWithAudio(session, audioRuntime)

	m.handleSceneTransition(app.Scene{}, app.Scene{Kind: "reward", Lines: []string{"Unlocked starter: Scheduler."}}, nil, "continue")
	if got := audioRuntime.played[len(audioRuntime.played)-1]; got != audio.EventUnlock {
		t.Fatalf("expected unlock reward cue, got %+v", audioRuntime.played)
	}

	m.handleSceneTransition(app.Scene{}, app.Scene{Kind: "reward", Lines: []string{"Node complete."}}, nil, "continue")
	if got := audioRuntime.played[len(audioRuntime.played)-1]; got != audio.EventLevelClear {
		t.Fatalf("expected level clear cue, got %+v", audioRuntime.played)
	}

	m.handleSceneTransition(app.Scene{}, app.Scene{Kind: "game_over", Lines: []string{"Run won."}}, nil, "continue")
	if got := audioRuntime.played[len(audioRuntime.played)-1]; got != audio.EventRunVictory {
		t.Fatalf("expected run victory cue, got %+v", audioRuntime.played)
	}

	before := len(audioRuntime.played)
	m.handleSceneTransition(app.Scene{}, app.Scene{Kind: "game_over", Lines: []string{"Run failed."}}, nil, "continue")
	if len(audioRuntime.played) != before {
		t.Fatalf("expected run defeat to avoid a duplicate scene-entry cue, got %+v", audioRuntime.played)
	}
}

func TestSwitchMusicUpdatesReactiveCombatState(t *testing.T) {
	audioRuntime := &fakeAudioRuntime{available: true}
	m := model{audio: audioRuntime}
	scene := app.Scene{
		Kind: "combat",
		Combat: &app.CombatView{
			NodeType:   "Boss",
			PlayerTurn: true,
			Player: app.CombatantView{
				IntegrityCurrent: 18,
				IntegrityMax:     40,
				Statuses:         []string{"Corrupted(2)"},
			},
			Enemy: app.CombatantView{
				IntegrityCurrent: 20,
				IntegrityMax:     55,
				Statuses:         []string{"None"},
			},
		},
	}

	m.switchMusic(scene)

	if len(audioRuntime.loops) != 1 || audioRuntime.loops[0] != music.LoopBattle {
		t.Fatalf("expected battle loop start, got %+v", audioRuntime.loops)
	}
	if len(audioRuntime.reactive) != 1 {
		t.Fatalf("expected reactive music update, got %+v", audioRuntime.reactive)
	}
	got := audioRuntime.reactive[0]
	if got.BattleTheme != music.BattleThemeBoss || got.Intensity != 1 {
		t.Fatalf("unexpected reactive combat state %+v", got)
	}
}

func TestMenuNavigationRemainsSilent(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newModelWithAudio(session, audioRuntime)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "j"})
	next := updated.(model)
	if len(audioRuntime.played) != 0 {
		t.Fatalf("expected menu navigation to stay silent, got %+v", audioRuntime.played)
	}
	if next.selectedIndex == m.selectedIndex {
		t.Fatalf("expected navigation to still move the selection")
	}
}

func TestControlsHintReflectsSpacePlaybackAndEnterSelect(t *testing.T) {
	reg, err := content.Load()
	if err != nil {
		t.Fatalf("content load failed: %v", err)
	}

	state := meta.DefaultState()
	session := app.NewSession(reg, meta.NewStore(""), state, 12345, true)
	m := newModelWithAudio(session, &fakeAudioRuntime{available: true})
	if hint := m.controlsHint(); hint != "hint: press c for command sheet" {
		t.Fatalf("expected compact non-combat hint, got %q", hint)
	}

	combatModel := newCombatTestModelWithAudio(t, 2, &fakeAudioRuntime{available: true})
	if hint := combatModel.controlsHint(); !strings.Contains(hint, "enter/space accelerate") || !strings.Contains(hint, "f toggle fast-forward") || !strings.Contains(hint, "press c for command sheet") {
		t.Fatalf("expected playback hint to advertise space fast-forward, got %q", hint)
	}

	settleCombatPlayback(&combatModel)
	if hint := combatModel.controlsHint(); hint != "hint: press c for command sheet" {
		t.Fatalf("expected compact idle combat hint, got %q", hint)
	}
}

func TestCombatChoiceModalOpensAndCloses(t *testing.T) {
	m := newCombatTestModel(t, 2)
	settleCombatPlayback(&m)

	m.openChoiceModal()
	if m.activeModal == nil {
		t.Fatalf("expected choice modal to open for selected combat ability")
	}
	if m.activeModal.title != "Spike + Single" {
		t.Fatalf("unexpected modal title %q", m.activeModal.title)
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

func TestNodeSelectHidesMaintenanceAndRotateChoices(t *testing.T) {
	m := newNodeSelectTestModel(t)

	view := m.render()
	if strings.Contains(view, "Open Maintenance Console") || strings.Contains(view, "Rotate Lead") {
		t.Fatalf("expected node-select command deck to hide maintenance shortcuts, got:\n%s", view)
	}
	if !strings.Contains(view, "Merge Daemons") {
		t.Fatalf("expected node-select command deck to show merge command, got:\n%s", view)
	}
}

func TestNodeSelectShortcutOpensMaintenance(t *testing.T) {
	m := newNodeSelectTestModel(t)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "x"})
	next := updated.(model)
	if next.scene.Kind != "maintenance" {
		t.Fatalf("expected x to open maintenance, got %q", next.scene.Kind)
	}
}

func TestNodeSelectRotateShortcutWithoutReserveShowsError(t *testing.T) {
	m := newNodeSelectTestModel(t)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "r"})
	next := updated.(model)
	if next.scene.Kind != "node_select" {
		t.Fatalf("expected rotate failure to stay on node_select, got %q", next.scene.Kind)
	}
	if !strings.Contains(next.lastError, "no reserve daemon") {
		t.Fatalf("expected rotate shortcut error, got %q", next.lastError)
	}
}

func TestNodeSelectCommandSheetShowsShortcutCommands(t *testing.T) {
	m := newNodeSelectTestModel(t)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "c"})
	next := updated.(model)
	if next.activeModal == nil || next.activeModal.kind != modalKindCommands {
		t.Fatalf("expected command sheet to open")
	}
	rendered := next.render()
	if !strings.Contains(rendered, "x") || !strings.Contains(rendered, "open maintenance console") {
		t.Fatalf("expected maintenance shortcut in command sheet, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "r") || !strings.Contains(rendered, "rotate lead") {
		t.Fatalf("expected rotate shortcut in command sheet, got:\n%s", rendered)
	}
}

func TestCombatCommandSheetShowsFastForwardToggle(t *testing.T) {
	m := newCombatTestModel(t, 2)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "c"})
	next := updated.(model)
	if next.activeModal == nil || next.activeModal.kind != modalKindCommands {
		t.Fatalf("expected combat command sheet to open")
	}
	rendered := next.render()
	if !strings.Contains(rendered, "toggle playback fast-forward") || !strings.Contains(rendered, "(off)") {
		t.Fatalf("expected combat command sheet to show fast-forward toggle state, got:\n%s", rendered)
	}
}

func TestMergeResultStartsAnimatedCutsceneAndRevealsDaemonCard(t *testing.T) {
	audioRuntime := &fakeAudioRuntime{available: true}
	m := newMergeResultTestModel(t, audioRuntime)

	if !m.mergeCutsceneActive() {
		t.Fatalf("expected merge cutscene to start active")
	}
	rendered := m.render()
	if !strings.Contains(rendered, "FORK SIGNAL DETECTED") {
		t.Fatalf("expected initial merge phase in render, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Sequence running. Enter or Space fast-forward.") {
		t.Fatalf("expected locked merge command deck hint, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "/* merged daemon */") {
		t.Fatalf("expected merged daemon block to stay hidden until cutscene completes, got:\n%s", rendered)
	}

	updated, _ := m.Update(tea.KeyPressMsg{Text: "enter"})
	next := updated.(model)
	if next.mergeCutscene == nil || !next.mergeCutscene.complete {
		t.Fatalf("expected enter to fast-forward merge cutscene")
	}
	rendered = next.render()
	if !strings.Contains(rendered, "/* merged daemon */") || !strings.Contains(rendered, "Firewall +1") {
		t.Fatalf("expected merged daemon block after reveal, got:\n%s", rendered)
	}
	if !containsAudioEvent(audioRuntime.played, audio.EventUnlock) {
		t.Fatalf("expected merge reveal to play success audio, got %+v", audioRuntime.played)
	}
}

func TestCommandsModalOpensAndCloses(t *testing.T) {
	m := newNodeSelectTestModel(t)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "c"})
	next := updated.(model)
	if next.activeModal == nil || next.activeModal.kind != modalKindCommands {
		t.Fatalf("expected c to open command sheet")
	}
	rendered := next.render()
	if !strings.Contains(rendered, "Command Sheet") || !strings.Contains(rendered, "Available now") {
		t.Fatalf("expected command sheet modal content, got:\n%s", rendered)
	}

	updated, _ = next.Update(tea.KeyPressMsg{Text: "c"})
	closed := updated.(model)
	if closed.activeModal != nil {
		t.Fatalf("expected c to close command sheet")
	}
}

func TestCommandsModalCanReplaceAndBeReplacedByDetailModal(t *testing.T) {
	m := newCombatTestModel(t, 2)
	settleCombatPlayback(&m)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "c"})
	withCommands := updated.(model)
	if withCommands.activeModal == nil || withCommands.activeModal.kind != modalKindCommands {
		t.Fatalf("expected command sheet to open")
	}

	updated, _ = withCommands.Update(tea.KeyPressMsg{Text: "i"})
	withDetail := updated.(model)
	if withDetail.activeModal == nil || withDetail.activeModal.kind != modalKindDetail {
		t.Fatalf("expected i to replace command sheet with detail modal")
	}

	updated, _ = withDetail.Update(tea.KeyPressMsg{Text: "c"})
	backToCommands := updated.(model)
	if backToCommands.activeModal == nil || backToCommands.activeModal.kind != modalKindCommands {
		t.Fatalf("expected c to replace detail modal with command sheet")
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
			"Your Firewall 41/47 HP | SPD 7 | STB 11 | Volatile",
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
