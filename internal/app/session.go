package app

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"voidnet/internal/content"
	"voidnet/internal/game"
	"voidnet/internal/meta"
)

type Choice struct {
	ID      string
	Label   string
	Enabled bool
}

type Event struct {
	Message string
}

type Scene struct {
	Kind    string
	Title   string
	Lines   []string
	Choices []Choice
}

type Session struct {
	registry   *content.Registry
	store      *meta.Store
	metaState  meta.State
	engine     *game.Engine
	seed       int64
	fixedSeed  bool
	quit       bool
	lastEvents []Event
}

func NewSession(registry *content.Registry, store *meta.Store, state meta.State, seed int64, fixedSeed bool) *Session {
	session := &Session{
		registry:  registry,
		store:     store,
		metaState: state,
		seed:      seed,
		fixedSeed: fixedSeed,
	}
	session.engine = game.New(registry, &session.metaState, seed, fixedSeed)
	return session
}

func (s *Session) Snapshot() Scene {
	run := s.engine.Run
	switch run.Phase {
	case game.PhaseStarterSelect:
		choices := []Choice{}
		for _, id := range s.engine.AvailableStarterIDs() {
			arch := s.registry.Archetypes[id]
			choices = append(choices, Choice{
				ID:      "starter:" + id,
				Label:   fmt.Sprintf("%s (%s)", arch.Name, arch.Role),
				Enabled: true,
			})
		}
		return Scene{
			Kind:  string(run.Phase),
			Title: fmt.Sprintf("System Breakers | Seed %d", run.Seed),
			Lines: []string{
				"Select a starter daemon.",
				"Capture unlocks new starters and modifiers between runs.",
			},
			Choices: append(choices, Choice{ID: "quit", Label: "Quit", Enabled: true}),
		}

	case game.PhaseNodeSelect:
		lines := []string{
			fmt.Sprintf("Active daemon: %s", daemonSummary(s.engine.ActiveDaemon().Name, s.engine.ActiveDaemon(), s.registry)),
			"",
			"Visible network nodes:",
		}
		choices := []Choice{}
		selectable := map[string]struct{}{}
		for _, id := range s.engine.NodeChoices() {
			selectable[id] = struct{}{}
		}
		for _, nodeID := range run.NodeOrder {
			node := run.Nodes[nodeID]
			switch {
			case node.Resolved && node.Failed:
				lines = append(lines, fmt.Sprintf("- %s (%s, failed)", node.Label, node.Type))
			case node.Resolved:
				lines = append(lines, fmt.Sprintf("- %s (%s, cleared)", node.Label, node.Type))
			case !node.Visible:
				lines = append(lines, "- ??? (Unknown)")
			default:
				lines = append(lines, fmt.Sprintf("- %s (%s)", node.Label, node.Type))
				if _, ok := selectable[nodeID]; ok {
					choices = append(choices, Choice{
						ID:      "node:" + nodeID,
						Label:   fmt.Sprintf("Enter %s", node.Label),
						Enabled: true,
					})
				}
			}
		}
		choices = append(choices, Choice{ID: "quit", Label: "Quit", Enabled: true})
		return Scene{Kind: string(run.Phase), Title: "Select Node", Lines: lines, Choices: choices}

	case game.PhaseCombat:
		combat := run.Combat
		active := s.engine.ActiveDaemon()
		lines := []string{
			fmt.Sprintf("Enemy Daemon: %s [%s]", combatantName("Enemy", &combat.Enemy), combat.NodeType),
			fmt.Sprintf("Integrity: %d/%d | Status: %s", combat.Enemy.Integrity, combat.Enemy.MaxIntegrity, formatStatuses(combat.Enemy.Statuses, s.registry)),
			"",
			fmt.Sprintf("Your Daemon: %s [%s]", combatantName("Your", active), s.registry.Traits[active.TraitID].Name),
			fmt.Sprintf("Integrity: %d/%d | Status: %s", active.Integrity, active.MaxIntegrity, formatStatuses(active.Statuses, s.registry)),
			fmt.Sprintf("Round: %d", combat.Round),
		}
		if len(combat.LastLog) > 0 {
			lines = append(lines, "", "Last resolution:")
			lines = append(lines, combat.LastLog...)
		}
		choices := []Choice{
			{ID: "ability:0", Label: abilityLabel(active.Abilities[0], s.registry), Enabled: true},
			{ID: "ability:1", Label: abilityLabel(active.Abilities[1], s.registry), Enabled: true},
			{ID: "isolate", Label: "Isolate", Enabled: true},
			{ID: "inspect", Label: "Inspect", Enabled: true},
			{ID: "quit", Label: "Quit", Enabled: true},
		}
		return Scene{Kind: string(run.Phase), Title: "Combat", Lines: lines, Choices: choices}

	case game.PhaseInspect:
		active := s.engine.ActiveDaemon()
		enemy := run.Combat.Enemy
		lines := []string{
			daemonSummary(combatantName("Your", active), active, s.registry),
			fmt.Sprintf("Abilities: %s, %s", abilityLabel(active.Abilities[0], s.registry), abilityLabel(active.Abilities[1], s.registry)),
			"",
			daemonSummary(combatantName("Enemy", &enemy), &enemy, s.registry),
			fmt.Sprintf("Abilities: %s, %s", abilityLabel(enemy.Abilities[0], s.registry), abilityLabel(enemy.Abilities[1], s.registry)),
		}
		return Scene{
			Kind:    string(run.Phase),
			Title:   "Inspect",
			Lines:   lines,
			Choices: []Choice{{ID: "inspect:back", Label: "Back", Enabled: true}, {ID: "quit", Label: "Quit", Enabled: true}},
		}

	case game.PhaseReplace:
		lines := []string{
			fmt.Sprintf("Captured: %s", daemonSummary(run.PendingCapture.Name, run.PendingCapture, s.registry)),
			"Roster is full. Choose a daemon to replace or discard the capture.",
		}
		choices := []Choice{}
		for i, daemon := range run.Roster {
			choices = append(choices, Choice{
				ID:      "replace:" + strconv.Itoa(i),
				Label:   fmt.Sprintf("Replace %s", daemonSummary(daemon.Name, &daemon, s.registry)),
				Enabled: true,
			})
		}
		choices = append(choices, Choice{ID: "replace:discard", Label: "Discard captured daemon", Enabled: true})
		choices = append(choices, Choice{ID: "quit", Label: "Quit", Enabled: true})
		return Scene{Kind: string(run.Phase), Title: "Roster Replacement", Lines: lines, Choices: choices}

	case game.PhaseReward:
		lines := append([]string(nil), run.PendingRewardLines...)
		if len(lines) == 0 {
			lines = []string{"Node complete."}
		}
		return Scene{
			Kind:    string(run.Phase),
			Title:   run.PendingRewardTitle,
			Lines:   lines,
			Choices: []Choice{{ID: "continue", Label: "Continue", Enabled: true}, {ID: "quit", Label: "Quit", Enabled: true}},
		}

	case game.PhaseSelectActive:
		lines := []string{"Select the next active daemon."}
		choices := []Choice{}
		for i, daemon := range run.Roster {
			choices = append(choices, Choice{
				ID:      "active:" + strconv.Itoa(i),
				Label:   daemonSummary(daemon.Name, &daemon, s.registry),
				Enabled: true,
			})
		}
		choices = append(choices, Choice{ID: "quit", Label: "Quit", Enabled: true})
		return Scene{Kind: string(run.Phase), Title: "Select Active Daemon", Lines: lines, Choices: choices}

	case game.PhaseGameOver:
		result := "Run failed."
		if run.Won {
			result = "Run won."
		}
		lines := []string{
			result,
			fmt.Sprintf("Seed: %d", run.Seed),
			fmt.Sprintf("Remaining daemons: %d", len(run.Roster)),
		}
		choices := []Choice{
			{ID: "continue", Label: "New Run", Enabled: true},
			{ID: "quit", Label: "Quit", Enabled: true},
		}
		return Scene{Kind: string(run.Phase), Title: "Run Summary", Lines: lines, Choices: choices}
	}

	return Scene{
		Kind:    "unknown",
		Title:   "Unknown State",
		Lines:   []string{"The session entered an unknown state."},
		Choices: []Choice{{ID: "quit", Label: "Quit", Enabled: true}},
	}
}

func (s *Session) Apply(choiceID string) (Scene, []Event, error) {
	if choiceID == "quit" {
		s.quit = true
		return s.Snapshot(), nil, nil
	}

	var lines []string
	var err error

	switch {
	case strings.HasPrefix(choiceID, "starter:"):
		lines, err = s.engine.ChooseStarter(strings.TrimPrefix(choiceID, "starter:"))
	case strings.HasPrefix(choiceID, "node:"):
		lines, err = s.engine.ChooseNode(strings.TrimPrefix(choiceID, "node:"))
	case strings.HasPrefix(choiceID, "ability:"):
		index, parseErr := strconv.Atoi(strings.TrimPrefix(choiceID, "ability:"))
		if parseErr != nil {
			return s.Snapshot(), nil, parseErr
		}
		lines, err = s.engine.UseAbility(index)
	case choiceID == "isolate":
		lines, err = s.engine.AttemptCapture()
	case choiceID == "inspect":
		lines, err = s.engine.ToggleInspect(true)
	case choiceID == "inspect:back":
		lines, err = s.engine.ToggleInspect(false)
	case strings.HasPrefix(choiceID, "replace:"):
		token := strings.TrimPrefix(choiceID, "replace:")
		if token == "discard" {
			lines, err = s.engine.ChooseReplacement(-1)
		} else {
			index, parseErr := strconv.Atoi(token)
			if parseErr != nil {
				return s.Snapshot(), nil, parseErr
			}
			lines, err = s.engine.ChooseReplacement(index)
		}
	case choiceID == "continue":
		lines, err = s.engine.Continue()
	case strings.HasPrefix(choiceID, "active:"):
		index, parseErr := strconv.Atoi(strings.TrimPrefix(choiceID, "active:"))
		if parseErr != nil {
			return s.Snapshot(), nil, parseErr
		}
		lines, err = s.engine.ChooseActive(index)
	default:
		err = fmt.Errorf("unknown choice %q", choiceID)
	}
	if err != nil {
		return s.Snapshot(), nil, err
	}

	if s.engine.MetaDirty {
		if saveErr := s.store.Save(s.metaState); saveErr != nil {
			return s.Snapshot(), nil, saveErr
		}
		s.engine.MetaDirty = false
	}

	events := make([]Event, 0, len(lines))
	for _, line := range lines {
		events = append(events, Event{Message: line})
	}
	s.lastEvents = events
	return s.Snapshot(), events, nil
}

func (s *Session) ShouldQuit() bool {
	return s.quit
}

func (s *Session) Seed() int64 {
	return s.seed
}

func NewSeed() int64 {
	return time.Now().UnixNano()
}

func daemonSummary(label string, d *game.Daemon, registry *content.Registry) string {
	if d == nil {
		return "none"
	}
	trait := registry.Traits[d.TraitID]
	return fmt.Sprintf("%s %d/%d INT | SPD %d | STB %d | %s", label, d.Integrity, d.MaxIntegrity, d.Speed, d.Stability, trait.Name)
}

func abilityLabel(ability game.Ability, registry *content.Registry) string {
	effect := registry.Effects[ability.EffectID]
	modifier := registry.Modifiers[ability.ModifierID]
	return fmt.Sprintf("%s + %s", effect.Name, modifier.Name)
}

func formatStatuses(statuses map[string]int, registry *content.Registry) string {
	if len(statuses) == 0 {
		return "None"
	}
	keys := make([]string, 0, len(statuses))
	for statusID := range statuses {
		keys = append(keys, statusID)
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(statuses))
	for _, statusID := range keys {
		turns := statuses[statusID]
		parts = append(parts, fmt.Sprintf("%s(%d)", registry.Statuses[statusID].Name, turns))
	}
	return strings.Join(parts, ", ")
}

func combatantName(side string, daemon *game.Daemon) string {
	if daemon == nil {
		return side + " Daemon"
	}
	return side + " " + daemon.Name
}
