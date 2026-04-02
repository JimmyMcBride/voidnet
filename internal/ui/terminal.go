package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"voidnet/internal/app"
)

const panelWidth = 74

func Run(session *app.Session) error {
	input := os.Stdin
	restore, err := enableRawMode(input)
	if err != nil {
		return err
	}
	if restore != nil {
		defer restore()
	}

	selectedIndex := 0
	for !session.ShouldQuit() {
		scene := session.Snapshot()
		selectedIndex = clampSelection(selectedIndex, scene)
		render(scene, selectedIndex)

		if len(scene.Choices) == 0 {
			return nil
		}

		for {
			key, readErr := readKey(input)
			if readErr != nil {
				return readErr
			}

			switch key {
			case keyUp:
				selectedIndex = previousChoice(scene, selectedIndex)
				render(scene, selectedIndex)
			case keyDown:
				selectedIndex = nextChoice(scene, selectedIndex)
				render(scene, selectedIndex)
			case keySelect:
				choiceID := enabledChoices(scene)[selectedIndex].ID
				_, _, err = session.Apply(choiceID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
				}
				goto nextScene
			case keyQuit:
				_, _, err = session.Apply("quit")
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
				}
				goto nextScene
			}
		}
	nextScene:
	}
	return nil
}

func render(scene app.Scene, selectedIndex int) {
	fmt.Print("\033[H\033[2J")
	printLogo(scene)
	fmt.Println()
	printPanel(sceneHeader(scene), decorateLines(scene))
	fmt.Println()
	printMenu(enabledChoices(scene), selectedIndex)
	fmt.Println()
	printFooter(scene)
}

func printLogo(scene app.Scene) {
	logo := []string{
		"   _____           __                 ____                 __                  ",
		"  / ___/__  ______/ /____  ____ ___  / __ )_______  ____ _/ /_____  __________",
		"  \\__ \\ / / ___/ __/ _ \\/ __ `__ \\/ __  / ___/ _ \\/ __ `/ //_/ _ \\/ ___/ ___/",
		" ___/ // / (__  ) /_/  __/ / / / / / /_/ / /  /  __/ /_/ / ,< /  __/ /  (__  ) ",
		"/____//_/ /____/\\__/\\___/_/ /_/ /_/_____/_/   \\___/\\__,_/_/|_|\\___/_/  /____/  ",
	}
	for _, line := range logo {
		fmt.Println(line)
	}
	fmt.Println(centerLine(":: "+strings.ToUpper(scene.Title)+" ::", panelWidth))
}

func sceneHeader(scene app.Scene) string {
	switch scene.Kind {
	case "combat":
		return "ENGAGED ENCOUNTER"
	case "node_select":
		return "NETWORK MAP"
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
	lines := make([]string, 0, len(scene.Lines)+6)
	switch scene.Kind {
	case "combat":
		lines = append(lines, "[ combat telemetry ]")
	case "node_select":
		lines = append(lines, "[ branch scan ]")
	case "starter_select":
		lines = append(lines, "[ select an operator daemon ]")
	case "reward":
		lines = append(lines, "[ resolved payload ]")
	case "game_over":
		lines = append(lines, "[ archive summary ]")
	}
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
	case strings.HasPrefix(line, "Enemy Daemon:"):
		return ">>> " + line
	case strings.HasPrefix(line, "Your Daemon:"):
		return "<<< " + line
	case strings.HasPrefix(line, "Active daemon:"):
		return "::  " + line
	case strings.HasPrefix(line, "Visible network nodes:"):
		return "::  " + line
	case strings.HasPrefix(line, "Last resolution:"):
		return "::  " + line
	case strings.HasPrefix(line, "Round:"):
		return "::  " + line
	case strings.HasPrefix(line, "Integrity:"):
		return "::  " + line
	case strings.HasPrefix(line, "Seed:"):
		return "::  " + line
	case strings.HasPrefix(line, "Remaining daemons:"):
		return "::  " + line
	case strings.HasPrefix(line, "- "):
		return "   " + line
	case strings.HasPrefix(line, "["):
		return "::  " + line
	default:
		return "    " + line
	}
}

func printPanel(title string, lines []string) {
	fmt.Printf("+-%s-+\n", strings.Repeat("-", panelWidth))
	fmt.Printf("| %s |\n", padRight(title, panelWidth))
	fmt.Printf("+-%s-+\n", strings.Repeat("-", panelWidth))
	for _, line := range lines {
		for _, wrapped := range wrapLine(line, panelWidth) {
			fmt.Printf("| %s |\n", padRight(wrapped, panelWidth))
		}
	}
	fmt.Printf("+-%s-+\n", strings.Repeat("-", panelWidth))
}

func printMenu(choices []app.Choice, selectedIndex int) {
	fmt.Printf("+-%s-+\n", strings.Repeat("-", panelWidth))
	fmt.Printf("| %s |\n", padRight("COMMAND DECK", panelWidth))
	fmt.Printf("+-%s-+\n", strings.Repeat("-", panelWidth))
	for i, choice := range choices {
		cursor := "  "
		marker := "[ ]"
		if i == selectedIndex {
			cursor = ">>"
			marker = "[*]"
		}
		line := fmt.Sprintf("%s %s %s", cursor, marker, choice.Label)
		fmt.Printf("| %s |\n", padRight(line, panelWidth))
	}
	fmt.Printf("+-%s-+\n", strings.Repeat("-", panelWidth))
}

func printFooter(scene app.Scene) {
	footer := fmt.Sprintf("controls: up/down or j/k | enter/space select | q quit | scene=%s", scene.Kind)
	fmt.Println(centerLine(footer, panelWidth+4))
}

type keyAction int

const (
	keyIgnore keyAction = iota
	keyUp
	keyDown
	keySelect
	keyQuit
)

func enabledChoices(scene app.Scene) []app.Choice {
	out := make([]app.Choice, 0, len(scene.Choices))
	for _, choice := range scene.Choices {
		if choice.Enabled {
			out = append(out, choice)
		}
	}
	return out
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

func readKey(r io.Reader) (keyAction, error) {
	buffer := make([]byte, 3)
	n, err := r.Read(buffer[:1])
	if err != nil {
		return keyIgnore, err
	}
	if n == 0 {
		return keyIgnore, nil
	}

	switch buffer[0] {
	case 'k', 'K':
		return keyUp, nil
	case 'j', 'J':
		return keyDown, nil
	case ' ', '\r', '\n':
		return keySelect, nil
	case 'q', 'Q':
		return keyQuit, nil
	case 0x1b:
		n, err = r.Read(buffer[1:3])
		if err != nil {
			return keyIgnore, err
		}
		if n >= 2 && buffer[1] == '[' {
			switch buffer[2] {
			case 'A':
				return keyUp, nil
			case 'B':
				return keyDown, nil
			}
		}
	}
	return keyIgnore, nil
}

func enableRawMode(file *os.File) (func(), error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Mode()&os.ModeCharDevice == 0 {
		return nil, nil
	}

	fd := file.Fd()
	original, err := getTermios(fd)
	if err != nil {
		return nil, err
	}
	raw := *original
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := setTermios(fd, &raw); err != nil {
		return nil, err
	}
	fmt.Print("\033[?25l")
	return func() {
		_ = setTermios(fd, original)
		fmt.Print("\033[?25h")
	}, nil
}

func getTermios(fd uintptr) (*syscall.Termios, error) {
	state := &syscall.Termios{}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(state)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	return state, nil
}

func setTermios(fd uintptr, state *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(state)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

func wrapLine(line string, width int) []string {
	if line == "" {
		return []string{""}
	}
	if len(line) <= width {
		return []string{line}
	}

	parts := []string{}
	remaining := line
	for len(remaining) > width {
		cut := strings.LastIndex(remaining[:width+1], " ")
		if cut <= 0 {
			cut = width
		}
		parts = append(parts, strings.TrimSpace(remaining[:cut]))
		remaining = strings.TrimSpace(remaining[cut:])
	}
	if remaining != "" {
		parts = append(parts, remaining)
	}
	return parts
}

func padRight(in string, width int) string {
	if len(in) >= width {
		return in[:width]
	}
	return in + strings.Repeat(" ", width-len(in))
}

func centerLine(in string, width int) string {
	if len(in) >= width {
		return in
	}
	left := (width - len(in)) / 2
	right := width - len(in) - left
	return strings.Repeat(" ", left) + in + strings.Repeat(" ", right)
}
