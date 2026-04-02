package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	progress "charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"

	"voidnet/internal/app"
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
	playbackIntroDelay  = 140 * time.Millisecond
	playbackTurnDelay   = 180 * time.Millisecond
	playbackActionDelay = 140 * time.Millisecond
	playbackRollDelay   = 120 * time.Millisecond
	playbackImpactDelay = 180 * time.Millisecond
)

type model struct {
	session             *app.Session
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
}

type playbackTickMsg struct{}

func Run(session *app.Session) error {
	m := newModel(session)
	program := tea.NewProgram(
		m,
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err := program.Run()
	return err
}

func newModel(session *app.Session) model {
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
			case "enter", " ":
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
		case "enter", " ":
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
	v.WindowTitle = "System Breakers"
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

	cmds := m.handleSceneTransition(previous, scene, events, choiceID)
	if m.session.ShouldQuit() {
		cmds = append(cmds, func() tea.Msg { return tea.Quit() })
	}
	return cmds
}

func (m *model) handleSceneTransition(previous app.Scene, next app.Scene, events []app.Event, choiceID string) []tea.Cmd {
	lines := eventMessages(events)
	cmds := []tea.Cmd{}
	m.activeModal = nil

	switch {
	case isCombatEntry(previous, next):
		m.resetCombatLog()
		m.scene = next
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
		cmds = append(cmds, m.syncBars())
		if len(lines) > 0 {
			cmds = append(cmds, m.beginPlayback(next, buildPreludeBeats(lines)))
			return cmds
		}
		if cmd := m.maybeAdvanceEnemy(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return cmds
	case isCombatChoice(choiceID, previous):
		if len(lines) > 0 {
			cmds = append(cmds, m.beginPlayback(next, buildActionBeats(actorFromScene(previous), previous, lines)))
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
		m.playback = nil
		m.playbackLines = nil
		m.playbackActor = ""
		m.playbackImpact = false
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
	m.appendCombatLog(beat.lines)

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

	m.appendCombatLog(remainingPlaybackLines(m.playback))
	m.playback = nil
	m.playbackLines = nil
	m.playbackActor = ""
	m.playbackImpact = false

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
	lines := eventMessages(events)
	if len(lines) == 0 {
		m.scene = next
		m.selectedIndex = clampSelection(0, m.scene)
		m.syncBarWidth()
		return []tea.Cmd{m.syncBars()}
	}
	return []tea.Cmd{m.beginPlayback(next, buildActionBeats(actorEnemy, previous, lines))}
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
		sections = append(sections, renderMenu(enabledChoices(m.scene), m.selectedIndex, width, m.playback != nil))
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
		return append([]string{turnLine}, sideBySideLines(enemyLines, playerLines, width, 4)...)
	}

	return []string{
		turnLine,
		enemyLines[0],
		enemyLines[1],
		playerLines[0],
		playerLines[1],
	}
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

	preferredLogBody := max(5, min(10, wrappedLineCount(m.combatLogRows(width), width)+1))
	if m.height <= 0 {
		return combatLayoutMetrics{
			encounterBody: encounterBody,
			logBody:       preferredLogBody,
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

	logBody := preferredLogBody
	if encounterBody+logBody > availableBody {
		logBody = max(1, availableBody-encounterBody)
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
	logo := []string{
		colorize(ansiBold+ansiCyan, "   _____           __                 ____                 __                  "),
		colorize(ansiBold+ansiCyan, "  / ___/__  ______/ /____  ____ ___  / __ )_______  ____ _/ /_____  __________"),
		colorize(ansiBold+ansiCyan, "  \\__ \\/ / / ___/ __/ _ \\/ __ `__ \\/ __  / ___/ _ \\/ __ `/ //_/ _ \\/ ___/ ___/"),
		colorize(ansiBold+ansiCyan, " ___/ / / (__  ) /_/  __/ / / / / / /_/ / /  /  __/ /_/ / ,< /  __/ /  (__  ) "),
		colorize(ansiBold+ansiCyan, "/____/_/ /____/\\__/\\___/_/ /_/ /_/_____/ /   \\___/\\__,_/_/|_|\\___/_/  /____/  "),
		centerLine(colorize(ansiGreen, ":: "+strings.ToUpper(scene.Title)+" ::"), width),
	}
	return strings.Join(logo, "\n")
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

func (m model) controlsHint() string {
	if m.activeModal != nil {
		return fmt.Sprintf("controls: i/esc/q close detail | scene=%s", m.scene.Kind)
	}
	if m.playback != nil {
		return fmt.Sprintf("controls: gg/G/ctrl+u/ctrl+d log | enter/space fast-forward | q quit | scene=%s", m.scene.Kind)
	}
	if m.scene.Combat != nil {
		return fmt.Sprintf("controls: j/k menu | i detail | gg/G/ctrl+u/ctrl+d log | enter/space select | q quit | scene=%s", m.scene.Kind)
	}
	return fmt.Sprintf("controls: up/down or j/k | enter/space select | q quit | scene=%s", m.scene.Kind)
}

func renderMenu(choices []app.Choice, selectedIndex int, width int, locked bool) string {
	return panel("COMMAND DECK", renderMenuLines(choices, selectedIndex, locked), width)
}

func (m model) renderCombatMenuLines() []string {
	lines := renderMenuLines(enabledChoices(m.scene), m.selectedIndex, m.playback != nil)
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
		lines = append(lines, "", colorize(ansiYellow, "Sequence running. Enter/Space fast-forward."))
	}
	return lines
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

func buildPreludeBeats(lines []string) []playbackBeat {
	beats := make([]playbackBeat, 0, len(lines))
	for _, line := range lines {
		beats = append(beats, playbackBeat{
			lines: []string{line},
			delay: playbackIntroDelay,
		})
	}
	return beats
}

func buildActionBeats(actor string, scene app.Scene, lines []string) []playbackBeat {
	if actor == "" {
		return buildPreludeBeats(lines)
	}

	beats := []playbackBeat{{
		lines: []string{turnBanner(actor, scene)},
		delay: playbackTurnDelay,
		actor: actor,
	}}
	if len(lines) == 0 {
		return beats
	}

	if len(lines) == 1 {
		beats = append(beats, playbackBeat{
			lines:      append([]string(nil), lines...),
			delay:      playbackImpactDelay,
			actor:      actor,
			impact:     true,
			applyScene: true,
		})
		return beats
	}

	beats = append(beats, playbackBeat{
		lines: []string{lines[0]},
		delay: playbackActionDelay,
		actor: actor,
	})

	index := 1
	if index < len(lines) && isRollLine(lines[index]) {
		beats = append(beats, playbackBeat{
			lines: []string{lines[index]},
			delay: playbackRollDelay,
			actor: actor,
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
		})
		return beats
	}

	last := len(beats) - 1
	beats[last].impact = true
	beats[last].applyScene = true
	beats[last].delay = playbackImpactDelay
	return beats
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
	return strings.HasPrefix(line, "Success chance ")
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
