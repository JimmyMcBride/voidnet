package audiolab

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"voidnet/internal/audio"
	"voidnet/internal/audio/music"
	"voidnet/internal/audio/sfx"
)

const (
	defaultPanelWidth = 84
	minPanelWidth     = 68
	maxPanelWidth     = 108

	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiBold   = "\033[1m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
)

type tabID int

const (
	tabSFX tabID = iota
	tabLoops
)

type tabState struct {
	query    string
	selected int
	scroll   int
}

type sfxItem struct {
	Preset sfx.Preset
	Seed   int64
}

type loopItem struct {
	Label string
	Loop  music.LoopID
	State music.ReactiveState
}

type model struct {
	audio      audio.Runtime
	width      int
	height     int
	tab        tabID
	tabs       map[tabID]*tabState
	sfxItems   []sfxItem
	loopItems  []loopItem
	searching  bool
	lastPlayed string
	lastError  string
	activeLoop string
}

func Run() (err error) {
	audioRuntime, err := audio.NewRuntime(audio.Options{})
	if err != nil {
		return err
	}
	if audioRuntime != nil {
		defer func() {
			err = errors.Join(err, audioRuntime.Close())
		}()
	}

	program := tea.NewProgram(
		newModel(audioRuntime),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err = program.Run()
	return err
}

func newModel(audioRuntime audio.Runtime) model {
	if audioRuntime == nil {
		audioRuntime = audio.NewNoopRuntime()
	}
	return model{
		audio:     audioRuntime,
		width:     defaultPanelWidth + 4,
		sfxItems:  buildSFXCatalog(),
		loopItems: buildLoopCatalog(),
		tabs: map[tabID]*tabState{
			tabSFX:   {},
			tabLoops: {},
		},
	}
}

func buildSFXCatalog() []sfxItem {
	presets := sfx.AllPresets()
	out := make([]sfxItem, 0, len(presets))
	for _, preset := range presets {
		out = append(out, sfxItem{Preset: preset, Seed: 1})
	}
	return out
}

func buildLoopCatalog() []loopItem {
	return []loopItem{
		{Label: "Boot", Loop: music.LoopBoot},
		{Label: "Ambient", Loop: music.LoopAmbient},
		{Label: "Victory", Loop: music.LoopVictory},
		{Label: "Defeat", Loop: music.LoopDefeat},
		{Label: "Battle / Standard / Tempo 0", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeStandard, Intensity: 0}},
		{Label: "Battle / Standard / Tempo 1", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeStandard, Intensity: 1}},
		{Label: "Battle / Standard / Tempo 2", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeStandard, Intensity: 2}},
		{Label: "Battle / Corrupted / Tempo 0", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeCorrupted, Intensity: 0}},
		{Label: "Battle / Corrupted / Tempo 1", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeCorrupted, Intensity: 1}},
		{Label: "Battle / Corrupted / Tempo 2", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeCorrupted, Intensity: 2}},
		{Label: "Battle / Boss / Tempo 0", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeBoss, Intensity: 0}},
		{Label: "Battle / Boss / Tempo 1", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeBoss, Intensity: 1}},
		{Label: "Battle / Boss / Tempo 2", Loop: music.LoopBattle, State: music.ReactiveState{BattleTheme: music.BattleThemeBoss, Intensity: 2}},
	}
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestWindowSize() }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
		if m.searching {
			m.handleSearchKey(msg)
			return m, nil
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "m":
			m.audio.SetMuted(!m.audio.Muted())
		case "tab":
			m.switchTab(1)
		case "shift+tab":
			m.switchTab(-1)
		case "/":
			m.searching = true
			m.lastError = ""
		case "down", "j":
			m.moveSelection(1)
		case "up", "k":
			m.moveSelection(-1)
		case "enter":
			m.playSelected()
		case "left":
			if m.tab == tabSFX {
				m.adjustSeed(-1)
			}
		case "right":
			if m.tab == tabSFX {
				m.adjustSeed(1)
			}
		case "s":
			m.stopLoop()
		}
	}
	return m, nil
}

func (m *model) handleSearchKey(msg tea.KeyPressMsg) {
	state := m.currentTab()
	switch msg.String() {
	case "enter", "esc":
		m.searching = false
	case "backspace":
		if len(state.query) > 0 {
			state.query = state.query[:len(state.query)-1]
			m.clampCurrentSelection()
		}
	default:
		if len(msg.Text) > 0 && msg.Text != "\n" && msg.Text != "\r" {
			state.query += msg.Text
			m.clampCurrentSelection()
		}
	}
}

func (m *model) switchTab(delta int) {
	if delta > 0 {
		if m.tab == tabLoops {
			m.tab = tabSFX
		} else {
			m.tab++
		}
	} else {
		if m.tab == tabSFX {
			m.tab = tabLoops
		} else {
			m.tab--
		}
	}
	m.searching = false
	m.clampCurrentSelection()
}

func (m *model) currentTab() *tabState {
	return m.tabs[m.tab]
}

func (m *model) moveSelection(delta int) {
	items := m.filteredIndices()
	if len(items) == 0 {
		return
	}
	state := m.currentTab()
	state.selected += delta
	if state.selected < 0 {
		state.selected = len(items) - 1
	}
	if state.selected >= len(items) {
		state.selected = 0
	}
	m.ensureSelectionVisible(len(items))
}

func (m *model) clampCurrentSelection() {
	items := m.filteredIndices()
	state := m.currentTab()
	if len(items) == 0 {
		state.selected = 0
		state.scroll = 0
		return
	}
	if state.selected < 0 {
		state.selected = 0
	}
	if state.selected >= len(items) {
		state.selected = len(items) - 1
	}
	m.ensureSelectionVisible(len(items))
}

func (m *model) adjustSeed(delta int64) {
	filtered := m.filteredIndices()
	if len(filtered) == 0 {
		return
	}
	state := m.currentTab()
	itemIndex := filtered[state.selected]
	seed := m.sfxItems[itemIndex].Seed + delta
	if seed < 1 {
		seed = 1
	}
	m.sfxItems[itemIndex].Seed = seed
}

func (m *model) stopLoop() {
	if m.audio == nil {
		return
	}
	m.audio.StopMusicLoop()
	m.activeLoop = ""
	m.lastPlayed = "stopped active loop"
}

func (m *model) playSelected() {
	m.lastError = ""
	if m.audio == nil {
		return
	}
	switch m.tab {
	case tabSFX:
		filtered := m.filteredIndices()
		if len(filtered) == 0 {
			return
		}
		item := m.sfxItems[filtered[m.currentTab().selected]]
		m.audio.Play(audio.Event(item.Preset), item.Seed)
		m.lastPlayed = fmt.Sprintf("played sfx %s (seed %d)", item.Preset, item.Seed)
	case tabLoops:
		filtered := m.filteredIndices()
		if len(filtered) == 0 {
			return
		}
		item := m.loopItems[filtered[m.currentTab().selected]]
		if err := m.audio.StartMusicLoop(item.Loop); err != nil {
			m.lastError = err.Error()
			return
		}
		if err := m.audio.SetMusicReactiveState(item.State); err != nil {
			m.lastError = err.Error()
			return
		}
		m.activeLoop = item.Label
		m.lastPlayed = "playing loop " + item.Label
	}
}

func (m model) filteredIndices() []int {
	query := strings.ToLower(strings.TrimSpace(m.currentTab().query))
	switch m.tab {
	case tabLoops:
		out := make([]int, 0, len(m.loopItems))
		for i, item := range m.loopItems {
			if query == "" || strings.Contains(strings.ToLower(item.Label), query) {
				out = append(out, i)
			}
		}
		return out
	default:
		out := make([]int, 0, len(m.sfxItems))
		for i, item := range m.sfxItems {
			if query == "" || strings.Contains(strings.ToLower(string(item.Preset)), query) {
				out = append(out, i)
			}
		}
		return out
	}
}

func (m *model) ensureSelectionVisible(total int) {
	state := m.currentTab()
	rows := m.listBodyHeight()
	if rows <= 0 {
		rows = 1
	}
	if state.scroll > max(0, total-rows) {
		state.scroll = max(0, total-rows)
	}
	if state.selected < state.scroll {
		state.scroll = state.selected
	}
	if state.selected >= state.scroll+rows {
		state.scroll = state.selected - rows + 1
	}
	if state.scroll < 0 {
		state.scroll = 0
	}
}

func (m model) View() tea.View {
	width := m.panelWidth()
	sections := []string{
		renderLogo(width),
		panel("AUDIO LAB", []string{
			m.renderTabs(width),
			m.renderSearchLine(width),
			m.renderStatusLine(width),
		}, width),
		panelFixed(m.listTitle(), m.renderListLines(width), width, m.listBodyHeight()),
		panelFixed("INSPECTOR", m.renderDetailLines(), width, m.detailBodyHeight()),
	}
	if m.lastError != "" {
		sections = append(sections, panel("AUDIO ERROR", []string{colorize(ansiRed, m.lastError)}, width))
	}
	sections = append(sections, colorize(ansiDim, m.footer()))
	return tea.NewView(strings.Join(sections, "\n\n"))
}

func (m model) renderTabs(width int) string {
	sfx := "[ SFX ]"
	loops := "[ LOOPS ]"
	if m.tab == tabSFX {
		sfx = colorize(ansiBold+ansiGreen, "[ SFX ]")
		loops = colorize(ansiDim, "[ LOOPS ]")
	} else {
		sfx = colorize(ansiDim, "[ SFX ]")
		loops = colorize(ansiBold+ansiGreen, "[ LOOPS ]")
	}
	return centerLine(sfx+"   "+loops, width)
}

func (m model) renderSearchLine(width int) string {
	label := "search"
	if m.searching {
		label = colorize(ansiBold+ansiYellow, "search")
	}
	query := m.currentTab().query
	if query == "" {
		query = colorize(ansiDim, "type / to filter the current tab")
	}
	return padRight(fmt.Sprintf("%s: %s", label, query), width)
}

func (m model) renderStatusLine(width int) string {
	parts := []string{
		"audio: " + m.audioStatus(),
	}
	if m.activeLoop != "" {
		parts = append(parts, "loop: "+m.activeLoop)
	}
	if m.lastPlayed != "" {
		parts = append(parts, m.lastPlayed)
	}
	return padRight(strings.Join(parts, colorize(ansiDim, " | ")), width)
}

func (m model) listTitle() string {
	if m.tab == tabLoops {
		return "MUSIC LOOPS"
	}
	return "SFX PRESETS"
}

func (m model) renderListLines(width int) []string {
	filtered := m.filteredIndices()
	if len(filtered) == 0 {
		return []string{colorize(ansiDim, "no matches")}
	}
	state := m.currentTab()
	rows := m.listBodyHeight()
	if rows <= 0 {
		rows = len(filtered)
	}
	start := min(state.scroll, max(0, len(filtered)-1))
	end := min(len(filtered), start+rows)
	lines := make([]string, 0, end-start+2)
	if start > 0 {
		lines = append(lines, colorize(ansiDim, "^^ more ^^"))
	}
	for i := start; i < end; i++ {
		lines = append(lines, m.renderRow(filtered[i], i == state.selected))
	}
	if end < len(filtered) {
		lines = append(lines, colorize(ansiDim, "vv more vv"))
	}
	return lines
}

func (m model) renderRow(idx int, selected bool) string {
	prefix := colorize(ansiDim, "[ ]")
	if selected {
		prefix = colorize(ansiBold+ansiGreen, "[>]")
	}
	switch m.tab {
	case tabLoops:
		item := m.loopItems[idx]
		line := item.Label
		if item.Label == m.activeLoop {
			line += colorize(ansiDim, "  // playing")
		}
		if selected {
			line = colorize(ansiBold+ansiCyan, line)
		}
		return prefix + " " + line
	default:
		item := m.sfxItems[idx]
		line := fmt.Sprintf("%s  seed=%d", item.Preset, item.Seed)
		if selected {
			line = colorize(ansiBold+ansiCyan, string(item.Preset)) + colorize(ansiDim, fmt.Sprintf("  seed=%d", item.Seed))
		}
		return prefix + " " + line
	}
}

func (m model) renderDetailLines() []string {
	filtered := m.filteredIndices()
	if len(filtered) == 0 {
		return []string{colorize(ansiDim, "nothing selected")}
	}
	index := filtered[m.currentTab().selected]
	if m.tab == tabLoops {
		return m.loopDetailLines(m.loopItems[index])
	}
	return m.sfxDetailLines(m.sfxItems[index])
}

func (m model) sfxDetailLines(item sfxItem) []string {
	params, err := sfx.GenerateParamsForDebug(item.Preset, item.Seed)
	if err != nil {
		return []string{colorize(ansiRed, err.Error())}
	}
	return []string{
		colorize(ansiBold+ansiCyan, string(item.Preset)),
		fmt.Sprintf("Seed: %d", item.Seed),
		fmt.Sprintf("Waveform: %s", params.WaveType),
		fmt.Sprintf("Duration: %.3fs", params.Duration),
		fmt.Sprintf("Sample rate: %d", params.SampleRate),
		fmt.Sprintf("Base frequency: %.1f Hz", params.BaseFreq),
		fmt.Sprintf("Volume: %.2f", params.Volume),
		fmt.Sprintf("Bit crush: %d", params.BitCrush),
		colorize(ansiDim, "left/right adjust seed • enter replay"),
	}
}

func (m model) loopDetailLines(item loopItem) []string {
	pattern, ok := music.PatternForState(item.Loop, item.State)
	if !ok {
		return []string{colorize(ansiRed, "unknown loop")}
	}
	lines := []string{
		colorize(ansiBold+ansiCyan, item.Label),
		fmt.Sprintf("Loop ID: %s", item.Loop),
		fmt.Sprintf("Reactive state: %s", formatReactiveState(item.State)),
		fmt.Sprintf("BPM: %.0f", pattern.BPM),
		fmt.Sprintf("Beats per loop: %d", pattern.BeatsPerLoop),
		fmt.Sprintf("Sample rate: %d", pattern.SampleRate),
		fmt.Sprintf("Tracks: %d", len(pattern.Tracks)),
	}
	for i, track := range pattern.Tracks {
		lines = append(lines, fmt.Sprintf("Track %d: %s gain=%.2f notes=%d", i+1, waveformLabel(track.Waveform), track.Gain, len(track.Notes)))
	}
	lines = append(lines, colorize(ansiDim, "enter restart loop • s stop"))
	return lines
}

func formatReactiveState(state music.ReactiveState) string {
	state = state
	parts := []string{}
	if state.BattleTheme != "" {
		parts = append(parts, "theme="+string(state.BattleTheme))
	}
	parts = append(parts, fmt.Sprintf("intensity=%d", state.Intensity))
	return strings.Join(parts, ", ")
}

func waveformLabel(w music.Waveform) string {
	switch w {
	case music.WaveSine:
		return "sine"
	case music.WaveSquare:
		return "square"
	case music.WaveSaw:
		return "saw"
	case music.WaveTriangle:
		return "triangle"
	default:
		return "unknown"
	}
}

func (m model) audioStatus() string {
	if m.audio == nil || !m.audio.Available() {
		return "unavailable"
	}
	if m.audio.Muted() {
		return "muted"
	}
	return "on"
}

func (m model) listBodyHeight() int {
	if m.height <= 0 {
		return 10
	}
	return max(6, min(16, m.height/3))
}

func (m model) detailBodyHeight() int {
	if m.height <= 0 {
		return 10
	}
	return max(8, min(18, m.height/3))
}

func (m model) footer() string {
	parts := []string{
		"tab switch",
		"/ search",
		"enter play",
	}
	if m.tab == tabSFX {
		parts = append(parts, "left/right seed")
	}
	parts = append(parts, "s stop loop", "m mute", "q quit")
	return "hint: " + strings.Join(parts, " | ")
}

func renderLogo(width int) string {
	screenWidth := width + 4
	logo := []string{
		renderLogoLine(" _    __      _     __           __ ", screenWidth, ".::", "::."),
		renderLogoLine("| |  / /___  (_)___/ /___  ___  / /_", screenWidth, "//:", ":\\\\"),
		renderLogoLine("| | / / __ \\/ / __  / __ \\/ _ \\/ __/", screenWidth, "[[ ", " ]]"),
		renderLogoLine("| |/ / /_/ / / /_/ / / / /  __/ /_  ", screenWidth, "\\\\:", "://"),
		renderLogoLine("|___/\\____/_/\\__,_/_/ /_/\\___/\\__/  ", screenWidth, "`::", "::'"),
		centerLine(colorize(ansiGreen, ":: AUDIO LAB ::"), screenWidth),
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

func (m model) panelWidth() int {
	if m.width <= 0 {
		return defaultPanelWidth
	}
	return clamp(m.width-4, minPanelWidth, maxPanelWidth)
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

func clamp(value int, lo int, hi int) int {
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
