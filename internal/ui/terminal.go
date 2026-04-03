package ui

import (
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"time"

	progress "charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"

	"voidnet/internal/app"
	"voidnet/internal/audio"
	"voidnet/internal/audio/music"
)

const (
	defaultPanelWidth = 74
	minPanelWidth     = 60
	maxPanelWidth     = 96
)

const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiBold   = "\033[1m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
)

const (
	playbackIntroDelay  = 220 * time.Millisecond
	playbackTurnDelay   = 320 * time.Millisecond
	playbackActionDelay = 260 * time.Millisecond
	playbackRollDelay   = 220 * time.Millisecond
	playbackImpactDelay = 520 * time.Millisecond
)

type model struct {
	session             *app.Session
	audio               audio.Runtime
	audioCounter        uint64
	scene               app.Scene
	selectedIndex       int
	width               int
	height              int
	playerBar           progress.Model
	enemyBar            progress.Model
	lastError           string
	playback            *playbackSequence
	playbackLines       []string
	playbackActor       string
	playbackImpact      bool
	playbackPhase       string
	playbackPhaseStyle  string
	combatLogLines      []string
	combatLogScroll     int
	combatLogAutoFollow bool
	combatLogGPrefix    bool
	activeModal         *app.ChoiceDetails
}

type playbackSequence struct {
	resolvedScene app.Scene
	beats         []playbackBeat
	index         int
}

type playbackBeat struct {
	lines      []string
	delay      time.Duration
	actor      string
	impact     bool
	applyScene bool
	phase      string
	phaseStyle string
	cue        audio.Event
}

type playbackTickMsg struct{}

func Run(session *app.Session) (err error) {
	bootLoop := music.LoopBoot
	audioRuntime, err := audio.NewRuntime(audio.Options{AutoStart: &bootLoop})
	if err != nil {
		return err
	}
	if audioRuntime != nil {
		defer func() {
			err = errors.Join(err, audioRuntime.Close())
		}()
	}

	m := newModelWithAudio(session, audioRuntime)
	m.playCue(audio.EventSystemBoot)
	program := tea.NewProgram(
		m,
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err = program.Run()
	return err
}

func newModel(session *app.Session) model {
	return newModelWithAudio(session, audio.NewNoopRuntime())
}

func newModelWithAudio(session *app.Session, audioRuntime audio.Runtime) model {
	playerBar := progress.New(
		progress.WithFillCharacters('=', '-'),
		progress.WithSpringOptions(18, 0.92),
		progress.WithoutPercentage(),
		progress.WithScaled(false),
		progress.WithWidth(24),
	)
	enemyBar := progress.New(
		progress.WithFillCharacters('=', '-'),
		progress.WithSpringOptions(18, 0.92),
		progress.WithoutPercentage(),
		progress.WithScaled(false),
		progress.WithWidth(24),
	)

	m := model{
		session:             session,
		audio:               audioRuntime,
		scene:               session.Snapshot(),
		width:               defaultPanelWidth + 4,
		playerBar:           playerBar,
		enemyBar:            enemyBar,
		combatLogAutoFollow: true,
	}
	m.selectedIndex = clampSelection(0, m.scene)
	m.syncBarWidth()
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		func() tea.Msg { return tea.RequestWindowSize() },
		m.syncBars(),
	}
	if shouldAutoAdvanceEnemy(m.scene) {
		cmds = append(cmds, m.maybeAdvanceEnemy())
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncBarWidth()
		m.clampCombatLogScroll()
	case playbackTickMsg:
		if cmd := m.advancePlayback(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case tea.KeyPressMsg:
		if msg.String() == "m" {
			m.toggleAudio()
			break
		}
		if m.activeModal != nil {
			switch msg.String() {
			case "i", "esc", "q":
				m.activeModal = nil
			case "ctrl+c":
				_, _, _ = m.session.Apply("quit")
				return m, func() tea.Msg { return tea.Quit() }
			}
			break
		}
		if m.scene.Combat != nil && m.handleCombatLogKey(msg.String()) {
			break
		}
		if m.playback != nil {
			switch msg.String() {
			case "space", " ":
				if cmd := m.fastForwardPlayback(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case "q", "ctrl+c":
				_, _, _ = m.session.Apply("quit")
				return m, func() tea.Msg { return tea.Quit() }
			}
			break
		}
		switch msg.String() {
		case "up", "k":
			m.selectedIndex = previousChoice(m.scene, m.selectedIndex)
		case "down", "j":
			m.selectedIndex = nextChoice(m.scene, m.selectedIndex)
		case "i":
			m.openChoiceModal()
		case "enter":
			choice, ok := selectedChoice(m.scene, m.selectedIndex)
			if ok {
				cmds = append(cmds, m.applyChoice(choice.ID)...)
			}
		case "q", "ctrl+c":
			_, _, _ = m.session.Apply("quit")
			return m, func() tea.Msg { return tea.Quit() }
		}
	}

	var cmd tea.Cmd
	m.playerBar, cmd = m.playerBar.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.enemyBar, cmd = m.enemyBar.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if m.session.ShouldQuit() {
		cmds = append(cmds, func() tea.Msg { return tea.Quit() })
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "Voidnet"
	return v
}

func (m *model) applyChoice(choiceID string) []tea.Cmd {
	previous := m.scene
	scene, events, err := m.session.Apply(choiceID)
	if err != nil {
		m.lastError = err.Error()
		return nil
	}
	m.lastError = ""
	m.playSelectCue(choiceID, previous, scene, events)

	cmds := m.handleSceneTransition(previous, scene, events, choiceID)
	if m.session.ShouldQuit() {
		cmds = append(cmds, func() tea.Msg { return tea.Quit() })
	}
	return cmds
}

func (m *model) handleSceneTransition(previous app.Scene, next app.Scene, events []app.Event, choiceID string) []tea.Cmd {
	cmds := []tea.Cmd{}
	m.activeModal = nil

	switch {
	case isCombatEntry(previous, next):
		m.resetCombatLog()
		m.scene = next
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
		cmds = append(cmds, m.syncBars())
		m.playCue(m.sceneEntryCue(previous, next, events))
		if len(events) > 0 {
			cmds = append(cmds, m.beginPlayback(next, buildPreludeBeats(events)))
			return cmds
		}
		if cmd := m.maybeAdvanceEnemy(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return cmds
	case isCombatChoice(choiceID, previous):
		if len(events) > 0 {
			cmds = append(cmds, m.beginPlayback(next, buildActionBeats(actorFromScene(previous), previous, events)))
			return cmds
		}
		m.scene = next
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
		cmds = append(cmds, m.syncBars())
		if cmd := m.maybeAdvanceEnemy(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return cmds
	default:
		m.scene = next
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
		cmds = append(cmds, m.syncBars())
		m.playEventCues(events)
		m.playCue(m.sceneEntryCue(previous, next, events))
		if !retainsCombatLog(next) {
			m.clearCombatLog()
		}
	}

	return cmds
}

func (m *model) beginPlayback(resolvedScene app.Scene, beats []playbackBeat) tea.Cmd {
	if len(beats) == 0 {
		m.scene = resolvedScene
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
		cmds := []tea.Cmd{m.syncBars()}
		if cmd := m.maybeAdvanceEnemy(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return tea.Batch(cmds...)
	}

	m.playback = &playbackSequence{
		resolvedScene: resolvedScene,
		beats:         beats,
	}
	return m.advancePlayback()
}

func (m *model) advancePlayback() tea.Cmd {
	if m.playback == nil {
		return nil
	}
	if m.playback.index >= len(m.playback.beats) {
		resolved := m.playback.resolvedScene
		m.playback = nil
		m.playbackLines = nil
		m.playbackActor = ""
		m.playbackImpact = false
		m.playbackPhase = ""
		m.playbackPhaseStyle = ""
		m.playCue(m.sceneEntryCue(app.Scene{Kind: "combat"}, resolved, nil))
		if cmd := m.maybeAdvanceEnemy(); cmd != nil {
			return cmd
		}
		if !retainsCombatLog(m.scene) {
			m.clearCombatLog()
		}
		return nil
	}

	beat := m.playback.beats[m.playback.index]
	m.playback.index++
	m.playbackLines = append([]string(nil), beat.lines...)
	m.playbackActor = beat.actor
	m.playbackImpact = beat.impact
	m.playbackPhase = beat.phase
	m.playbackPhaseStyle = beat.phaseStyle
	m.appendCombatLog(beat.lines)
	m.playCue(beat.cue)

	cmds := []tea.Cmd{}
	if beat.applyScene {
		m.scene = m.playback.resolvedScene
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
		cmds = append(cmds, m.syncBars())
	}

	cmds = append(cmds, playbackTickCmd(beat.delay))
	return tea.Batch(cmds...)
}

func (m *model) fastForwardPlayback() tea.Cmd {
	if m.playback == nil {
		return nil
	}

	applyScene := false
	for _, beat := range m.playback.beats[m.playback.index-1:] {
		if beat.applyScene {
			applyScene = true
			break
		}
	}
	if applyScene {
		m.scene = m.playback.resolvedScene
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
	}

	remainingBeats := remainingPlaybackBeats(m.playback)
	m.appendCombatLog(remainingPlaybackLines(m.playback))
	m.playCue(dominantPlaybackCue(remainingBeats))
	m.playback = nil
	m.playbackLines = nil
	m.playbackActor = ""
	m.playbackImpact = false
	m.playbackPhase = ""
	m.playbackPhaseStyle = ""
	m.playCue(m.sceneEntryCue(app.Scene{Kind: "combat"}, m.scene, nil))

	cmds := []tea.Cmd{}
	if applyScene {
		cmds = append(cmds, m.syncBars())
	}
	if cmd := m.maybeAdvanceEnemy(); cmd != nil {
		cmds = append(cmds, cmd)
	} else if !retainsCombatLog(m.scene) {
		m.clearCombatLog()
	}
	return tea.Batch(cmds...)
}

func (m *model) maybeAdvanceEnemy() tea.Cmd {
	if !shouldAutoAdvanceEnemy(m.scene) {
		return nil
	}

	previous := m.scene
	scene, events, err := m.session.AdvanceEnemyTurn()
	if err != nil {
		m.lastError = err.Error()
		return nil
	}
	m.lastError = ""
	return tea.Batch(m.handleEnemyAdvance(previous, scene, events)...)
}

func (m *model) handleEnemyAdvance(previous app.Scene, next app.Scene, events []app.Event) []tea.Cmd {
	if len(events) == 0 {
		m.scene = next
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
		m.playCue(m.sceneEntryCue(previous, next, events))
		return []tea.Cmd{m.syncBars()}
	}
	return []tea.Cmd{m.beginPlayback(next, buildActionBeats(actorEnemy, previous, events))}
}

func (m *model) playCue(event audio.Event) {
	if m.audio == nil || event == "" {
		return
	}
	m.audioCounter++
	m.audio.Play(event, m.audioSeed(event))
}

func (m model) audioSeed(event audio.Event) int64 {
	base := uint64(m.session.Seed())
	return int64(base ^ m.audioCounter ^ hashAudioEvent(event))
}

func hashAudioEvent(event audio.Event) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(event))
	return h.Sum64()
}

func (m *model) toggleAudio() {
	if m.audio == nil || !m.audio.Available() {
		return
	}
	m.audio.SetMuted(!m.audio.Muted())
}

func (m model) audioStatus() string {
	if m.audio == nil || !m.audio.Available() {
		return "audio=unavailable"
	}
	if m.audio.Muted() {
		return "audio=muted"
	}
	return "audio=on"
}

func (m *model) playEventCues(events []app.Event) {
	for _, event := range events {
		m.playCue(event.Cue)
	}
}

func (m *model) playSelectCue(choiceID string, previous app.Scene, next app.Scene, events []app.Event) {
	if shouldPlaySelectCue(choiceID, previous, next, events) {
		m.playCue(audio.EventSelect)
	}
}

func shouldPlaySelectCue(choiceID string, previous app.Scene, next app.Scene, events []app.Event) bool {
	if choiceID == "quit" || choiceID == "" || len(events) == 0 {
		return false
	}
	if strings.HasPrefix(choiceID, "ability:") || choiceID == "isolate" || choiceID == "node:" || choiceID == "inspect" {
		return false
	}
	if next.Kind == "starter_select" {
		return false
	}
	return true
}

func (m model) sceneEntryCue(previous app.Scene, next app.Scene, events []app.Event) audio.Event {
	switch next.Kind {
	case "reward":
		if hasUnlockLine(next.Lines) {
			return audio.EventUnlock
		}
		return audio.EventLevelClear
	case "select_active":
		if next.Title == "Select Active Daemon" {
			return audio.EventAlert
		}
		return ""
	case "game_over":
		if len(next.Lines) > 0 && next.Lines[0] == "Run won." {
			return audio.EventRunVictory
		}
		return ""
	case "combat":
		if !isCombatEntry(previous, next) {
			return ""
		}
		if next.Combat != nil && strings.EqualFold(next.Combat.NodeType, "Boss") {
			return audio.EventAlert
		}
	}
	return ""
}

func hasUnlockLine(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(line, "Unlocked ") {
			return true
		}
	}
	return false
}

func (m *model) syncBarWidth() {
	width := clamp(m.panelWidth()-20, 16, 32)
	m.playerBar.SetWidth(width)
	m.enemyBar.SetWidth(width)
}

func (m *model) syncBars() tea.Cmd {
	if m.scene.Combat == nil {
		return nil
	}
	playerPercent := ratio(m.scene.Combat.Player.IntegrityCurrent, m.scene.Combat.Player.IntegrityMax)
	enemyPercent := ratio(m.scene.Combat.Enemy.IntegrityCurrent, m.scene.Combat.Enemy.IntegrityMax)
	return tea.Batch(
		m.playerBar.SetPercent(playerPercent),
		m.enemyBar.SetPercent(enemyPercent),
	)
}

func (m model) render() string {
	width := m.panelWidth()
	sections := []string{
		renderLogo(m.scene, width),
	}

	if m.lastError != "" {
		sections = append(sections, panel("SYSTEM ERROR", []string{colorize(ansiRed, m.lastError)}, width))
	}

	switch {
	case m.scene.Kind == "combat" && m.scene.Combat != nil:
		sections = append(sections, m.renderCombatLayout(width))
	case m.scene.Kind == "node_select" && m.scene.NodeMap != nil:
		sections = append(sections, m.renderNodeSelect(width))
	default:
		sections = append(sections, panel(sceneHeader(m.scene), decorateLines(m.scene), width))
		sections = append(sections, m.renderSceneMenu(width))
	}

	sections = append(sections, centerLine(colorize(ansiDim, m.controlsHint()), width+4))
	separator := "\n\n"
	if m.scene.Combat != nil {
		separator = "\n"
	}
	rendered := strings.Join(sections, separator)
	if m.activeModal != nil {
		return m.renderModalOverlay(rendered)
	}
	return rendered
}

func (m model) renderCombatLayout(width int) string {
	metrics := m.combatLayoutMetrics(width)
	encounterPanel := panelFixed("ENGAGED ENCOUNTER", m.renderCombatEncounterLines(width, metrics.wide), width, metrics.encounterBody)
	logPanel := m.renderCombatLog(width, metrics.logBody)
	menuPanel := panel("COMMAND DECK", m.renderCombatMenuLines(), width)
	return strings.Join([]string{encounterPanel, logPanel, menuPanel}, "\n")
}

func (m model) renderCombat(width int) string {
	return m.renderCombatLayout(width)
}

func (m model) renderCombatEncounterLines(width int, wide bool) []string {
	combat := m.scene.Combat
	turnLine := fmt.Sprintf("Round %d | %s", combat.Round, turnBanner(actorFromCombatView(combat), m.scene))
	phaseLine := m.renderCombatPhaseLine()
	enemyLabel := colorize(ansiRed, fmt.Sprintf("%s [%s]", combat.Enemy.Label, combat.NodeType))
	playerLabel := colorize(ansiCyan, combat.Player.Label)
	switch m.playbackActor {
	case actorEnemy:
		enemyLabel = colorize(ansiBold+ansiRed, fmt.Sprintf("%s [%s]", combat.Enemy.Label, combat.NodeType))
		if m.playbackImpact {
			playerLabel = colorize(ansiBold+ansiYellow, combat.Player.Label)
		} else {
			playerLabel = colorize(ansiDim+ansiCyan, combat.Player.Label)
		}
	case actorPlayer:
		playerLabel = colorize(ansiBold+ansiCyan, combat.Player.Label)
		if m.playbackImpact {
			enemyLabel = colorize(ansiBold+ansiYellow, fmt.Sprintf("%s [%s]", combat.Enemy.Label, combat.NodeType))
		} else {
			enemyLabel = colorize(ansiDim+ansiRed, fmt.Sprintf("%s [%s]", combat.Enemy.Label, combat.NodeType))
		}
	}

	enemyLines := []string{
		fmt.Sprintf("%s | Trait: %s", enemyLabel, combat.Enemy.Trait),
		fmt.Sprintf("Status: %s | %s", strings.Join(combat.Enemy.Statuses, ", "), barLine(m.enemyBar.View(), combat.Enemy.IntegrityCurrent, combat.Enemy.IntegrityMax)),
	}
	playerLines := []string{
		fmt.Sprintf("%s | Trait: %s", playerLabel, combat.Player.Trait),
		fmt.Sprintf("Status: %s | %s", strings.Join(combat.Player.Statuses, ", "), barLine(m.playerBar.View(), combat.Player.IntegrityCurrent, combat.Player.IntegrityMax)),
	}
	if wide {
		return append([]string{turnLine, phaseLine}, sideBySideLines(enemyLines, playerLines, width, 4)...)
	}

	return []string{
		turnLine,
		phaseLine,
		enemyLines[0],
		enemyLines[1],
		playerLines[0],
		playerLines[1],
	}
}

func (m model) renderCombatPhaseLine() string {
	if m.playbackPhase != "" {
		return colorize(m.playbackPhaseStyle, m.playbackPhase)
	}
	if m.scene.Combat != nil && m.scene.Combat.PlayerTurn {
		return colorize(ansiDim+ansiGreen, "AWAITING COMMAND INPUT")
	}
	return colorize(ansiDim+ansiYellow, "COMBAT LINK STANDBY")
}

func (m model) renderCombatLog(width int, bodyHeight int) string {
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	contentHeight := max(0, bodyHeight-1)
	rows := m.combatLogRows(width)
	start := m.combatLogStart(width, contentHeight)
	end := min(len(rows), start+contentHeight)
	visible := append([]string(nil), rows[start:end]...)
	for len(visible) < contentHeight {
		visible = append(visible, "")
	}

	if len(visible) > 0 {
		if start > 0 {
			visible[0] = colorize(ansiDim, "^^ older entries ^^")
		}
		if end < len(rows) {
			visible[len(visible)-1] = colorize(ansiDim, "vv newer entries vv")
		}
	}

	status := m.combatLogStatus(start, end, len(rows))
	lines := []string{colorize(ansiDim, status)}
	lines = append(lines, visible...)
	return panelFixed("COMBAT LOG", lines, width, bodyHeight)
}

type combatLayoutMetrics struct {
	encounterBody int
	logBody       int
	wide          bool
}

type combatLogClass string

const (
	combatLogPrelude  combatLogClass = "NODE"
	combatLogTurn     combatLogClass = "TURN"
	combatLogAction   combatLogClass = "ACT"
	combatLogRoll     combatLogClass = "ROLL"
	combatLogCapture  combatLogClass = "CAPT"
	combatLogHit      combatLogClass = "HIT"
	combatLogHeal     combatLogClass = "HEAL"
	combatLogStatus   combatLogClass = "STAT"
	combatLogMiss     combatLogClass = "MISS"
	combatLogDot      combatLogClass = "DOT"
	combatLogBackfire combatLogClass = "RISK"
	combatLogCrash    combatLogClass = "FAIL"
	combatLogGain     combatLogClass = "GAIN"
	combatLogLore     combatLogClass = "LOG"
	combatLogFallback combatLogClass = ""
)

func (m model) combatLayoutMetrics(width int) combatLayoutMetrics {
	wide := width >= 78
	encounterBody := wrappedLineCount(m.renderCombatEncounterLines(width, wide), width)
	if encounterBody < 1 {
		encounterBody = 1
	}

	menuBody := wrappedLineCount(m.renderCombatMenuLines(), width)
	if menuBody < 1 {
		menuBody = 1
	}
	if m.height <= 0 {
		return combatLayoutMetrics{
			encounterBody: encounterBody,
			logBody:       8,
			wide:          wide,
		}
	}

	const (
		panelFrameLines = 12
		sectionGaps     = 4
		footerLines     = 1
	)
	logoLines := lineCount(renderLogo(m.scene, width))
	availableBody := m.height - logoLines - footerLines - sectionGaps - panelFrameLines - menuBody
	if m.lastError != "" {
		errorLines := lineCount(panel("SYSTEM ERROR", []string{colorize(ansiRed, m.lastError)}, width))
		availableBody -= errorLines + 1
	}
	if availableBody <= 0 {
		return combatLayoutMetrics{
			encounterBody: encounterBody,
			logBody:       1,
			wide:          wide,
		}
	}

	logBody := max(1, availableBody-encounterBody)
	if logBody < 4 && availableBody > 4 {
		logBody = min(4, availableBody)
	}
	return combatLayoutMetrics{
		encounterBody: encounterBody,
		logBody:       logBody,
		wide:          wide,
	}
}

func sideBySideLines(left []string, right []string, width int, gap int) []string {
	leftWidth := max(1, (width-gap)/2)
	rightWidth := max(1, width-leftWidth-gap)
	leftRows := wrapLines(left, leftWidth)
	rightRows := wrapLines(right, rightWidth)
	rowCount := max(len(leftRows), len(rightRows))
	rows := make([]string, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		leftCell := ""
		if i < len(leftRows) {
			leftCell = leftRows[i]
		}
		rightCell := ""
		if i < len(rightRows) {
			rightCell = rightRows[i]
		}
		rows = append(rows, padRight(leftCell, leftWidth)+strings.Repeat(" ", gap)+padRight(rightCell, rightWidth))
	}
	return rows
}

func wrapLines(lines []string, width int) []string {
	rows := []string{}
	for _, line := range lines {
		rows = append(rows, wrapLine(line, width)...)
	}
	return rows
}

func wrappedLineCount(lines []string, width int) int {
	return len(wrapLines(lines, width))
}

func lineCount(text string) int {
	if text == "" {
		return 0
	}
	return len(strings.Split(text, "\n"))
}

func (m *model) resetCombatLog() {
	m.combatLogLines = nil
	m.combatLogScroll = 0
	m.combatLogAutoFollow = true
	m.combatLogGPrefix = false
}

func (m *model) clearCombatLog() {
	m.resetCombatLog()
}

func (m *model) appendCombatLog(lines []string) {
	if len(lines) == 0 {
		return
	}
	m.combatLogLines = append(m.combatLogLines, lines...)
	m.clampCombatLogScroll()
}

func (m *model) handleCombatLogKey(key string) bool {
	if m.scene.Combat == nil {
		m.combatLogGPrefix = false
		return false
	}
	switch key {
	case "ctrl+u":
		m.combatLogGPrefix = false
		m.scrollCombatLog(-m.combatLogStep())
		return true
	case "ctrl+d":
		m.combatLogGPrefix = false
		m.scrollCombatLog(m.combatLogStep())
		return true
	case "G", "shift+g":
		m.combatLogGPrefix = false
		m.scrollCombatLogBottom()
		return true
	case "g":
		if m.combatLogGPrefix {
			m.combatLogGPrefix = false
			m.scrollCombatLogTop()
		} else {
			m.combatLogGPrefix = true
		}
		return true
	default:
		if m.combatLogGPrefix {
			m.combatLogGPrefix = false
		}
		return false
	}
}

func (m *model) combatLogStep() int {
	visible := max(1, m.combatLayoutMetrics(m.panelWidth()).logBody-1)
	return max(1, visible/2)
}

func (m *model) scrollCombatLog(delta int) {
	contentHeight := max(1, m.combatLayoutMetrics(m.panelWidth()).logBody-1)
	rows := m.combatLogRows(m.panelWidth())
	maxStart := max(0, len(rows)-contentHeight)
	start := m.combatLogStart(m.panelWidth(), contentHeight)
	start = clamp(start+delta, 0, maxStart)
	m.combatLogScroll = start
	m.combatLogAutoFollow = start >= maxStart
}

func (m *model) scrollCombatLogTop() {
	m.combatLogScroll = 0
	m.combatLogAutoFollow = false
}

func (m *model) scrollCombatLogBottom() {
	contentHeight := max(1, m.combatLayoutMetrics(m.panelWidth()).logBody-1)
	rows := m.combatLogRows(m.panelWidth())
	m.combatLogScroll = max(0, len(rows)-contentHeight)
	m.combatLogAutoFollow = true
}

func (m *model) clampCombatLogScroll() {
	if m.scene.Combat == nil {
		m.combatLogScroll = 0
		return
	}
	contentHeight := max(1, m.combatLayoutMetrics(m.panelWidth()).logBody-1)
	rows := m.combatLogRows(m.panelWidth())
	maxStart := max(0, len(rows)-contentHeight)
	if m.combatLogAutoFollow {
		m.combatLogScroll = maxStart
		return
	}
	m.combatLogScroll = clamp(m.combatLogScroll, 0, maxStart)
}

func (m model) combatLogRows(width int) []string {
	if len(m.combatLogLines) == 0 {
		return []string{colorize(ansiDim, "Awaiting combat telemetry...")}
	}
	styled := make([]string, 0, len(m.combatLogLines))
	for _, line := range m.combatLogLines {
		styled = append(styled, formatCombatLogLine(line))
	}
	return wrapLines(styled, width)
}

func (m model) combatLogStart(width int, contentHeight int) int {
	rows := m.combatLogRows(width)
	maxStart := max(0, len(rows)-contentHeight)
	if m.combatLogAutoFollow {
		return maxStart
	}
	return clamp(m.combatLogScroll, 0, maxStart)
}

func (m model) combatLogStatus(start int, end int, total int) string {
	mode := "scrolled"
	if m.combatLogAutoFollow {
		mode = "auto-follow"
	}
	if total == 0 {
		return mode + " | empty"
	}
	if end < start {
		end = start
	}
	return fmt.Sprintf("%s | lines %d-%d of %d", mode, start+1, max(start+1, end), total)
}

func formatCombatLogLine(line string) string {
	class := classifyCombatLogLine(line)
	tag := combatLogTag(class)
	body := styleCombatLogBody(class, line)
	if tag == "" {
		return body
	}
	return tag + " " + body
}

func classifyCombatLogLine(line string) combatLogClass {
	switch {
	case strings.HasPrefix(line, "Entered "), strings.HasPrefix(line, "Encountered "):
		return combatLogPrelude
	case line == "Your turn.", strings.HasPrefix(line, "Enemy turn."):
		return combatLogTurn
	case strings.Contains(line, " used "):
		return combatLogAction
	case strings.HasPrefix(line, "Success chance "):
		return combatLogRoll
	case strings.HasPrefix(line, "Isolation chance "), strings.HasPrefix(line, "Isolation successful."), strings.HasPrefix(line, "Isolation failed."):
		return combatLogCapture
	case strings.Contains(line, " restored ") && strings.Contains(line, " Integrity"):
		return combatLogHeal
	case strings.Contains(line, " took ") && strings.HasSuffix(line, " damage."):
		return combatLogHit
	case strings.Contains(line, " gained +"), strings.Contains(line, " joined the roster."), strings.Contains(line, " is ready to join the roster"), strings.HasPrefix(line, "Unlocked "):
		return combatLogGain
	case strings.Contains(line, " is now "), strings.Contains(line, " gained "), strings.Contains(line, " cleared "):
		return combatLogStatus
	case line == "The action failed to land.":
		return combatLogMiss
	case strings.Contains(line, " suffered ") && strings.Contains(line, " from "):
		return combatLogDot
	case strings.Contains(line, " backfired for "):
		return combatLogBackfire
	case strings.HasSuffix(line, " crashed."), strings.Contains(line, " triggered "):
		return combatLogCrash
	case strings.HasPrefix(line, "[LOG_"):
		return combatLogLore
	default:
		return combatLogFallback
	}
}

func combatLogTag(class combatLogClass) string {
	switch class {
	case combatLogPrelude:
		return colorize(ansiDim+ansiCyan, "[NODE]")
	case combatLogTurn:
		return colorize(ansiBold+ansiYellow, "[TURN]")
	case combatLogAction:
		return colorize(ansiBold+ansiYellow, "[ACT ]")
	case combatLogRoll:
		return colorize(ansiDim+ansiYellow, "[ROLL]")
	case combatLogCapture:
		return colorize(ansiBold+ansiGreen, "[CAPT]")
	case combatLogHit:
		return colorize(ansiBold+ansiRed, "[HIT ]")
	case combatLogHeal:
		return colorize(ansiBold+ansiGreen, "[HEAL]")
	case combatLogStatus:
		return colorize(ansiBold+ansiCyan, "[STAT]")
	case combatLogMiss:
		return colorize(ansiBold+ansiYellow, "[MISS]")
	case combatLogDot:
		return colorize(ansiYellow, "[DOT ]")
	case combatLogBackfire:
		return colorize(ansiYellow, "[RISK]")
	case combatLogCrash:
		return colorize(ansiBold+ansiRed, "[FAIL]")
	case combatLogGain:
		return colorize(ansiBold+ansiGreen, "[GAIN]")
	case combatLogLore:
		return colorize(ansiDim+ansiCyan, "[LOG ]")
	default:
		return ""
	}
}

func styleCombatLogBody(class combatLogClass, line string) string {
	switch class {
	case combatLogPrelude:
		return styleCombatPreludeLine(line)
	case combatLogTurn:
		return styleCombatTurnLine(line)
	case combatLogAction:
		return styleCombatActionLine(line)
	case combatLogRoll:
		return styleCombatRollLine(line)
	case combatLogCapture:
		return styleCombatCaptureLine(line)
	case combatLogHit:
		return styleCombatImpactLine(line, ansiYellow)
	case combatLogHeal:
		return styleCombatImpactLine(line, ansiGreen)
	case combatLogStatus:
		return styleCombatStatusLine(line)
	case combatLogMiss:
		return colorize(ansiBold+ansiYellow, line)
	case combatLogDot:
		return styleCombatDotLine(line)
	case combatLogBackfire:
		return styleCombatBackfireLine(line)
	case combatLogCrash:
		return styleCombatCrashLine(line)
	case combatLogGain:
		return styleCombatGainLine(line)
	case combatLogLore:
		return colorize(ansiDim+ansiCyan, line)
	default:
		return emphasizeCombatActors(line)
	}
}

func styleCombatPreludeLine(line string) string {
	switch {
	case strings.HasPrefix(line, "Entered "):
		return colorize(ansiDim, "Entered ") + colorize(ansiCyan, strings.TrimSuffix(strings.TrimPrefix(line, "Entered "), "."))
	case strings.HasPrefix(line, "Encountered "):
		return colorize(ansiDim, "Encountered ") + colorize(ansiRed, strings.TrimSuffix(strings.TrimPrefix(line, "Encountered "), "."))
	default:
		return colorize(ansiDim, line)
	}
}

func styleCombatTurnLine(line string) string {
	switch {
	case line == "Your turn.":
		return colorize(ansiBold+ansiCyan, "Your turn.")
	case strings.HasPrefix(line, "Enemy turn."):
		return colorize(ansiBold+ansiRed, "Enemy turn.") + colorize(ansiDim, strings.TrimPrefix(line, "Enemy turn."))
	default:
		return colorize(ansiBold+ansiYellow, line)
	}
}

func styleCombatActionLine(line string) string {
	left, right, ok := strings.Cut(line, " used ")
	if !ok {
		return emphasizeCombatActors(line)
	}
	effect, modifierPart, ok := strings.Cut(right, " + ")
	if !ok {
		return styleActorLabel(left) + colorize(ansiDim, " used ") + colorize(ansiBold+ansiYellow, strings.TrimSuffix(right, "."))
	}
	modifier := strings.TrimSuffix(modifierPart, ".")
	return styleActorLabel(left) +
		colorize(ansiDim, " used ") +
		colorize(ansiBold+ansiYellow, effect) +
		colorize(ansiDim, " + ") +
		colorize(ansiYellow, modifier) +
		colorize(ansiDim, ".")
}

func styleCombatRollLine(line string) string {
	prefixPart, breakdownPart, ok := strings.Cut(line, " (")
	if ok {
		breakdown, rollPart, ok := strings.Cut(breakdownPart, "). Roll ")
		if ok {
			prefixFields := strings.Fields(prefixPart)
			if len(prefixFields) >= 3 {
				label := strings.Join(prefixFields[:2], " ")
				chance := prefixFields[2]
				roll := strings.TrimSuffix(rollPart, ".")
				return colorize(ansiDim, label+" ") +
					colorize(ansiBold+ansiYellow, chance) +
					colorize(ansiDim, " (") +
					styleChanceBreakdown(breakdown) +
					colorize(ansiDim, "). Roll ") +
					colorize(ansiBold+ansiYellow, roll) +
					colorize(ansiDim, ".")
			}
		}
	}

	parts := strings.Fields(line)
	if len(parts) < 5 {
		return line
	}
	chance := strings.TrimSuffix(parts[2], ".")
	roll := strings.TrimSuffix(parts[4], ".")
	return colorize(ansiDim, strings.Join(parts[:2], " ")+" ") +
		colorize(ansiBold+ansiYellow, chance) +
		colorize(ansiDim, ".") +
		colorize(ansiDim, " "+parts[3]+" ") +
		colorize(ansiBold+ansiYellow, roll) +
		colorize(ansiDim, ".")
}

func styleChanceBreakdown(breakdown string) string {
	parts := strings.Split(breakdown, ", ")
	styled := make([]string, 0, len(parts))
	for _, part := range parts {
		styled = append(styled, styleChanceBreakdownTerm(part))
	}
	return strings.Join(styled, colorize(ansiDim, ", "))
}

func styleChanceBreakdownTerm(term string) string {
	label, value, ok := splitChanceTerm(term)
	if !ok {
		return colorize(ansiDim, term)
	}
	return styleChanceBreakdownLabel(label) +
		colorize(ansiDim, " ") +
		colorize(ansiBold+ansiYellow, value)
}

func splitChanceTerm(term string) (string, string, bool) {
	index := strings.LastIndex(term, " ")
	if index <= 0 || index >= len(term)-1 {
		return "", "", false
	}
	label := term[:index]
	value := term[index+1:]
	if len(value) == 0 {
		return "", "", false
	}
	switch value[0] {
	case '+', '-':
		if len(value) == 1 {
			return "", "", false
		}
	default:
		if value[0] < '0' || value[0] > '9' {
			return "", "", false
		}
	}
	return label, value, true
}

func styleChanceBreakdownLabel(label string) string {
	switch label {
	case "target Stability", "stability diff", "target resistance":
		return colorize(ansiCyan, label)
	default:
		return colorize(ansiDim, label)
	}
}

func styleCombatCaptureLine(line string) string {
	switch {
	case strings.HasPrefix(line, "Isolation chance "):
		return styleCombatRollLine(line)
	case strings.HasPrefix(line, "Isolation successful."):
		return colorize(ansiBold+ansiGreen, line)
	case strings.Contains(line, "above 50%"):
		return colorize(ansiYellow, "Isolation failed.") + colorize(ansiDim, " Target Integrity is above ") + colorize(ansiBold+ansiYellow, "50%") + colorize(ansiDim, ".")
	case strings.Contains(line, "resisted the breach"):
		return colorize(ansiYellow, "Isolation failed.") + colorize(ansiDim, " The daemon resisted the breach.")
	default:
		return line
	}
}

func styleCombatImpactLine(line string, amountColor string) string {
	var separator string
	switch {
	case strings.Contains(line, " took "):
		separator = " took "
	case strings.Contains(line, " restored "):
		separator = " restored "
	default:
		return emphasizeCombatActors(line)
	}
	left, right, ok := strings.Cut(line, separator)
	if !ok {
		return emphasizeCombatActors(line)
	}
	parts := strings.Fields(right)
	if len(parts) == 0 {
		return emphasizeCombatActors(line)
	}
	amount := parts[0]
	remainder := strings.TrimPrefix(right, amount)
	return styleActorLabel(left) +
		colorize(ansiDim, separator) +
		colorize(ansiBold+amountColor, amount) +
		colorize(ansiDim, remainder)
}

func styleCombatStatusLine(line string) string {
	switch {
	case strings.Contains(line, " is now "):
		left, status, _ := strings.Cut(line, " is now ")
		return styleActorLabel(left) + colorize(ansiDim, " is now ") + styleStatusName(strings.TrimSuffix(status, ".")) + colorize(ansiDim, ".")
	case strings.Contains(line, " gained "):
		left, status, _ := strings.Cut(line, " gained ")
		return styleActorLabel(left) + colorize(ansiDim, " gained ") + styleStatusName(strings.TrimSuffix(status, ".")) + colorize(ansiDim, ".")
	case strings.Contains(line, " cleared "):
		left, status, _ := strings.Cut(line, " cleared ")
		return styleActorLabel(left) + colorize(ansiDim, " cleared ") + colorize(ansiBold+ansiGreen, strings.TrimSuffix(status, ".")) + colorize(ansiDim, ".")
	default:
		return emphasizeCombatActors(line)
	}
}

func styleCombatDotLine(line string) string {
	left, right, ok := strings.Cut(line, " suffered ")
	if !ok {
		return emphasizeCombatActors(line)
	}
	damage, sourcePart, ok := strings.Cut(right, " damage from ")
	if !ok {
		return emphasizeCombatActors(line)
	}
	return styleActorLabel(left) +
		colorize(ansiDim, " suffered ") +
		colorize(ansiBold+ansiYellow, damage) +
		colorize(ansiDim, " damage from ") +
		colorize(ansiYellow, strings.TrimSuffix(sourcePart, ".")) +
		colorize(ansiDim, ".")
}

func styleCombatBackfireLine(line string) string {
	left, right, ok := strings.Cut(line, " backfired for ")
	if !ok {
		return line
	}
	damage := strings.TrimSuffix(strings.TrimSuffix(right, " damage."), " damage")
	return colorize(ansiYellow, left) +
		colorize(ansiDim, " backfired for ") +
		colorize(ansiBold+ansiRed, damage) +
		colorize(ansiDim, " damage.")
}

func styleCombatCrashLine(line string) string {
	switch {
	case strings.HasSuffix(line, " crashed."):
		return colorize(ansiBold+ansiRed, strings.TrimSuffix(line, " crashed.")) + colorize(ansiDim, " crashed.")
	case strings.Contains(line, " triggered "):
		left, right, _ := strings.Cut(line, " triggered ")
		trait, damagePart, _ := strings.Cut(right, " for ")
		damage := strings.TrimSuffix(strings.TrimSuffix(damagePart, " damage."), " damage")
		return styleActorLabel(left) +
			colorize(ansiDim, " triggered ") +
			colorize(ansiYellow, trait) +
			colorize(ansiDim, " for ") +
			colorize(ansiBold+ansiRed, damage) +
			colorize(ansiDim, " damage.")
	default:
		return line
	}
}

func styleCombatGainLine(line string) string {
	switch {
	case strings.Contains(line, " gained +"):
		left, right, _ := strings.Cut(line, " gained ")
		return colorize(ansiGreen, left) + colorize(ansiDim, " gained ") + highlightSignedNumbers(right, ansiBold+ansiGreen)
	case strings.Contains(line, " joined the roster."):
		name := strings.TrimSuffix(line, " joined the roster.")
		return colorize(ansiBold+ansiGreen, name) + colorize(ansiDim, " joined the roster.")
	case strings.Contains(line, " is ready to join the roster"):
		name, rest, _ := strings.Cut(line, " is ready to join the roster")
		return colorize(ansiGreen, name) + colorize(ansiDim, " is ready to join the roster") + colorize(ansiDim, rest)
	case strings.HasPrefix(line, "Unlocked "):
		prefix, value, _ := strings.Cut(line, ": ")
		return colorize(ansiGreen, prefix+":") + colorize(ansiBold+ansiGreen, " "+strings.TrimSuffix(value, ".")) + colorize(ansiDim, ".")
	default:
		return colorize(ansiGreen, line)
	}
}

func emphasizeCombatActors(line string) string {
	line = strings.ReplaceAll(line, "Enemy ", colorize(ansiRed, "Enemy "))
	line = strings.ReplaceAll(line, "Your ", colorize(ansiCyan, "Your "))
	return line
}

func styleActorLabel(label string) string {
	switch {
	case strings.HasPrefix(label, "Enemy "):
		return colorize(ansiBold+ansiRed, label)
	case strings.HasPrefix(label, "Your "):
		return colorize(ansiBold+ansiCyan, label)
	default:
		return colorize(ansiBold, label)
	}
}

func styleStatusName(name string) string {
	switch name {
	case "Stabilized":
		return colorize(ansiBold+ansiGreen, name)
	case "Corrupted", "Leaking", "Delayed":
		return colorize(ansiBold+ansiYellow, name)
	default:
		return colorize(ansiBold+ansiCyan, name)
	}
}

func highlightSignedNumbers(text string, code string) string {
	parts := strings.Fields(text)
	for i, part := range parts {
		trimmed := strings.Trim(part, ",.")
		if strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") {
			if _, err := strconv.Atoi(trimmed); err == nil {
				suffix := strings.TrimPrefix(part, trimmed)
				parts[i] = colorize(code, trimmed) + colorize(ansiDim, suffix)
			}
		}
	}
	return colorize(ansiDim, strings.Join(parts, " "))
}

func (m model) renderNodeSelect(width int) string {
	focusedID := focusedNodeID(m.scene, m.selectedIndex, m.scene.NodeMap.CurrentNodeID)
	mapLines := renderNodeMap(*m.scene.NodeMap, focusedID)
	detailLines := nodeDetailLines(*m.scene.NodeMap, focusedID)
	lines := append(mapLines, "")
	lines = append(lines, detailLines...)
	return panel("NETWORK MAP", lines, width)
}

func renderLogo(scene app.Scene, width int) string {
	screenWidth := width + 4
	logo := []string{
		renderLogoLine(" _    __      _     __           __ ", screenWidth, ".::", "::."),
		renderLogoLine("| |  / /___  (_)___/ /___  ___  / /_", screenWidth, "//:", ":\\\\"),
		renderLogoLine("| | / / __ \\/ / __  / __ \\/ _ \\/ __/", screenWidth, "[[ ", " ]]"),
		renderLogoLine("| |/ / /_/ / / /_/ / / / /  __/ /_  ", screenWidth, "\\\\:", "://"),
		renderLogoLine("|___/\\____/_/\\__,_/_/ /_/\\___/\\__/  ", screenWidth, "`::", "::'"),
		centerLine(colorize(ansiGreen, ":: "+strings.ToUpper(scene.Title)+" ::"), screenWidth),
	}
	return strings.Join(logo, "\n")
}

func renderLogoLine(text string, width int, leftAccent string, rightAccent string) string {
	text = colorize(ansiBold+ansiCyan, text)
	left := colorize(ansiDim+ansiCyan, leftAccent)
	right := colorize(ansiDim+ansiCyan, rightAccent)

	extra := width - visibleWidth(text)
	if extra <= 0 {
		return text
	}

	leftSpan := extra / 2
	rightSpan := extra - leftSpan
	accentWidth := visibleWidth(left) + visibleWidth(right) + 2
	if extra < accentWidth+4 {
		return centerLine(text, width)
	}

	leftPad := max(0, leftSpan-visibleWidth(left)-1)
	rightPad := max(0, rightSpan-visibleWidth(right)-1)
	return strings.Repeat(" ", leftPad) + left + " " + text + " " + right + strings.Repeat(" ", rightPad)
}

func sceneHeader(scene app.Scene) string {
	switch scene.Kind {
	case "starter_select":
		return "BOOTSTRAP"
	case "inspect":
		return "TRACE VIEW"
	case "replace":
		return "ROSTER OVERRIDE"
	case "reward":
		return "NODE PAYLOAD"
	case "select_active":
		return "ACTIVE SLOT"
	case "game_over":
		return "RUN SUMMARY"
	default:
		return strings.ToUpper(scene.Title)
	}
}

func decorateLines(scene app.Scene) []string {
	if scene.Kind == "inspect" {
		return decorateInspectLines(scene.Lines)
	}
	if scene.Kind == "maintenance" {
		return decorateMaintenanceLines(scene.Lines)
	}
	lines := make([]string, 0, len(scene.Lines)+4)
	for _, line := range scene.Lines {
		if strings.TrimSpace(line) == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, stylizeLine(line))
	}
	return lines
}

func decorateMaintenanceLines(lines []string) []string {
	out := make([]string, 0, len(lines)+3)
	for _, line := range lines {
		switch {
		case strings.TrimSpace(line) == "":
			out = append(out, "")
		case strings.HasPrefix(line, "Choose repair or fortify"):
			prefix, suffix, ok := strings.Cut(line, ", then ")
			if !ok {
				out = append(out, colorize(ansiBold+ansiGreen, line))
				continue
			}
			out = append(out, colorize(ansiBold+ansiGreen, prefix)+colorize(ansiDim, ", then "+suffix))
		case strings.HasPrefix(line, "Active daemon: "):
			out = append(out, stylizeMaintenanceSummary(line))
			out = append(out, colorize(ansiDim+ansiCyan, ":: repair // fortify // bank ::"))
		case strings.HasPrefix(line, "Maintenance charge: "):
			out = append(out, stylizeMaintenanceCharge(line))
		case line == "Roster telemetry:":
			out = append(out, colorize(ansiBold+ansiGreen, line))
		case strings.HasPrefix(line, "* "):
			out = append(out, colorize(ansiBold+ansiGreen, "* ")+stylizeTraceSummary(strings.TrimPrefix(line, "* "), ansiCyan))
		case strings.HasPrefix(line, "  "):
			out = append(out, "  "+stylizeTraceSummary(strings.TrimSpace(line), ansiCyan))
		default:
			out = append(out, stylizeLine(line))
		}
	}
	return out
}

func decorateInspectLines(lines []string) []string {
	out := make([]string, 0, len(lines)+6)
	for _, line := range lines {
		switch {
		case strings.TrimSpace(line) == "":
			out = append(out, colorize(ansiDim+ansiCyan, "····················"))
		case strings.HasPrefix(line, "Your "):
			out = append(out, stylizeTraceSummary(line, ansiCyan))
		case strings.HasPrefix(line, "Enemy "):
			out = append(out, stylizeTraceSummary(line, ansiRed))
		case strings.HasPrefix(line, "Abilities:"):
			out = append(out, stylizeTraceAbilities(line))
		case strings.HasPrefix(line, "Trait:"):
			out = append(out, stylizeTraceTrait(line))
		case strings.HasPrefix(line, "Active status:"):
			out = append(out, stylizeTraceStatusLine(line))
		case strings.HasPrefix(line, "Active statuses:"):
			out = append(out, colorize(ansiBold+ansiGreen, "Active statuses:"))
		case strings.HasPrefix(line, "  "):
			out = append(out, "  "+stylizeTraceStatusLine(strings.TrimSpace(line)))
		case strings.HasSuffix(line, ":"):
			out = append(out, colorize(ansiBold+ansiGreen, line))
		case strings.HasPrefix(line, "INT ") || strings.HasPrefix(line, "SPD ") || strings.HasPrefix(line, "STB "):
			out = append(out, stylizeTraceGlossary(line))
		case strings.Contains(line, ": "):
			out = append(out, stylizeTraceReferenceLine(line))
		default:
			out = append(out, line)
		}
	}
	return out
}

func stylizeLine(line string) string {
	switch {
	case strings.HasPrefix(line, "Captured:"):
		return colorize(ansiGreen, line)
	case strings.HasPrefix(line, "Seed:"):
		return colorize(ansiDim, line)
	case strings.HasPrefix(line, "Remaining daemons:"):
		return colorize(ansiDim, line)
	default:
		return line
	}
}

func stylizeMaintenanceSummary(line string) string {
	label, rest, ok := strings.Cut(line, ": ")
	if !ok {
		return stylizeTraceSummary(line, ansiCyan)
	}
	return colorize(ansiBold+ansiCyan, label+": ") + stylizeTraceSummary(rest, ansiCyan)
}

func stylizeMaintenanceCharge(line string) string {
	label, rest, ok := strings.Cut(line, ": ")
	if !ok {
		return line
	}
	color := ansiDim
	if strings.Contains(rest, "ready") {
		color = ansiBold + ansiGreen
	}
	return colorize(ansiBold+ansiCyan, label+": ") + colorize(color, rest)
}

func stylizeTraceSummary(line string, actorColor string) string {
	parts := strings.Split(line, " | ")
	if len(parts) == 0 {
		return line
	}

	var out []string
	for i, part := range parts {
		switch {
		case i == 0:
			head, tail, ok := strings.Cut(part, " ")
			if !ok {
				out = append(out, colorize(ansiBold+actorColor, part))
				continue
			}
			out = append(out, colorize(ansiBold+actorColor, head+" ")+colorize(ansiBold, tail))
		case strings.Contains(part, " INT"):
			out = append(out, colorize(ansiBold+ansiYellow, part))
		case strings.HasPrefix(part, "SPD "):
			out = append(out, colorize(ansiBold+ansiCyan, part))
		case strings.HasPrefix(part, "STB "):
			out = append(out, colorize(ansiBold+ansiGreen, part))
		default:
			out = append(out, colorize(ansiBold+ansiYellow, part))
		}
	}
	return strings.Join(out, colorize(ansiDim, " | "))
}

func stylizeTraceAbilities(line string) string {
	label, rest, ok := strings.Cut(line, ": ")
	if !ok {
		return line
	}
	parts := strings.Split(rest, ", ")
	for i, part := range parts {
		parts[i] = colorize(ansiBold+ansiYellow, part)
	}
	return colorize(ansiCyan, label+": ") + strings.Join(parts, colorize(ansiDim, ", "))
}

func stylizeTraceTrait(line string) string {
	label, rest, ok := strings.Cut(line, ": ")
	if !ok {
		return line
	}
	name, detail, hasDetail := strings.Cut(rest, " - ")
	styled := colorize(ansiGreen, label+": ") + colorize(ansiBold+ansiYellow, name)
	if hasDetail {
		styled += colorize(ansiDim, " - ") + stylizeSignedTerms(detail)
	}
	return styled
}

func stylizeTraceStatusLine(line string) string {
	label, rest, ok := strings.Cut(line, ": ")
	if !ok {
		return line
	}
	if rest == "None" {
		return colorize(ansiCyan, label+": ") + colorize(ansiDim, rest)
	}
	return colorize(ansiCyan, label+": ") + stylizeTraceStatusPayload(rest)
}

func stylizeTraceStatusPayload(payload string) string {
	name, detail, hasDetail := strings.Cut(payload, ": ")
	styled := colorize(ansiBold+ansiYellow, name)
	if hasDetail {
		styled += colorize(ansiDim, ": ") + stylizeSignedTerms(detail)
	}
	return styled
}

func stylizeTraceGlossary(line string) string {
	code, rest, ok := strings.Cut(line, " ")
	if !ok {
		return line
	}
	label, detail, ok := strings.Cut(rest, ": ")
	if !ok {
		return colorize(ansiBold+ansiGreen, line)
	}
	return colorize(ansiBold+ansiGreen, code+" ") + colorize(ansiBold+ansiYellow, label) + colorize(ansiDim, ": ") + detail
}

func stylizeTraceReferenceLine(line string) string {
	label, detail, ok := strings.Cut(line, ": ")
	if !ok {
		return line
	}
	return colorize(ansiBold+ansiYellow, label) + colorize(ansiDim, ": ") + stylizeSignedTerms(detail)
}

func stylizeSignedTerms(text string) string {
	parts := strings.Split(text, ", ")
	for i, part := range parts {
		parts[i] = stylizeSignedTerm(part)
	}
	return strings.Join(parts, colorize(ansiDim, ", "))
}

func stylizeSignedTerm(part string) string {
	switch {
	case strings.HasPrefix(part, "+"):
		return colorize(ansiBold+ansiGreen, part)
	case strings.HasPrefix(part, "-"):
		return colorize(ansiBold+ansiRed, part)
	case strings.HasPrefix(part, "deals "), strings.HasPrefix(part, "random "), strings.Contains(part, "damage"):
		return colorize(ansiYellow, part)
	case strings.Contains(part, "resistance"):
		return colorize(ansiCyan, part)
	default:
		return part
	}
}

func (m model) controlsHint() string {
	audioHint := " | m toggle | " + m.audioStatus()
	if m.audio == nil || !m.audio.Available() {
		audioHint = " | " + m.audioStatus()
	}
	if m.activeModal != nil {
		return fmt.Sprintf("controls: i/esc/q close detail%s | scene=%s", audioHint, m.scene.Kind)
	}
	if m.playback != nil {
		return fmt.Sprintf("controls: gg/G/ctrl+u/ctrl+d log | space fast-forward | q quit%s | scene=%s", audioHint, m.scene.Kind)
	}
	if m.scene.Combat != nil {
		return fmt.Sprintf("controls: j/k menu | i detail | gg/G/ctrl+u/ctrl+d log | enter select | q quit%s | scene=%s", audioHint, m.scene.Kind)
	}
	if m.selectedChoiceDetails() != nil {
		return fmt.Sprintf("controls: up/down or j/k | i detail | enter select | q quit%s | scene=%s", audioHint, m.scene.Kind)
	}
	return fmt.Sprintf("controls: up/down or j/k | enter select | q quit%s | scene=%s", audioHint, m.scene.Kind)
}

func renderMenu(choices []app.Choice, selectedIndex int, width int, locked bool) string {
	return panel("COMMAND DECK", renderMenuLines(choices, selectedIndex, locked), width)
}

func (m model) renderSceneMenu(width int) string {
	lines := renderMenuLines(enabledChoices(m.scene), m.selectedIndex, m.playback != nil)
	if m.scene.Kind == "maintenance" {
		lines = renderMaintenanceMenuLines(enabledChoices(m.scene), m.selectedIndex, m.playback != nil)
	}
	detail := m.selectedChoiceDetails()
	if detail != nil && m.playback == nil {
		lines = append(lines, "")
		if m.scene.Kind == "maintenance" {
			lines = append(lines, colorize(ansiDim+ansiCyan, ":: service note ::"))
			lines = append(lines, colorize(ansiCyan, "Preview:"))
			if code := maintenanceChoiceColor(selectedChoiceID(m.scene, m.selectedIndex)); code != "" {
				lines = append(lines, colorize(code, detail.Preview))
			} else {
				lines = append(lines, detail.Preview)
			}
			lines = append(lines, colorize(ansiDim, "Press i for full command detail."))
		} else {
			lines = append(lines, colorize(ansiCyan, "Preview:"))
			lines = append(lines, detail.Preview)
			lines = append(lines, colorize(ansiDim, "Press i for full command detail."))
		}
	}
	return panel("COMMAND DECK", lines, width)
}

func (m model) renderCombatMenuLines() []string {
	lines := renderMenuLines(enabledChoices(m.scene), m.selectedIndex, m.playback != nil)
	if m.playback != nil {
		return lines
	}
	detail := m.selectedChoiceDetails()
	if detail == nil {
		return lines
	}

	lines = append(lines, "")
	lines = append(lines, colorize(ansiCyan, "Preview:"))
	lines = append(lines, detail.Preview)
	if m.playback != nil {
		lines = append(lines, colorize(ansiDim, "Detail modal unlocks when the sequence ends."))
	} else {
		lines = append(lines, colorize(ansiDim, "Press i for full command detail."))
	}
	return lines
}

func renderMenuLines(choices []app.Choice, selectedIndex int, locked bool) []string {
	lines := []string{}
	for i, choice := range choices {
		prefix := "  [ ]"
		label := choice.Label
		if locked {
			lines = append(lines, colorize(ansiDim, fmt.Sprintf("%s %s", prefix, label)))
			continue
		}
		if i == selectedIndex {
			prefix = colorize(ansiGreen, ">> [*]")
			label = colorize(ansiBold+ansiGreen, choice.Label)
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, label))
	}
	if len(lines) == 0 {
		lines = append(lines, colorize(ansiDim, "No available actions"))
	}
	if locked {
		lines = append(lines, "", colorize(ansiYellow, "Sequence running. Space fast-forward."))
	}
	return lines
}

func renderMaintenanceMenuLines(choices []app.Choice, selectedIndex int, locked bool) []string {
	lines := []string{}
	for i, choice := range choices {
		prefix := "  [ ]"
		label := maintenanceChoiceLabel(choice)
		if locked {
			lines = append(lines, colorize(ansiDim, fmt.Sprintf("%s %s", prefix, choice.Label)))
			continue
		}
		if i == selectedIndex {
			prefix = colorize(ansiGreen, ">> [*]")
			label = colorize(ansiBold+ansiGreen, choice.Label)
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, label))
	}
	if len(lines) == 0 {
		lines = append(lines, colorize(ansiDim, "No available actions"))
	}
	if locked {
		lines = append(lines, "", colorize(ansiYellow, "Sequence running. Space fast-forward."))
	}
	return lines
}

func maintenanceChoiceLabel(choice app.Choice) string {
	switch choice.ID {
	case "maintenance:repair":
		return colorize(ansiBold+ansiGreen, choice.Label)
	case "maintenance:fortify":
		return colorize(ansiBold+ansiCyan, choice.Label)
	case "rotate":
		return colorize(ansiBold+ansiYellow, choice.Label)
	case "view:maintenance", "view:nodes":
		return colorize(ansiBold+ansiCyan, choice.Label)
	case "quit":
		return colorize(ansiDim, choice.Label)
	default:
		return choice.Label
	}
}

func maintenanceChoiceColor(choiceID string) string {
	switch choiceID {
	case "maintenance:repair":
		return ansiBold + ansiGreen
	case "maintenance:fortify":
		return ansiBold + ansiCyan
	case "rotate":
		return ansiBold + ansiYellow
	case "view:maintenance", "view:nodes":
		return ansiBold + ansiCyan
	default:
		return ""
	}
}

func selectedChoiceID(scene app.Scene, selectedIndex int) string {
	choice, ok := selectedChoice(scene, selectedIndex)
	if !ok {
		return ""
	}
	return choice.ID
}

func (m *model) openChoiceModal() {
	if m.playback != nil {
		return
	}
	detail := m.selectedChoiceDetails()
	if detail == nil {
		return
	}
	copyDetail := *detail
	copyDetail.Lines = append([]string(nil), detail.Lines...)
	m.activeModal = &copyDetail
}

func (m model) selectedChoiceDetails() *app.ChoiceDetails {
	choice, ok := selectedChoice(m.scene, m.selectedIndex)
	if !ok || choice.Details == nil {
		return nil
	}
	return choice.Details
}

func (m model) renderModalOverlay(base string) string {
	if m.activeModal == nil {
		return base
	}

	screenWidth := max(m.width, m.panelWidth()+4)
	modalWidth := clamp(m.panelWidth()-10, 38, 68)
	lines := []string{colorize(ansiBold+ansiGreen, m.activeModal.Preview), ""}
	lines = append(lines, m.activeModal.Lines...)
	lines = append(lines, "", colorize(ansiDim, "Close with i, Esc, or q."))

	modal := panel(m.activeModal.Title, lines, modalWidth)
	baseRows := strings.Split(base, "\n")
	modalRows := strings.Split(modal, "\n")
	top := max(0, (len(baseRows)-len(modalRows))/2)

	for i, row := range modalRows {
		idx := top + i
		centered := centerLine(row, screenWidth)
		if idx >= len(baseRows) {
			baseRows = append(baseRows, centered)
			continue
		}
		baseRows[idx] = centered
	}
	return strings.Join(baseRows, "\n")
}

const (
	actorPlayer = "player"
	actorEnemy  = "enemy"
)

func playbackTickCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return playbackTickMsg{}
	})
}

func eventMessages(events []app.Event) []string {
	lines := make([]string, 0, len(events))
	for _, event := range events {
		lines = append(lines, event.Message)
	}
	return lines
}

func isCombatEntry(previous app.Scene, next app.Scene) bool {
	return !retainsCombatLog(previous) && next.Combat != nil
}

func isCombatChoice(choiceID string, scene app.Scene) bool {
	if scene.Combat == nil {
		return false
	}
	return strings.HasPrefix(choiceID, "ability:") || choiceID == "isolate"
}

func actorFromScene(scene app.Scene) string {
	if scene.Combat == nil {
		return ""
	}
	if scene.Combat.PlayerTurn {
		return actorPlayer
	}
	return actorEnemy
}

func shouldAutoAdvanceEnemy(scene app.Scene) bool {
	return scene.Combat != nil && !scene.Combat.PlayerTurn
}

func actorFromCombatView(combat *app.CombatView) string {
	if combat == nil {
		return ""
	}
	if combat.PlayerTurn {
		return actorPlayer
	}
	return actorEnemy
}

func buildPreludeBeats(events []app.Event) []playbackBeat {
	beats := make([]playbackBeat, 0, len(events))
	for _, event := range events {
		beats = append(beats, playbackBeat{
			lines:      []string{event.Message},
			delay:      playbackIntroDelay,
			phase:      "LINK ESTABLISHED",
			phaseStyle: ansiBold + ansiCyan,
			cue:        event.Cue,
		})
	}
	return beats
}

func buildActionBeats(actor string, scene app.Scene, events []app.Event) []playbackBeat {
	if actor == "" {
		return buildPreludeBeats(events)
	}

	beats := []playbackBeat{{
		lines:      []string{turnBanner(actor, scene)},
		delay:      playbackTurnDelay,
		actor:      actor,
		phase:      playbackTurnPhase(actor),
		phaseStyle: playbackActorStyle(actor),
		cue:        turnBannerCue(actor),
	}}
	if len(events) == 0 {
		return beats
	}
	lines := eventMessages(events)

	if len(lines) == 1 {
		beats = append(beats, playbackBeat{
			lines:      append([]string(nil), lines...),
			delay:      playbackImpactDelay,
			actor:      actor,
			impact:     true,
			applyScene: true,
			phase:      "PAYLOAD LANDED",
			phaseStyle: ansiBold + ansiYellow,
			cue:        dominantEventCue(events),
		})
		return beats
	}

	beats = append(beats, playbackBeat{
		lines:      []string{lines[0]},
		delay:      playbackActionDelay,
		actor:      actor,
		phase:      "ABILITY PRIMED",
		phaseStyle: playbackActorStyle(actor),
		cue:        actionAnnounceCue(actor),
	})

	index := 1
	if index < len(lines) && isRollLine(lines[index]) {
		beats = append(beats, playbackBeat{
			lines:      []string{lines[index]},
			delay:      playbackRollDelay,
			actor:      actor,
			phase:      "RESOLUTION CHECK",
			phaseStyle: ansiBold + ansiYellow,
		})
		index++
	}

	if index < len(lines) {
		beats = append(beats, playbackBeat{
			lines:      append([]string(nil), lines[index:]...),
			delay:      playbackImpactDelay,
			actor:      actor,
			impact:     true,
			applyScene: true,
			phase:      "PAYLOAD LANDED",
			phaseStyle: ansiBold + ansiYellow,
			cue:        dominantEventCue(events[index:]),
		})
		return beats
	}

	last := len(beats) - 1
	beats[last].impact = true
	beats[last].applyScene = true
	beats[last].delay = playbackImpactDelay
	beats[last].phase = "PAYLOAD LANDED"
	beats[last].phaseStyle = ansiBold + ansiYellow
	return beats
}

func playbackTurnPhase(actor string) string {
	switch actor {
	case actorEnemy:
		return "HOSTILE EXECUTION"
	case actorPlayer:
		return "OPERATOR EXECUTION"
	default:
		return "COMBAT SEQUENCE"
	}
}

func playbackActorStyle(actor string) string {
	switch actor {
	case actorEnemy:
		return ansiBold + ansiRed
	case actorPlayer:
		return ansiBold + ansiCyan
	default:
		return ansiBold + ansiYellow
	}
}

func turnBannerCue(actor string) audio.Event {
	if actor == actorEnemy {
		return audio.EventAlert
	}
	return ""
}

func actionAnnounceCue(actor string) audio.Event {
	if actor == actorPlayer {
		return audio.EventHackStart
	}
	return ""
}

func turnBanner(actor string, scene app.Scene) string {
	switch actor {
	case actorEnemy:
		if scene.Combat != nil && scene.Combat.Round == 1 {
			return "Enemy turn. Acting first."
		}
		return "Enemy turn."
	case actorPlayer:
		return "Your turn."
	default:
		return "Combat pulse."
	}
}

func isRollLine(line string) bool {
	return strings.HasPrefix(line, "Success chance ") || strings.HasPrefix(line, "Isolation chance ")
}

func retainsCombatLog(scene app.Scene) bool {
	return scene.Combat != nil || scene.Kind == "inspect"
}

func remainingPlaybackLines(playback *playbackSequence) []string {
	if playback == nil {
		return nil
	}
	lines := []string{}
	for i := playback.index; i < len(playback.beats); i++ {
		lines = append(lines, playback.beats[i].lines...)
	}
	return lines
}

func remainingPlaybackBeats(playback *playbackSequence) []playbackBeat {
	if playback == nil || playback.index >= len(playback.beats) {
		return nil
	}
	return append([]playbackBeat(nil), playback.beats[playback.index:]...)
}

func dominantPlaybackCue(beats []playbackBeat) audio.Event {
	best := audio.Event("")
	bestPriority := 0
	for _, beat := range beats {
		priority := cuePriority(beat.cue)
		if priority > bestPriority {
			best = beat.cue
			bestPriority = priority
		}
	}
	return best
}

func dominantEventCue(events []app.Event) audio.Event {
	best := audio.Event("")
	bestPriority := 0
	for _, event := range events {
		priority := cuePriority(event.Cue)
		if priority > bestPriority {
			best = event.Cue
			bestPriority = priority
		}
	}
	return best
}

func cuePriority(event audio.Event) int {
	switch event {
	case audio.EventDaemonCaptured:
		return 8
	case audio.EventCrash:
		return 7
	case audio.EventBackfire:
		return 6
	case audio.EventPatchRestore:
		return 5
	case audio.EventCorruptionBurst:
		return 4
	case audio.EventGlitchStinger:
		return 3
	case audio.EventHackSuccess:
		return 2
	case audio.EventHackFail:
		return 1
	default:
		return 0
	}
}

func renderNodeMap(view app.NodeMapView, focusedID string) []string {
	const (
		colWidth = 14
		rows     = 9
	)
	maxDepth := 0
	for _, node := range view.Nodes {
		if node.Depth > maxDepth {
			maxDepth = node.Depth
		}
	}
	canvasWidth := max(12, maxDepth*colWidth+8)
	canvas := make([][]rune, rows)
	for i := range canvas {
		canvas[i] = []rune(strings.Repeat(" ", canvasWidth))
	}

	positions := map[string]point{}
	for _, node := range view.Nodes {
		x := 2 + node.Depth*colWidth
		y := laneRow(node.Lane)
		positions[node.ID] = point{X: x, Y: y}
		drawText(canvas, x, y, nodeGlyph(node, focusedID))
	}

	for _, edge := range view.Edges {
		from, okFrom := positions[edge.From]
		to, okTo := positions[edge.To]
		if !okFrom || !okTo {
			continue
		}
		drawConnector(canvas, from, to)
	}

	lines := make([]string, 0, rows)
	for _, row := range canvas {
		line := strings.TrimRight(string(row), " ")
		if strings.Contains(line, "[>]") {
			line = strings.Replace(line, "[>]", colorize(ansiBold+ansiGreen, "[>]"), 1)
		}
		lines = append(lines, line)
	}
	return lines
}

func nodeDetailLines(view app.NodeMapView, focusedID string) []string {
	node := findNode(view, focusedID)
	if node == nil {
		return []string{colorize(ansiDim, "No node data available.")}
	}
	state := "Locked"
	switch {
	case node.Current:
		state = "Current position"
	case node.Failed:
		state = "Failed"
	case node.Resolved:
		state = "Cleared"
	case node.Selectable:
		state = "Reachable"
	case !node.Visible:
		state = "Unknown"
	default:
		state = "Visible"
	}

	label := node.Label
	nodeType := node.Type
	if !node.Visible {
		label = "???"
		nodeType = "Unknown"
	}

	return []string{
		colorize(ansiYellow, "Focused node telemetry:"),
		fmt.Sprintf("Label: %s", label),
		fmt.Sprintf("Type: %s", nodeType),
		fmt.Sprintf("State: %s", state),
	}
}

func panel(title string, lines []string, width int) string {
	return panelFixed(title, lines, width, -1)
}

func panelFixed(title string, lines []string, width int, bodyHeight int) string {
	inner := width
	out := []string{
		fmt.Sprintf("+-%s-+", strings.Repeat("-", inner)),
		fmt.Sprintf("| %s |", padRight(colorize(ansiBold, title), inner)),
		fmt.Sprintf("+-%s-+", strings.Repeat("-", inner)),
	}
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		for _, wrapped := range wrapLine(line, inner) {
			body = append(body, wrapped)
		}
	}
	if bodyHeight >= 0 {
		if len(body) > bodyHeight {
			body = body[:bodyHeight]
		}
		for len(body) < bodyHeight {
			body = append(body, "")
		}
	}
	for _, line := range body {
		out = append(out, fmt.Sprintf("| %s |", padRight(line, inner)))
	}
	out = append(out, fmt.Sprintf("+-%s-+", strings.Repeat("-", inner)))
	return strings.Join(out, "\n")
}

func barLine(view string, current int, total int) string {
	return fmt.Sprintf("HP [%s] %d/%d", view, current, total)
}

type point struct {
	X int
	Y int
}

func drawConnector(canvas [][]rune, from point, to point) {
	startX := from.X + 3
	endX := to.X - 1
	if endX < startX {
		return
	}
	if from.Y == to.Y {
		for x := startX; x <= endX; x++ {
			drawRune(canvas, x, from.Y, '-')
		}
		return
	}

	midX := startX + (endX-startX)/2
	for x := startX; x < midX; x++ {
		drawRune(canvas, x, from.Y, '-')
	}
	for x := midX + 1; x <= endX; x++ {
		drawRune(canvas, x, to.Y, '-')
	}

	if from.Y < to.Y {
		drawRune(canvas, midX, (from.Y+to.Y)/2, '\\')
	} else {
		drawRune(canvas, midX, (from.Y+to.Y)/2, '/')
	}
}

func drawText(canvas [][]rune, x int, y int, text string) {
	runes := []rune(text)
	for i, r := range runes {
		drawRune(canvas, x+i, y, r)
	}
}

func drawRune(canvas [][]rune, x int, y int, r rune) {
	if y < 0 || y >= len(canvas) {
		return
	}
	if x < 0 || x >= len(canvas[y]) {
		return
	}
	if canvas[y][x] == ' ' || r == '[' || r == ']' || r == '@' || r == '>' || r == '?' || r == 'x' || r == '!' || r == 'B' {
		canvas[y][x] = r
	}
}

func laneRow(lane int) int {
	switch lane {
	case 0:
		return 0
	case 2:
		return 8
	default:
		return 4
	}
}

func nodeGlyph(node app.NodeMapNode, focusedID string) string {
	switch {
	case node.ID == focusedID:
		return "[>]"
	case node.Current:
		return "[@]"
	case !node.Visible:
		return "[?]"
	case node.Failed:
		return "[!]"
	case node.Resolved:
		return "[x]"
	case node.Type == "Standard":
		return "[s]"
	case node.Type == "Corrupted":
		return "[c]"
	case node.Type == "Boss":
		return "[b]"
	case node.Type == "Start":
		return "[@]"
	default:
		return "[ ]"
	}
}

func findNode(view app.NodeMapView, nodeID string) *app.NodeMapNode {
	for i := range view.Nodes {
		if view.Nodes[i].ID == nodeID {
			return &view.Nodes[i]
		}
	}
	return nil
}

func focusedNodeID(scene app.Scene, selectedIndex int, fallback string) string {
	choices := enabledChoices(scene)
	if selectedIndex >= 0 && selectedIndex < len(choices) {
		choiceID := choices[selectedIndex].ID
		if strings.HasPrefix(choiceID, "node:") {
			return strings.TrimPrefix(choiceID, "node:")
		}
	}
	return fallback
}

func enabledChoices(scene app.Scene) []app.Choice {
	out := make([]app.Choice, 0, len(scene.Choices))
	for _, choice := range scene.Choices {
		if choice.Enabled {
			out = append(out, choice)
		}
	}
	return out
}

func selectedChoice(scene app.Scene, index int) (app.Choice, bool) {
	choices := enabledChoices(scene)
	if len(choices) == 0 || index < 0 || index >= len(choices) {
		return app.Choice{}, false
	}
	return choices[index], true
}

func clampSelection(index int, scene app.Scene) int {
	count := len(enabledChoices(scene))
	if count == 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= count {
		return count - 1
	}
	return index
}

func previousChoice(scene app.Scene, index int) int {
	count := len(enabledChoices(scene))
	if count == 0 {
		return 0
	}
	if index <= 0 {
		return count - 1
	}
	return index - 1
}

func nextChoice(scene app.Scene, index int) int {
	count := len(enabledChoices(scene))
	if count == 0 {
		return 0
	}
	if index >= count-1 {
		return 0
	}
	return index + 1
}

func (m model) panelWidth() int {
	if m.width <= 0 {
		return defaultPanelWidth
	}
	return clamp(m.width-4, minPanelWidth, maxPanelWidth)
}

func wrapLine(line string, width int) []string {
	if line == "" {
		return []string{""}
	}
	if visibleWidth(line) <= width {
		return []string{line}
	}

	parts := []string{}
	remaining := line
	for visibleWidth(remaining) > width {
		cut := width
		if cut > len([]rune(remaining)) {
			cut = len([]rune(remaining))
		}
		runes := []rune(remaining)
		snippet := string(runes[:cut])
		if idx := strings.LastIndex(snippet, " "); idx > 0 {
			cut = idx
		}
		parts = append(parts, strings.TrimSpace(string(runes[:cut])))
		remaining = strings.TrimSpace(string(runes[cut:]))
	}
	if remaining != "" {
		parts = append(parts, remaining)
	}
	return parts
}

func padRight(in string, width int) string {
	extra := width - visibleWidth(in)
	if extra <= 0 {
		return truncateVisible(in, width)
	}
	return in + strings.Repeat(" ", extra)
}

func centerLine(in string, width int) string {
	extra := width - visibleWidth(in)
	if extra <= 0 {
		return in
	}
	left := extra / 2
	right := extra - left
	return strings.Repeat(" ", left) + in + strings.Repeat(" ", right)
}

func truncateVisible(in string, width int) string {
	if visibleWidth(in) <= width {
		return in
	}
	var out strings.Builder
	visible := 0
	inEscape := false
	for _, r := range in {
		switch {
		case r == '\x1b':
			inEscape = true
			out.WriteRune(r)
		case inEscape:
			out.WriteRune(r)
			if r == 'm' {
				inEscape = false
			}
		default:
			if visible >= width {
				continue
			}
			out.WriteRune(r)
			visible++
		}
	}
	out.WriteString(ansiReset)
	return out.String()
}

func visibleWidth(in string) int {
	width := 0
	inEscape := false
	for _, r := range in {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		default:
			width++
		}
	}
	return width
}

func colorize(code string, text string) string {
	return code + text + ansiReset
}

func ratio(current int, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(current) / float64(total)
}

func clamp(value int, low int, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
