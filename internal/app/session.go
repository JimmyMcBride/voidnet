package app

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"voidnet/internal/audio"
	"voidnet/internal/content"
	"voidnet/internal/game"
	"voidnet/internal/meta"
)

type Choice struct {
	ID      string
	Label   string
	Enabled bool
	Details *ChoiceDetails
}

type ChoiceDetails struct {
	Title   string
	Preview string
	Lines   []string
}

type Event struct {
	Message string
	Cue     audio.Event
}

type Scene struct {
	Kind    string
	Title   string
	Lines   []string
	Choices []Choice
	Combat  *CombatView
	Merge   *MergeView
	NodeMap *NodeMapView
	WonRun  bool // set when Kind == "game_over" and the player won
}

type CombatView struct {
	NodeType       string
	Round          int
	PlayerTurn     bool
	LastResolution []string
	Player         CombatantView
	Enemy          CombatantView
}

type CombatantView struct {
	Label            string
	Trait            string
	IntegrityCurrent int
	IntegrityMax     int
	Statuses         []string
}

type MergeView struct {
	ForkName   string
	ResultName string
	Trait      string
	FocusStat  string
	StatBefore int
	StatAfter  int
	Healed     int
	Ability    string
	Daemon     CombatantView
}

type NodeMapView struct {
	CurrentNodeID string
	Nodes         []NodeMapNode
	Edges         []NodeMapEdge
}

type NodeMapNode struct {
	ID         string
	Label      string
	Type       string
	Depth      int
	Lane       int
	Visible    bool
	Resolved   bool
	Failed     bool
	Selectable bool
	Current    bool
}

type NodeMapEdge struct {
	From string
	To   string
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
			Title: fmt.Sprintf("Voidnet | Seed %d", run.Seed),
			Lines: []string{
				"Select a starter daemon.",
				"Capture unlocks new starters and modifiers between runs.",
			},
			Choices: append(choices, Choice{ID: "quit", Label: "Quit", Enabled: true}),
		}

	case game.PhaseNodeSelect:
		active := s.engine.ActiveDaemon()
		lines := []string{
			fmt.Sprintf("Active daemon: %s", daemonSummary(active, s.registry)),
			fmt.Sprintf("Maintenance charge: %s", formatMaintenanceCharge(run.MaintenanceCharge)),
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
		canMerge := s.engine.CanBeginMerge()
		choices = append(choices, Choice{
			ID:      "merge",
			Label:   "Merge Daemons",
			Enabled: canMerge,
			Details: mergeNodeDetails(canMerge),
		})
		choices = append(choices, Choice{ID: "quit", Label: "Quit", Enabled: true})
		return Scene{
			Kind:    string(run.Phase),
			Title:   "Select Node",
			Lines:   lines,
			Choices: choices,
			NodeMap: buildNodeMapView(run, selectable),
		}

	case game.PhaseCombat:
		combat := run.Combat
		active := s.engine.ActiveDaemon()
		lines := []string{
			fmt.Sprintf("Enemy Daemon: %s [%s]", combatantName("Enemy", &combat.Enemy), combat.NodeType),
			fmt.Sprintf("Health: %d/%d | Status: %s", combat.Enemy.Integrity, combat.Enemy.MaxIntegrity, formatStatuses(combat.Enemy.Statuses, s.registry)),
			"",
			fmt.Sprintf("Your Daemon: %s [%s]", combatantName("Your", active), s.registry.Traits[active.TraitID].Name),
			fmt.Sprintf("Health: %d/%d | Status: %s", active.Integrity, active.MaxIntegrity, formatStatuses(active.Statuses, s.registry)),
			fmt.Sprintf("Round: %d", combat.Round),
		}
		if len(combat.LastLog) > 0 {
			lines = append(lines, "", "Last resolution:")
			lines = append(lines, combat.LastLog...)
		}
		choices := make([]Choice, 0, len(active.Abilities)+3)
		for i, ability := range active.Abilities {
			choices = append(choices, Choice{
				ID:      "ability:" + strconv.Itoa(i),
				Label:   abilityLabel(ability, s.registry),
				Enabled: true,
				Details: abilityDetails(ability, s.registry),
			})
		}
		choices = append(choices,
			Choice{ID: "isolate", Label: "Isolate", Enabled: true, Details: isolateDetails()},
			Choice{ID: "inspect", Label: "Inspect", Enabled: true, Details: inspectDetails()},
			Choice{ID: "quit", Label: "Quit", Enabled: true},
		)
		return Scene{
			Kind:    string(run.Phase),
			Title:   "Combat",
			Lines:   lines,
			Choices: choices,
			Combat: &CombatView{
				NodeType:       string(combat.NodeType),
				Round:          combat.Round,
				PlayerTurn:     s.engine.IsPlayerTurn(),
				LastResolution: append([]string(nil), combat.LastLog...),
				Player: CombatantView{
					Label:            combatantName("Your", active),
					Trait:            s.registry.Traits[active.TraitID].Name,
					IntegrityCurrent: active.Integrity,
					IntegrityMax:     active.MaxIntegrity,
					Statuses:         formatStatusesList(active.Statuses, s.registry),
				},
				Enemy: CombatantView{
					Label:            combatantName("Enemy", &combat.Enemy),
					Trait:            s.registry.Traits[combat.Enemy.TraitID].Name,
					IntegrityCurrent: combat.Enemy.Integrity,
					IntegrityMax:     combat.Enemy.MaxIntegrity,
					Statuses:         formatStatusesList(combat.Enemy.Statuses, s.registry),
				},
			},
		}

	case game.PhaseInspect:
		active := s.engine.ActiveDaemon()
		enemy := run.Combat.Enemy
		lines := append([]string{}, inspectCombatantLines("Your", active, s.registry)...)
		lines = append(lines, "")
		lines = append(lines, inspectCombatantLines("Enemy", &enemy, s.registry)...)
		lines = append(lines,
			"",
			"Stat key:",
			"HP Health: how much damage a daemon can take before crashing.",
			"SPD Speed: who acts first each round.",
			"STB Stability: resists hostile effects and lowers isolation chance against this daemon.",
			"",
			"Status reference:",
		)
		lines = append(lines, statusReferenceLines(s.registry)...)
		return Scene{
			Kind:    string(run.Phase),
			Title:   "Inspect",
			Lines:   lines,
			Choices: []Choice{{ID: "inspect:back", Label: "Back", Enabled: true}, {ID: "quit", Label: "Quit", Enabled: true}},
		}

	case game.PhaseReplace:
		lines := []string{
			fmt.Sprintf("Captured: %s", daemonSummary(run.PendingCapture, s.registry)),
			"Roster is full. Choose a daemon to replace or discard the capture.",
		}
		choices := []Choice{}
		for i, daemon := range run.Roster {
			choices = append(choices, Choice{
				ID:      "replace:" + strconv.Itoa(i),
				Label:   fmt.Sprintf("Replace %s", daemonSummary(&daemon, s.registry)),
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

	case game.PhaseMaintenance:
		active := s.engine.ActiveDaemon()
		lines := []string{
			"Choose repair or fortify, then select any daemon in your roster as the target.",
			fmt.Sprintf("Active daemon: %s", daemonSummary(active, s.registry)),
			fmt.Sprintf("Maintenance charge: %s", formatMaintenanceCharge(run.MaintenanceCharge)),
			"",
			"Roster telemetry:",
		}
		for i, daemon := range run.Roster {
			prefix := "  "
			if i == run.ActiveIndex {
				prefix = "* "
			}
			lines = append(lines, prefix+daemonSummary(&daemon, s.registry))
		}
		hasCharge := run.MaintenanceCharge > 0
		choices := []Choice{
			{ID: "maintenance:repair", Label: "Repair Daemon", Enabled: hasCharge, Details: maintenanceRepairDetails(hasCharge)},
			{ID: "maintenance:fortify", Label: "Fortify Daemon", Enabled: hasCharge, Details: maintenanceFortifyDetails(hasCharge)},
			{ID: "view:nodes", Label: "Back to Network Map", Enabled: true},
			{ID: "quit", Label: "Quit", Enabled: true},
		}
		return Scene{Kind: string(run.Phase), Title: "Maintenance", Lines: lines, Choices: choices}

	case game.PhaseSelectActive:
		title := "Select Active Daemon"
		lines := []string{"Select the next active daemon."}
		disableCurrent := false
		allowBackToNode := false
		allowBackToBase := false
		if run.SelectActiveMode == game.SelectActiveRotateLead {
			title = "Rotate Lead"
			lines = []string{"Select a different daemon to rotate into the lead."}
			disableCurrent = true
			allowBackToNode = true
		} else if run.SelectActiveMode == game.SelectActiveRepair {
			title = "Repair Target"
			lines = []string{"Select a daemon to repair."}
		} else if run.SelectActiveMode == game.SelectActiveFortify {
			title = "Fortify Target"
			lines = []string{"Select a daemon to fortify."}
		} else if run.SelectActiveMode == game.SelectActiveMergeBase {
			title = "Select Merge Base"
			lines = []string{"Select the base daemon that will absorb a fork."}
			allowBackToNode = true
		} else if run.SelectActiveMode == game.SelectActiveMergeFork {
			title = "Select Merge Fork"
			lines = []string{"Select the fork daemon to be absorbed into the base daemon."}
			allowBackToBase = true
		}
		choices := []Choice{}
		for i, daemon := range run.Roster {
			enabled := !disableCurrent || i != run.ActiveIndex
			switch run.SelectActiveMode {
			case game.SelectActiveMergeBase:
				enabled = s.engine.CanMergeAsBase(i)
			case game.SelectActiveMergeFork:
				enabled = s.engine.CanMergeAsFork(i)
			}
			choices = append(choices, Choice{
				ID:      "active:" + strconv.Itoa(i),
				Label:   daemonSummary(&daemon, s.registry),
				Enabled: enabled,
			})
		}
		if run.SelectActiveMode == game.SelectActiveRotateLead {
			choices = append(choices, Choice{ID: "rotate:back", Label: "Back to Node Map", Enabled: true})
		}
		if run.SelectActiveMode == game.SelectActiveRepair || run.SelectActiveMode == game.SelectActiveFortify {
			choices = append(choices, Choice{ID: "maintenance:back", Label: "Back to Maintenance", Enabled: true})
		}
		if allowBackToNode && run.SelectActiveMode == game.SelectActiveMergeBase {
			choices = append(choices, Choice{ID: "merge:back", Label: "Back to Node Map", Enabled: true})
		}
		if allowBackToBase {
			choices = append(choices, Choice{ID: "merge:fork_back", Label: "Back to Base Selection", Enabled: true})
		}
		choices = append(choices, Choice{ID: "quit", Label: "Quit", Enabled: true})
		return Scene{Kind: string(run.Phase), Title: title, Lines: lines, Choices: choices}

	case game.PhaseMergeConfirm:
		base := &run.Roster[run.PendingMergeBase]
		fork := &run.Roster[run.PendingMergeFork]
		focusStat, beforeValue, afterValue := mergePreviewStatDelta(base, fork, s.registry)
		healAmount := mergePreviewHeal(base, fork, s.registry)
		lines := []string{
			fmt.Sprintf("Base: %s", daemonSummary(base, s.registry)),
			fmt.Sprintf("Fork: %s", daemonSummary(fork, s.registry)),
			"",
			fmt.Sprintf("Result: %s", mergedNamePreview(base)),
			fmt.Sprintf("Trait retained: %s", s.registry.Traits[base.TraitID].Name),
			fmt.Sprintf("%s boost: %d -> %d", focusStat, beforeValue, afterValue),
			fmt.Sprintf("Health restore: +%d", healAmount),
			fmt.Sprintf("Slot 3 gain: %s", mergePreviewAbility(fork, s.registry)),
			"",
			fmt.Sprintf("%s will be consumed.", game.DaemonDisplayName(fork)),
		}
		return Scene{
			Kind:  string(run.Phase),
			Title: "Merge Confirm",
			Lines: lines,
			Choices: []Choice{
				{ID: "merge:confirm", Label: "Confirm Merge", Enabled: true},
				{ID: "merge:confirm_back", Label: "Back to Fork Selection", Enabled: true},
				{ID: "quit", Label: "Quit", Enabled: true},
			},
		}

	case game.PhaseMergeResult:
		merge := run.PendingMergeResult
		if merge == nil || merge.RosterIndex < 0 || merge.RosterIndex >= len(run.Roster) {
			return Scene{
				Kind:    string(run.Phase),
				Title:   "Merge Complete",
				Lines:   []string{"Merge completed."},
				Choices: []Choice{{ID: "continue", Label: "Back to Node Map", Enabled: true}, {ID: "quit", Label: "Quit", Enabled: true}},
			}
		}
		result := &run.Roster[merge.RosterIndex]
		lines := []string{
			fmt.Sprintf("Base seed: %s", merge.BaseSeedName),
			fmt.Sprintf("Fork consumed: %s", merge.ForkName),
			fmt.Sprintf("Result daemon: %s", merge.ResultName),
		}
		return Scene{
			Kind:  string(run.Phase),
			Title: "Merge Complete",
			Lines: lines,
			Choices: []Choice{
				{ID: "continue", Label: "Back to Node Map", Enabled: true},
				{ID: "quit", Label: "Quit", Enabled: true},
			},
			Merge: &MergeView{
				ForkName:   merge.ForkName,
				ResultName: merge.ResultName,
				Trait:      s.registry.Traits[merge.TraitID].Name,
				FocusStat:  merge.FocusStat,
				StatBefore: merge.StatBefore,
				StatAfter:  merge.StatAfter,
				Healed:     merge.Healed,
				Ability:    abilityLabel(merge.Ability, s.registry),
				Daemon: CombatantView{
					Label:            game.DaemonDisplayName(result),
					Trait:            s.registry.Traits[result.TraitID].Name,
					IntegrityCurrent: result.Integrity,
					IntegrityMax:     result.MaxIntegrity,
					Statuses:         formatStatusesList(result.Statuses, s.registry),
				},
			},
		}

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
		return Scene{Kind: string(run.Phase), Title: "Run Summary", Lines: lines, Choices: choices, WonRun: run.Won}
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
	case choiceID == "view:maintenance":
		lines, err = s.engine.OpenMaintenance()
	case choiceID == "view:nodes":
		lines, err = s.engine.OpenNodeMap()
	case choiceID == "merge":
		lines, err = s.engine.BeginMerge()
	case choiceID == "merge:back", choiceID == "merge:fork_back":
		lines, err = s.engine.CancelMergeSelection()
	case choiceID == "merge:confirm_back":
		lines, err = s.engine.CancelMergeConfirm()
	case choiceID == "merge:confirm":
		lines, err = s.engine.ConfirmMerge()
	case choiceID == "rotate":
		lines, err = s.engine.BeginRotateLead()
	case choiceID == "rotate:back":
		lines, err = s.engine.CancelRotateLead()
	case choiceID == "maintenance:back":
		lines, err = s.engine.CancelMaintenanceTargeting()
	case strings.HasPrefix(choiceID, "maintenance:"):
		lines, err = s.engine.ChooseMaintenance(strings.TrimPrefix(choiceID, "maintenance:"))
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

	return s.completeAction(lines)
}

func (s *Session) AdvanceEnemyTurn() (Scene, []Event, error) {
	lines, err := s.engine.AdvanceEnemyTurn()
	if err != nil {
		return s.Snapshot(), nil, err
	}

	return s.completeAction(lines)
}

func (s *Session) completeAction(lines []string) (Scene, []Event, error) {

	if s.engine.MetaDirty {
		if saveErr := s.store.Save(s.metaState); saveErr != nil {
			return s.Snapshot(), nil, saveErr
		}
		s.engine.MetaDirty = false
	}

	events := make([]Event, 0, len(lines))
	for _, line := range lines {
		events = append(events, Event{
			Message: line,
			Cue:     cueForLine(line),
		})
	}
	s.lastEvents = events
	return s.Snapshot(), events, nil
}

func cueForLine(line string) audio.Event {
	switch {
	case strings.HasPrefix(line, "Started a new run with seed "):
		return audio.EventSystemBoot
	case strings.HasPrefix(line, "Entered "):
		return audio.EventScan
	case strings.HasPrefix(line, "Encountered "):
		return audio.EventDaemonAppears
	case line == "Inspecting combat state.":
		return audio.EventScan
	case line == "Isolation successful.",
		strings.Contains(line, " joined the roster."),
		strings.Contains(line, " is ready to join the roster."):
		return audio.EventDaemonCaptured
	case strings.HasPrefix(line, "Isolation failed."):
		return audio.EventHackFail
	case strings.Contains(line, " backfired for "):
		return audio.EventBackfire
	case strings.HasSuffix(line, " crashed.") || line == "Active daemon crashed.":
		return audio.EventCrash
	case strings.Contains(line, " restored ") || strings.Contains(line, " gained Stabilized.") || strings.Contains(line, " cleared "):
		return audio.EventPatchRestore
	case strings.Contains(line, " is now Corrupted.") || strings.Contains(line, " suffered ") && strings.Contains(line, " from Leaking."):
		return audio.EventCorruptionBurst
	case strings.Contains(line, " is now Delayed."):
		return audio.EventGlitchStinger
	case strings.Contains(line, " lost the turn to Delayed."):
		return audio.EventGlitchStinger
	case strings.Contains(line, " took ") && strings.HasSuffix(line, " damage."):
		return audio.EventHackSuccess
	default:
		return ""
	}
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

func daemonSummary(d *game.Daemon, registry *content.Registry) string {
	if d == nil {
		return "none"
	}
	trait := registry.Traits[d.TraitID]
	return fmt.Sprintf("%s %d/%d HP | SPD %d | STB %d | %s", game.DaemonDisplayName(d), d.Integrity, d.MaxIntegrity, d.Speed, d.Stability, trait.Name)
}

func inspectCombatantLines(side string, d *game.Daemon, registry *content.Registry) []string {
	if d == nil {
		return []string{side + " daemon unavailable."}
	}

	trait := registry.Traits[d.TraitID]
	abilityLabels := make([]string, 0, len(d.Abilities))
	for _, ability := range d.Abilities {
		abilityLabels = append(abilityLabels, abilityLabel(ability, registry))
	}
	lines := []string{
		fmt.Sprintf("%s %s", side, daemonSummary(d, registry)),
		fmt.Sprintf("Abilities: %s", strings.Join(abilityLabels, ", ")),
		fmt.Sprintf("Trait: %s - %s", trait.Name, traitEffectSummary(trait)),
	}

	statusLines := activeStatusLines(d.Statuses, registry)
	if len(statusLines) == 1 {
		lines = append(lines, "Active status: "+statusLines[0])
	} else {
		lines = append(lines, "Active statuses:")
		for _, line := range statusLines {
			lines = append(lines, "  "+line)
		}
	}

	return lines
}

func abilityLabel(ability game.Ability, registry *content.Registry) string {
	effect := registry.Effects[ability.EffectID]
	modifier := registry.Modifiers[ability.ModifierID]
	return fmt.Sprintf("%s + %s", effect.Name, modifier.Name)
}

func abilityDetails(ability game.Ability, registry *content.Registry) *ChoiceDetails {
	effect := registry.Effects[ability.EffectID]
	modifier := registry.Modifiers[ability.ModifierID]

	previewParts := []string{strings.Title(effect.Kind)}
	if effect.Accuracy > 0 {
		previewParts = append(previewParts, fmt.Sprintf("%d%% base accuracy", effect.Accuracy))
	}
	if effect.Power > 0 {
		previewParts = append(previewParts, fmt.Sprintf("%d base power", effect.Power))
	}

	lines := []string{
		fmt.Sprintf("Effect: %s", effect.Name),
		fmt.Sprintf("Type: %s", strings.Title(effect.Kind)),
	}
	if effect.Accuracy > 0 {
		lines = append(lines, fmt.Sprintf("Base accuracy: %d%%", effect.Accuracy))
	}
	if effect.Power > 0 {
		lines = append(lines, fmt.Sprintf("Base power: %d", effect.Power))
	}
	if effect.Kind != "heal" {
		lines = append(lines, "Hit chance is lower against targets with higher Stability.")
	}
	if effect.Status != "" {
		status := registry.Statuses[effect.Status]
		lines = append(lines, fmt.Sprintf("On hit: applies %s for %s (%s)", status.Name, turnsLabel(effect.StatusDuration), statusEffectSummary(status)))
	}
	if effect.CleanseNegative {
		lines = append(lines, "On use: clears one negative status")
	}
	if effect.ApplySelfStatus != "" {
		status := registry.Statuses[effect.ApplySelfStatus]
		lines = append(lines, fmt.Sprintf("On use: grants %s for %s (%s)", status.Name, turnsLabel(effect.ApplySelfDuration), statusEffectSummary(status)))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Modifier: %s", modifier.Name))
	lines = append(lines, fmt.Sprintf("Power multiplier: x%.2f", modifier.PowerMultiplier))
	if modifier.AccuracyDelta != 0 {
		lines = append(lines, fmt.Sprintf("Accuracy delta: %+d", modifier.AccuracyDelta))
	} else {
		lines = append(lines, "Accuracy delta: +0")
	}
	if modifier.SelfBackfireChance > 0 {
		lines = append(lines, fmt.Sprintf("Backfire: %d%% chance for %d self-damage", modifier.SelfBackfireChance, modifier.SelfBackfireDamage))
	}

	return &ChoiceDetails{
		Title:   abilityLabel(ability, registry),
		Preview: strings.Join(previewParts, " | "),
		Lines:   lines,
	}
}

func isolateDetails() *ChoiceDetails {
	return &ChoiceDetails{
		Title:   "Isolate",
		Preview: "Capture at 50% Health or lower. Stronger odds below 25%.",
		Lines: []string{
			"Attempt to capture the enemy without defeating it.",
			"",
			"Only available on non-boss encounters.",
			"Eligible at 50% Health or lower.",
			"Base chance: 50% from 25-50% Health.",
			"Base chance: 80% at 25% Health or lower.",
			"Corrupted adds +10%; other negative statuses add +5% each.",
			"Higher target Stability lowers the capture chance.",
			"Target Stability reduces chance by max(0, Stability/5 - 1).",
			"Final chance is clamped between 5% and 95%.",
		},
	}
}

func inspectDetails() *ChoiceDetails {
	return &ChoiceDetails{
		Title:   "Inspect",
		Preview: "Open a detailed readout for both daemons without spending a turn.",
		Lines: []string{
			"View both combatants in more detail.",
			"",
			"Shows traits, stats, and all ability labels.",
			"Does not spend a combat turn.",
			"Use Back to return to combat.",
		},
	}
}

func formatMaintenanceCharge(charge int) string {
	if charge > 0 {
		return fmt.Sprintf("%d/1 ready.", charge)
	}
	return "0/1 empty."
}

func maintenanceConsoleDetails(charge int) *ChoiceDetails {
	preview := "Open the maintenance console. Repair and Fortify spend a stored charge; you can bank up to one."
	lines := []string{
		"Review and spend your stored maintenance charge.",
		"",
		"Repair Active and Fortify Link each consume 1 charge.",
		"You can leave the console and come back before entering the next node.",
		"Charge cap: 1.",
	}
	if charge > 0 {
		lines = append(lines, "", "Current charge: ready.")
	} else {
		lines = append(lines, "", "Current charge: empty until you win another battle.")
	}
	return &ChoiceDetails{
		Title:   "Maintenance Console",
		Preview: preview,
		Lines:   lines,
	}
}

func maintenanceRepairDetails(enabled bool) *ChoiceDetails {
	preview := "Pick any daemon, then restore 30% max Health, minimum 10, and cleanse one negative status."
	lines := []string{
		"Choose a daemon, then perform a focused repair cycle.",
		"",
		"Cost: 1 maintenance charge.",
		"Restore: 30% of max Health.",
		"Minimum restore: 10.",
		"Also clears one negative status if present.",
		"Best when any roster member is damaged or corrupted.",
	}
	if !enabled {
		lines = append(lines, "", "Unavailable: no maintenance charge is stored.")
		preview = "Unavailable until you bank a maintenance charge from a battle win."
	}
	return &ChoiceDetails{
		Title:   "Repair Daemon",
		Preview: preview,
		Lines:   lines,
	}
}

func maintenanceFortifyDetails(enabled bool) *ChoiceDetails {
	preview := "Pick any daemon, then restore 10% max Health, minimum 4, and grant Stabilized for the next fight."
	lines := []string{
		"Choose a daemon, then apply a defensive hardening pass.",
		"",
		"Cost: 1 maintenance charge.",
		"Restore: 10% of max Health.",
		"Minimum restore: 4.",
		"Grants Stabilized(2).",
		"Best when you want to prep a specific daemon for the next node.",
	}
	if !enabled {
		lines = append(lines, "", "Unavailable: no maintenance charge is stored.")
		preview = "Unavailable until you bank a maintenance charge from a battle win."
	}
	return &ChoiceDetails{
		Title:   "Fortify Daemon",
		Preview: preview,
		Lines:   lines,
	}
}

func rotateLeadNodeDetails(enabled bool) *ChoiceDetails {
	lines := []string{
		"Hand the lead slot to a reserve daemon.",
		"",
		"Cost: free.",
		"After selection: the new active daemon restores 20% of max Health.",
		"Minimum restore: 6.",
		"Best when you want to preserve the current lead or pivot into a better matchup.",
	}
	preview := "Choose a reserve daemon; the new lead restores 20% max Health, minimum 6."
	if !enabled {
		lines = append(lines, "", "Unavailable: you do not have a reserve daemon yet.")
		preview = "Unavailable until you have at least one reserve daemon."
	}
	return &ChoiceDetails{
		Title:   "Rotate Lead",
		Preview: preview,
		Lines:   lines,
	}
}

func mergeNodeDetails(enabled bool) *ChoiceDetails {
	preview := "Pick a base and fork daemon. The fork is consumed and the base becomes +1 with a third skill."
	lines := []string{
		"Open the +1 merge flow from the node map.",
		"",
		"Cost: free.",
		"Choose a base daemon, then a fork daemon.",
		"Fork must be a different archetype and neither daemon can already be +1.",
		"Result: the base daemon keeps its trait and core kit, gains the fork special move as slot 3, heals 25% max Health (minimum 8), and gains +2 in the fork's focus stat.",
		"The new slot 3 modifier is rolled at confirm time and weighted toward modifiers already used by the fork.",
	}
	if !enabled {
		lines = append(lines, "", "Unavailable: you need two unmerged daemons of different archetypes.")
		preview = "Unavailable until your roster has an eligible base and fork pair."
	}
	return &ChoiceDetails{
		Title:   "Merge Daemons",
		Preview: preview,
		Lines:   lines,
	}
}

func mergePreviewAbility(fork *game.Daemon, registry *content.Registry) string {
	if fork == nil || len(fork.Abilities) < 2 {
		return "[//::??::\\\\]"
	}
	effect := registry.Effects[fork.Abilities[1].EffectID]
	return fmt.Sprintf("%s + [//::??::\\\\]", effect.Name)
}

func mergedNamePreview(base *game.Daemon) string {
	if base == nil {
		return ""
	}
	return fmt.Sprintf("%s +%d", base.Name, base.MergeLevel+1)
}

func mergePreviewStatDelta(base *game.Daemon, fork *game.Daemon, registry *content.Registry) (string, int, int) {
	if base == nil || fork == nil {
		return "Stat", 0, 0
	}
	switch registry.Archetypes[fork.ArchetypeID].MergeFocusStat {
	case "integrity":
		return "Max Health", base.MaxIntegrity, base.MaxIntegrity + 2
	case "stability":
		return "Stability", base.Stability, base.Stability + 2
	case "speed":
		return "Speed", base.Speed, base.Speed + 2
	default:
		return "Stat", 0, 0
	}
}

func mergePreviewHeal(base *game.Daemon, fork *game.Daemon, registry *content.Registry) int {
	if base == nil || fork == nil {
		return 0
	}
	maxIntegrity := base.MaxIntegrity
	if registry.Archetypes[fork.ArchetypeID].MergeFocusStat == "integrity" {
		maxIntegrity += 2
	}
	recovery := max(8, int(math.Ceil(float64(maxIntegrity)*0.25)))
	restored := min(maxIntegrity, base.Integrity+recovery) - base.Integrity
	if restored < 0 {
		return 0
	}
	return restored
}

func traitEffectSummary(trait content.TraitDef) string {
	parts := []string{}
	if trait.SpeedDelta != 0 {
		parts = append(parts, fmt.Sprintf("%+d speed", trait.SpeedDelta))
	}
	if trait.StabilityDelta != 0 {
		parts = append(parts, fmt.Sprintf("%+d stability", trait.StabilityDelta))
	}
	if trait.AccuracyDelta != 0 {
		parts = append(parts, fmt.Sprintf("%+d accuracy", trait.AccuracyDelta))
	}
	if trait.IncomingEffectResistance != 0 {
		parts = append(parts, fmt.Sprintf("%d hostile-effect resistance", trait.IncomingEffectResistance))
	}
	if trait.DeathDamage != 0 {
		parts = append(parts, fmt.Sprintf("deals %d damage on crash", trait.DeathDamage))
	}
	if trait.GlitchMin != 0 || trait.GlitchMax != 0 {
		parts = append(parts, fmt.Sprintf("random %+d to %+d action bonus", trait.GlitchMin, trait.GlitchMax))
	}
	if len(parts) == 0 {
		return trait.Description
	}
	return strings.Join(parts, ", ")
}

func statusEffectSummary(status content.StatusDef) string {
	if status.ID == "delayed" {
		return "skips next turn"
	}
	parts := []string{}
	if status.AccuracyDelta != 0 {
		parts = append(parts, fmt.Sprintf("%+d accuracy", status.AccuracyDelta))
	}
	if status.SpeedDelta != 0 {
		parts = append(parts, fmt.Sprintf("%+d speed", status.SpeedDelta))
	}
	if status.StabilityDelta != 0 {
		parts = append(parts, fmt.Sprintf("%+d stability", status.StabilityDelta))
	}
	if status.DOTDamage > 0 {
		parts = append(parts, fmt.Sprintf("%d damage at end of turn", status.DOTDamage))
	}
	if len(parts) == 0 {
		return "no direct stat change"
	}
	return strings.Join(parts, ", ")
}

func turnsLabel(turns int) string {
	if turns == 1 {
		return "1 turn"
	}
	return fmt.Sprintf("%d turns", turns)
}

func activeStatusLines(statuses map[string]int, registry *content.Registry) []string {
	if len(statuses) == 0 {
		return []string{"None"}
	}
	keys := make([]string, 0, len(statuses))
	for statusID := range statuses {
		keys = append(keys, statusID)
	}
	slices.Sort(keys)
	lines := make([]string, 0, len(statuses))
	for _, statusID := range keys {
		status := registry.Statuses[statusID]
		lines = append(lines, fmt.Sprintf("%s(%d): %s", status.Name, statuses[statusID], statusEffectSummary(status)))
	}
	return lines
}

func statusReferenceLines(registry *content.Registry) []string {
	lines := make([]string, 0, len(registry.Data.Statuses))
	for _, status := range registry.Data.Statuses {
		lines = append(lines, fmt.Sprintf("%s: %s", status.Name, statusEffectSummary(status)))
	}
	return lines
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

func formatStatusesList(statuses map[string]int, registry *content.Registry) []string {
	if len(statuses) == 0 {
		return []string{"None"}
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
	return parts
}

func combatantName(side string, daemon *game.Daemon) string {
	if daemon == nil {
		return side + " Daemon"
	}
	return side + " " + game.DaemonDisplayName(daemon)
}

func buildNodeMapView(run *game.RunState, selectable map[string]struct{}) *NodeMapView {
	depths := map[string]int{"start": 0}
	parents := map[string][]string{}
	for _, nodeID := range run.NodeOrder {
		node := run.Nodes[nodeID]
		if node == nil {
			continue
		}
		if nodeID == "n1" || nodeID == "n2" {
			parents[nodeID] = append(parents[nodeID], "start")
		}
		for _, child := range node.Children {
			parents[child] = append(parents[child], nodeID)
		}
	}

	var resolveDepth func(string) int
	resolveDepth = func(nodeID string) int {
		if depth, ok := depths[nodeID]; ok {
			return depth
		}
		best := 0
		for _, parentID := range parents[nodeID] {
			best = max(best, resolveDepth(parentID)+1)
		}
		depths[nodeID] = best
		return best
	}

	grouped := map[int][]string{}
	for _, nodeID := range run.NodeOrder {
		depth := resolveDepth(nodeID)
		grouped[depth] = append(grouped[depth], nodeID)
	}

	lanes := map[string]int{"start": 1}
	for depth, ids := range grouped {
		if depth == 0 {
			continue
		}
		switch len(ids) {
		case 1:
			lanes[ids[0]] = 1
		default:
			lanes[ids[0]] = 0
			if len(ids) > 1 {
				lanes[ids[1]] = 2
			}
		}
	}

	nodes := []NodeMapNode{{
		ID:      "start",
		Label:   "Entry Point",
		Type:    "Start",
		Depth:   0,
		Lane:    1,
		Visible: true,
		Current: run.PositionNodeID == "start",
	}}

	edges := []NodeMapEdge{}
	for _, nodeID := range run.NodeOrder {
		node := run.Nodes[nodeID]
		if node == nil {
			continue
		}
		nodes = append(nodes, NodeMapNode{
			ID:         node.ID,
			Label:      node.Label,
			Type:       string(node.Type),
			Depth:      depths[node.ID],
			Lane:       lanes[node.ID],
			Visible:    node.Visible,
			Resolved:   node.Resolved,
			Failed:     node.Failed,
			Selectable: containsChoice(selectable, node.ID),
			Current:    run.PositionNodeID == node.ID,
		})
		if node.ID == "n1" || node.ID == "n2" {
			edges = append(edges, NodeMapEdge{From: "start", To: node.ID})
		}
		for _, child := range node.Children {
			edges = append(edges, NodeMapEdge{From: node.ID, To: child})
		}
	}

	return &NodeMapView{
		CurrentNodeID: run.PositionNodeID,
		Nodes:         nodes,
		Edges:         edges,
	}
}

func containsChoice(selectable map[string]struct{}, nodeID string) bool {
	_, ok := selectable[nodeID]
	return ok
}
