package game

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"

	"voidnet/internal/content"
	"voidnet/internal/meta"
)

type Phase string

const (
	PhaseStarterSelect Phase = "starter_select"
	PhaseNodeSelect    Phase = "node_select"
	PhaseCombat        Phase = "combat"
	PhaseInspect       Phase = "inspect"
	PhaseReplace       Phase = "replace"
	PhaseReward        Phase = "reward"
	PhaseSelectActive  Phase = "select_active"
	PhaseGameOver      Phase = "game_over"
)

type NodeType string

const (
	NodeStandard  NodeType = "Standard"
	NodeCorrupted NodeType = "Corrupted"
	NodeBoss      NodeType = "Boss"
)

type Actor string

const (
	ActorPlayer Actor = "player"
	ActorEnemy  Actor = "enemy"
)

type Stats struct {
	Integrity int
	Speed     int
	Stability int
}

type Ability struct {
	EffectID   string
	ModifierID string
}

type Daemon struct {
	ID           string
	Name         string
	ArchetypeID  string
	MaxIntegrity int
	Integrity    int
	Speed        int
	Stability    int
	Abilities    []Ability
	TraitID      string
	Statuses     map[string]int
}

type Node struct {
	ID         string
	Label      string
	Type       NodeType
	Difficulty int
	Visible    bool
	Resolved   bool
	Failed     bool
	Children   []string
}

type CombatState struct {
	NodeID     string
	NodeType   NodeType
	Enemy      Daemon
	Queue      []Actor
	Round      int
	LastLog    []string
	CanCapture bool
}

type RunState struct {
	Seed               int64
	Phase              Phase
	PositionNodeID     string
	NodeOrder          []string
	Nodes              map[string]*Node
	Roster             []Daemon
	ActiveIndex        int
	Combat             *CombatState
	PendingCapture     *Daemon
	PendingRewardTitle string
	PendingRewardLines []string
	RecentUnlocks      []string
	Inspecting         bool
	Won                bool
	Lost               bool
}

type Engine struct {
	Content   *content.Registry
	Meta      *meta.State
	Run       *RunState
	MetaDirty bool

	rng          *rand.Rand
	fixedSeed    bool
	currentSeed  int64
	nextDaemonID int
}

func New(reg *content.Registry, state *meta.State, seed int64, fixedSeed bool) *Engine {
	engine := &Engine{
		Content:   reg,
		Meta:      state,
		fixedSeed: fixedSeed,
	}
	engine.Reset(seed)
	return engine
}

func (e *Engine) Reset(seed int64) {
	e.currentSeed = seed
	e.rng = rand.New(rand.NewSource(seed))
	e.nextDaemonID = 1
	e.Run = &RunState{
		Seed:           seed,
		Phase:          PhaseStarterSelect,
		PositionNodeID: "start",
		Nodes:          make(map[string]*Node),
		ActiveIndex:    -1,
	}
	e.buildGraph()
}

func (e *Engine) AvailableStarterIDs() []string {
	return e.Content.SortedStarterIDs(e.Meta.UnlockedStarters)
}

func (e *Engine) ChooseStarter(archetypeID string) ([]string, error) {
	if e.Run.Phase != PhaseStarterSelect {
		return nil, fmt.Errorf("starter selection is not active")
	}
	if _, ok := e.Content.Archetypes[archetypeID]; !ok {
		return nil, fmt.Errorf("unknown starter %q", archetypeID)
	}

	starter := e.generateDaemon(archetypeID, 0, true, "", Stats{})
	e.Run.Roster = []Daemon{starter}
	e.Run.ActiveIndex = 0
	e.Run.Phase = PhaseNodeSelect
	e.revealChildren("start")

	lines := []string{
		fmt.Sprintf("Boot sequence locked on %s.", starter.Name),
		fmt.Sprintf("%s enters the network with %d Integrity.", starter.Name, starter.Integrity),
	}

	lines = append(lines, e.applyUnlockTrigger("use_archetype:"+archetypeID)...)
	return lines, nil
}

func (e *Engine) ChooseNode(nodeID string) ([]string, error) {
	if e.Run.Phase != PhaseNodeSelect {
		return nil, fmt.Errorf("node selection is not active")
	}
	node, ok := e.Run.Nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("unknown node %q", nodeID)
	}
	if node.Resolved {
		return nil, fmt.Errorf("node %q already resolved", nodeID)
	}
	if !slices.Contains(e.currentChoicesForNodes(), nodeID) {
		return nil, fmt.Errorf("node %q is not reachable", nodeID)
	}

	prelude := []string{}
	if node.Type == NodeBoss {
		prelude = append(prelude, e.applyUnlockTrigger("first_boss_reached")...)
	}

	enemy := e.generateEnemyForNode(node)
	e.Run.Combat = &CombatState{
		NodeID:     nodeID,
		NodeType:   node.Type,
		Enemy:      enemy,
		Round:      1,
		CanCapture: node.Type != NodeBoss,
	}
	e.refreshQueue()
	e.Run.Phase = PhaseCombat

	lines := append(prelude,
		fmt.Sprintf("Entered %s.", node.Label),
		fmt.Sprintf("Encountered %s.", enemy.Name),
	)
	lines = append(lines, e.advanceEnemyTurns()...)
	e.setCombatLog(lines)
	return lines, nil
}

func (e *Engine) ToggleInspect(open bool) ([]string, error) {
	switch {
	case open && e.Run.Phase == PhaseCombat:
		e.Run.Inspecting = true
		e.Run.Phase = PhaseInspect
		return []string{"Inspecting combat state."}, nil
	case !open && e.Run.Phase == PhaseInspect:
		e.Run.Inspecting = false
		e.Run.Phase = PhaseCombat
		return nil, nil
	default:
		return nil, fmt.Errorf("inspect toggle is invalid in phase %s", e.Run.Phase)
	}
}

func (e *Engine) UseAbility(slot int) ([]string, error) {
	if e.Run.Phase != PhaseCombat {
		return nil, fmt.Errorf("combat is not active")
	}
	player := e.activeDaemon()
	if player == nil {
		return nil, fmt.Errorf("no active daemon")
	}
	if slot < 0 || slot >= len(player.Abilities) {
		return nil, fmt.Errorf("ability slot %d out of range", slot)
	}
	if !e.isPlayerTurn() {
		return nil, fmt.Errorf("it is not the player's turn")
	}

	ability := player.Abilities[slot]
	lines := e.resolveAbility(ActorPlayer, ability)
	lines = append(lines, e.advanceEnemyTurns()...)
	e.setCombatLog(lines)
	return lines, nil
}

func (e *Engine) AttemptCapture() ([]string, error) {
	if e.Run.Phase != PhaseCombat {
		return nil, fmt.Errorf("combat is not active")
	}
	if !e.isPlayerTurn() {
		return nil, fmt.Errorf("it is not the player's turn")
	}
	if !e.Run.Combat.CanCapture {
		return nil, fmt.Errorf("capture is not allowed in this encounter")
	}

	chance, eligible := e.captureChance()
	lines := []string{}
	if !eligible {
		lines = append(lines, "Isolation failed. Target Integrity is above 50%.")
	} else {
		roll := e.rng.Intn(100) + 1
		lines = append(lines, fmt.Sprintf("Isolation chance %d%%. Roll %d.", chance, roll))
		if roll <= chance {
			lines = append(lines, "Isolation successful.")
			captured := e.Run.Combat.Enemy
			e.finishCombatWin(true, &captured, lines)
			return lines, nil
		}
		lines = append(lines, "Isolation failed. The daemon resisted the breach.")
	}

	e.consumeTurn()
	lines = append(lines, e.postTurnResolution(ActorPlayer)...)
	lines = append(lines, e.advanceEnemyTurns()...)
	e.setCombatLog(lines)
	return lines, nil
}

func (e *Engine) ChooseReplacement(index int) ([]string, error) {
	if e.Run.Phase != PhaseReplace {
		return nil, fmt.Errorf("replacement choice is not active")
	}
	if e.Run.PendingCapture == nil {
		return nil, fmt.Errorf("no pending capture")
	}

	lines := []string{}
	switch {
	case index == -1:
		lines = append(lines, fmt.Sprintf("Discarded %s and kept the current roster.", e.Run.PendingCapture.Name))
	case index >= 0 && index < len(e.Run.Roster):
		old := e.Run.Roster[index]
		e.Run.Roster[index] = *e.Run.PendingCapture
		if e.Run.ActiveIndex == index {
			e.Run.ActiveIndex = index
		}
		lines = append(lines, fmt.Sprintf("Replaced %s with %s.", old.Name, e.Run.PendingCapture.Name))
	default:
		return nil, fmt.Errorf("replacement index %d out of range", index)
	}

	e.Run.PendingCapture = nil
	e.Run.Phase = PhaseReward
	e.Run.PendingRewardLines = append(e.Run.PendingRewardLines, lines...)
	return lines, nil
}

func (e *Engine) Continue() ([]string, error) {
	switch e.Run.Phase {
	case PhaseReward:
		if e.Run.Won {
			e.Run.Phase = PhaseGameOver
		} else if e.Run.ActiveIndex == -1 && len(e.Run.Roster) > 0 {
			e.Run.Phase = PhaseSelectActive
		} else {
			e.Run.Phase = PhaseNodeSelect
		}
		return nil, nil
	case PhaseGameOver:
		var nextSeed int64
		if e.fixedSeed {
			nextSeed = e.currentSeed
		} else {
			nextSeed = e.rng.Int63()
		}
		e.Reset(nextSeed)
		return []string{fmt.Sprintf("Started a new run with seed %d.", nextSeed)}, nil
	default:
		return nil, fmt.Errorf("continue is invalid in phase %s", e.Run.Phase)
	}
}

func (e *Engine) ChooseActive(index int) ([]string, error) {
	if e.Run.Phase != PhaseSelectActive {
		return nil, fmt.Errorf("active selection is not active")
	}
	if index < 0 || index >= len(e.Run.Roster) {
		return nil, fmt.Errorf("active index %d out of range", index)
	}
	e.Run.ActiveIndex = index
	e.Run.Phase = PhaseNodeSelect
	return []string{fmt.Sprintf("%s is now active.", e.Run.Roster[index].Name)}, nil
}

func (e *Engine) ActiveDaemon() *Daemon {
	return e.activeDaemon()
}

func (e *Engine) NodeChoices() []string {
	return e.currentChoicesForNodes()
}

func (e *Engine) captureChance() (int, bool) {
	enemy := &e.Run.Combat.Enemy
	if enemy.MaxIntegrity == 0 {
		return 0, false
	}

	ratio := float64(enemy.Integrity) / float64(enemy.MaxIntegrity)
	if ratio > 0.50 {
		return 0, false
	}

	chance := 40
	if ratio <= 0.25 {
		chance = 75
	}

	for statusID := range enemy.Statuses {
		if statusID == "corrupted" {
			chance += 10
			continue
		}
		def := e.Content.Statuses[statusID]
		if !def.IsPositive {
			chance += 5
		}
	}

	resistance := max(0, enemy.Stability/4-2)
	chance -= resistance
	return clamp(chance, 5, 95), true
}

func (e *Engine) IsPlayerTurn() bool {
	return e.isPlayerTurn()
}

func (e *Engine) buildGraph() {
	names := append([]string(nil), e.Content.Data.NodeNames...)
	e.rng.Shuffle(len(names), func(i, j int) {
		names[i], names[j] = names[j], names[i]
	})

	total := 3 + e.rng.Intn(3)
	makeNode := func(id, label string, nodeType NodeType, difficulty int, children ...string) {
		e.Run.Nodes[id] = &Node{
			ID:         id,
			Label:      label,
			Type:       nodeType,
			Difficulty: difficulty,
			Children:   children,
		}
		e.Run.NodeOrder = append(e.Run.NodeOrder, id)
	}

	switch total {
	case 3:
		makeNode("n1", names[0], randomNodeType(e.rng), 1+e.rng.Intn(2), "boss")
		makeNode("n2", names[1], randomNodeType(e.rng), 1+e.rng.Intn(2), "boss")
		makeNode("boss", names[2], NodeBoss, 3, nil...)
	case 4:
		makeNode("n1", names[0], randomNodeType(e.rng), 1+e.rng.Intn(2), "n3")
		makeNode("n2", names[1], randomNodeType(e.rng), 1+e.rng.Intn(2), "n3")
		makeNode("n3", names[2], randomNodeType(e.rng), 2+e.rng.Intn(2), "boss")
		makeNode("boss", names[3], NodeBoss, 4, nil...)
	default:
		makeNode("n1", names[0], randomNodeType(e.rng), 1+e.rng.Intn(2), "n3")
		makeNode("n2", names[1], randomNodeType(e.rng), 1+e.rng.Intn(2), "n4")
		makeNode("n3", names[2], randomNodeType(e.rng), 2+e.rng.Intn(2), "boss")
		makeNode("n4", names[3], randomNodeType(e.rng), 2+e.rng.Intn(2), "boss")
		makeNode("boss", names[4], NodeBoss, 4, nil...)
	}
}

func randomNodeType(rng *rand.Rand) NodeType {
	if rng.Intn(100) < 40 {
		return NodeCorrupted
	}
	return NodeStandard
}

func (e *Engine) currentChoicesForNodes() []string {
	var children []string
	switch e.Run.PositionNodeID {
	case "start":
		for _, nodeID := range e.Run.NodeOrder {
			if nodeID == "n1" || nodeID == "n2" {
				children = append(children, nodeID)
			}
		}
	default:
		node := e.Run.Nodes[e.Run.PositionNodeID]
		children = append(children, node.Children...)
	}

	reachable := make([]string, 0, len(children))
	for _, child := range children {
		node := e.Run.Nodes[child]
		if node != nil && !node.Resolved {
			reachable = append(reachable, child)
		}
	}
	return reachable
}

func (e *Engine) revealChildren(parentID string) {
	for _, nodeID := range e.currentChoicesForNodesFrom(parentID) {
		e.Run.Nodes[nodeID].Visible = true
	}
}

func (e *Engine) currentChoicesForNodesFrom(parentID string) []string {
	switch parentID {
	case "start":
		out := []string{}
		for _, nodeID := range e.Run.NodeOrder {
			if nodeID == "n1" || nodeID == "n2" {
				out = append(out, nodeID)
			}
		}
		return out
	default:
		node := e.Run.Nodes[parentID]
		if node == nil {
			return nil
		}
		return append([]string(nil), node.Children...)
	}
}

func (e *Engine) generateEnemyForNode(node *Node) Daemon {
	bonus := Stats{
		Integrity: 3 * node.Difficulty,
		Speed:     node.Difficulty,
		Stability: node.Difficulty,
	}

	if node.Type == NodeBoss {
		boss := e.Content.Data.Boss
		return e.generateDaemon(boss.Archetype, node.Difficulty, false, boss.Modifier, Stats{
			Integrity: bonus.Integrity + boss.IntegrityBonus,
			Speed:     bonus.Speed + boss.SpeedBonus,
			Stability: bonus.Stability + boss.StabilityBonus,
		})
	}

	archetypeIDs := make([]string, 0, len(e.Content.Archetypes))
	for id := range e.Content.Archetypes {
		archetypeIDs = append(archetypeIDs, id)
	}
	slices.Sort(archetypeIDs)
	archetypeID := archetypeIDs[e.rng.Intn(len(archetypeIDs))]
	enemy := e.generateDaemon(archetypeID, node.Difficulty, false, "", bonus)
	if node.Type == NodeCorrupted {
		enemy.Statuses["corrupted"] = 2
	}
	return enemy
}

func (e *Engine) generateDaemon(archetypeID string, difficulty int, starter bool, forceModifier string, bonus Stats) Daemon {
	def := e.Content.Archetypes[archetypeID]

	integrity := def.Base.Integrity + bonus.Integrity + e.rng.Intn(5) - 2
	speed := def.Base.Speed + bonus.Speed + e.rng.Intn(3) - 1
	stability := def.Base.Stability + bonus.Stability + e.rng.Intn(3) - 1

	traitPool := append([]string(nil), def.TraitPool...)
	if starter {
		allowed := make(map[string]struct{}, len(e.Meta.UnlockedTraits))
		for _, id := range e.Meta.UnlockedTraits {
			allowed[id] = struct{}{}
		}
		filtered := make([]string, 0, len(traitPool))
		for _, id := range traitPool {
			if _, ok := allowed[id]; ok {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) > 0 {
			traitPool = filtered
		}
	}
	traitID := traitPool[e.rng.Intn(len(traitPool))]
	trait := e.Content.Traits[traitID]

	modifierPool := []string{"single"}
	if starter {
		modifierPool = append([]string(nil), e.Meta.UnlockedModifiers...)
		if len(modifierPool) == 0 {
			modifierPool = []string{"single"}
		}
	} else {
		for id := range e.Content.Modifiers {
			modifierPool = append(modifierPool, id)
		}
	}
	modifierPool = uniqueStrings(modifierPool)
	if forceModifier != "" {
		modifierPool = []string{forceModifier}
	}

	effectIDs := e.pickAbilityEffects(def.EffectPool)

	abilities := make([]Ability, 0, 2)
	for _, effectID := range effectIDs {
		modifierID := modifierPool[e.rng.Intn(len(modifierPool))]
		abilities = append(abilities, Ability{EffectID: effectID, ModifierID: modifierID})
	}

	integrity = max(18, integrity)
	speed = max(1, speed+trait.SpeedDelta)
	stability = max(1, stability+trait.StabilityDelta)

	daemon := Daemon{
		ID:           fmt.Sprintf("daemon-%d", e.nextDaemonID),
		Name:         def.Name,
		ArchetypeID:  archetypeID,
		MaxIntegrity: integrity,
		Integrity:    integrity,
		Speed:        speed,
		Stability:    stability,
		Abilities:    abilities,
		TraitID:      traitID,
		Statuses:     map[string]int{},
	}
	e.nextDaemonID++
	return daemon
}

func (e *Engine) resolveAbility(actor Actor, ability Ability) []string {
	var source, target *Daemon
	if actor == ActorPlayer {
		source = e.activeDaemon()
		target = &e.Run.Combat.Enemy
	} else {
		source = &e.Run.Combat.Enemy
		target = e.activeDaemon()
	}
	if source == nil || target == nil {
		return nil
	}

	effect := e.Content.Effects[ability.EffectID]
	modifier := e.Content.Modifiers[ability.ModifierID]
	lines := []string{}

	sourceName := e.combatantLabel(actor, source)
	targetName := e.combatantLabel(oppositeActor(actor), target)

	lines = append(lines, fmt.Sprintf("%s used %s + %s.", sourceName, effect.Name, modifier.Name))

	targetIsSelf := effect.Kind == "heal"
	if targetIsSelf {
		target = source
		targetName = sourceName
	}

	chance := effect.Accuracy
	if !targetIsSelf {
		chance += modifier.AccuracyDelta
		chance += e.traitAccuracyBonus(source)
		chance += e.statusAccuracyBonus(source)
		chance += (e.effectiveStability(source) - e.effectiveStability(target)) / 2
		chance -= e.incomingResistance(target)
		chance = clamp(chance, 5, 95)
	}

	power := int(math.Round(float64(effect.Power) * modifier.PowerMultiplier))
	power += e.traitPowerJitter(source)
	if power < 0 {
		power = 0
	}

	hit := true
	if !targetIsSelf && chance < 100 {
		roll := e.rng.Intn(100) + 1
		hit = roll <= chance
		lines = append(lines, fmt.Sprintf("Success chance %d%%. Roll %d.", chance, roll))
	}

	if hit {
		switch effect.Kind {
		case "damage":
			damage := max(1, power)
			target.Integrity = max(0, target.Integrity-damage)
			lines = append(lines, fmt.Sprintf("%s took %d damage.", targetName, damage))
		case "hybrid":
			damage := max(1, power)
			target.Integrity = max(0, target.Integrity-damage)
			lines = append(lines, fmt.Sprintf("%s took %d damage.", targetName, damage))
			e.applyStatus(target, effect.Status, effect.StatusDuration)
			lines = append(lines, fmt.Sprintf("%s is now %s.", targetName, e.Content.Statuses[effect.Status].Name))
		case "status":
			e.applyStatus(target, effect.Status, effect.StatusDuration)
			lines = append(lines, fmt.Sprintf("%s is now %s.", targetName, e.Content.Statuses[effect.Status].Name))
		case "heal":
			heal := max(1, power)
			target.Integrity = min(target.MaxIntegrity, target.Integrity+heal)
			lines = append(lines, fmt.Sprintf("%s restored %d Integrity.", targetName, heal))
			if effect.CleanseNegative {
				if removed := e.cleanseOneNegative(target); removed != "" {
					lines = append(lines, fmt.Sprintf("%s cleared %s.", targetName, e.Content.Statuses[removed].Name))
				}
			}
			if effect.ApplySelfStatus != "" {
				e.applyStatus(target, effect.ApplySelfStatus, effect.ApplySelfDuration)
				lines = append(lines, fmt.Sprintf("%s gained %s.", targetName, e.Content.Statuses[effect.ApplySelfStatus].Name))
			}
		}
	} else {
		lines = append(lines, "The action failed to land.")
	}

	if modifier.SelfBackfireChance > 0 {
		roll := e.rng.Intn(100) + 1
		if roll <= modifier.SelfBackfireChance {
			source.Integrity = max(0, source.Integrity-modifier.SelfBackfireDamage)
			lines = append(lines, fmt.Sprintf("%s backfired for %d damage.", modifier.Name, modifier.SelfBackfireDamage))
		}
	}

	e.consumeTurn()
	lines = append(lines, e.postTurnResolution(actor)...)
	return lines
}

func (e *Engine) advanceEnemyTurns() []string {
	lines := []string{}
	for e.Run.Phase == PhaseCombat && !e.isPlayerTurn() {
		ability := e.pickEnemyAbility()
		lines = append(lines, e.resolveAbility(ActorEnemy, ability)...)
	}
	return lines
}

func (e *Engine) pickEnemyAbility() Ability {
	enemy := &e.Run.Combat.Enemy
	patchIndex := -1
	for i, ability := range enemy.Abilities {
		if ability.EffectID == "patch" {
			patchIndex = i
			break
		}
	}
	if patchIndex >= 0 && e.shouldUsePatch(enemy) {
		return enemy.Abilities[patchIndex]
	}

	best := enemy.Abilities[0]
	bestScore := -1
	for _, ability := range enemy.Abilities {
		effect := e.Content.Effects[ability.EffectID]
		if effect.Kind == "heal" {
			continue
		}
		score := effect.Power
		switch effect.Kind {
		case "hybrid":
			score += 3
		case "status":
			score += 2
		}
		if score > bestScore {
			best = ability
			bestScore = score
		}
	}
	return best
}

func (e *Engine) postTurnResolution(actor Actor) []string {
	lines := []string{}
	if actor == ActorPlayer {
		lines = append(lines, e.tickStatuses(e.activeDaemon(), e.combatantLabel(ActorPlayer, e.activeDaemon()))...)
		if e.activeDaemon() != nil {
			lines = append(lines, e.tickStatuses(&e.Run.Combat.Enemy, e.combatantLabel(ActorEnemy, &e.Run.Combat.Enemy))...)
		}
	} else {
		lines = append(lines, e.tickStatuses(&e.Run.Combat.Enemy, e.combatantLabel(ActorEnemy, &e.Run.Combat.Enemy))...)
		lines = append(lines, e.tickStatuses(e.activeDaemon(), e.combatantLabel(ActorPlayer, e.activeDaemon()))...)
	}

	player := e.activeDaemon()
	enemy := &e.Run.Combat.Enemy

	if player != nil && player.Integrity <= 0 {
		lines = append(lines, e.applyDeathEffects(player, enemy, ActorPlayer)...)
	}
	if enemy.Integrity <= 0 {
		lines = append(lines, e.applyDeathEffects(enemy, player, ActorEnemy)...)
	}
	if player != nil && player.Integrity <= 0 {
		e.finishCombatLoss(lines)
		return lines
	}
	if enemy.Integrity <= 0 {
		captured := false
		e.finishCombatWin(captured, nil, lines)
		return lines
	}

	if len(e.Run.Combat.Queue) == 0 {
		e.Run.Combat.Round++
		e.refreshQueue()
	}
	return lines
}

func (e *Engine) finishCombatLoss(lines []string) {
	node := e.Run.Nodes[e.Run.Combat.NodeID]
	node.Resolved = true
	node.Failed = true
	e.Run.PositionNodeID = node.ID
	e.revealChildren(node.ID)

	if e.Run.ActiveIndex >= 0 {
		e.Run.Roster = slices.Delete(e.Run.Roster, e.Run.ActiveIndex, e.Run.ActiveIndex+1)
	}
	e.Run.ActiveIndex = -1
	e.Run.Combat = nil

	if len(e.Run.Roster) == 0 {
		e.Run.Lost = true
		e.Run.Phase = PhaseGameOver
		return
	}

	e.Run.PendingRewardTitle = fmt.Sprintf("%s complete", node.Label)
	e.Run.PendingRewardLines = []string{
		"Active daemon crashed.",
		"No capture or stat gains were recovered from this node.",
	}
	e.Run.Phase = PhaseReward
}

func (e *Engine) finishCombatWin(captured bool, capturedDaemon *Daemon, lines []string) {
	node := e.Run.Nodes[e.Run.Combat.NodeID]
	node.Resolved = true
	e.Run.PositionNodeID = node.ID
	e.revealChildren(node.ID)

	active := e.activeDaemon()
	if active != nil {
		e.applyGrowth(active)
		lines = append(lines, fmt.Sprintf("%s gained +%d Integrity, +%d Speed, +%d Stability.", active.Name,
			e.Content.Archetypes[active.ArchetypeID].Growth.Integrity,
			e.Content.Archetypes[active.ArchetypeID].Growth.Speed,
			e.Content.Archetypes[active.ArchetypeID].Growth.Stability,
		))
	}

	if captured && capturedDaemon != nil {
		if len(e.Run.Roster) >= 3 {
			e.Run.PendingCapture = capturedDaemon
			lines = append(lines, fmt.Sprintf("%s is ready to join the roster, but capacity is full.", capturedDaemon.Name))
			e.Run.Phase = PhaseReplace
		} else {
			e.Run.Roster = append(e.Run.Roster, *capturedDaemon)
			lines = append(lines, fmt.Sprintf("%s joined the roster.", capturedDaemon.Name))
			e.Run.Phase = PhaseReward
		}
		lines = append(lines, e.applyUnlockTrigger("first_capture")...)
	} else {
		e.Run.Phase = PhaseReward
	}

	if node.Type == NodeBoss {
		e.Run.Won = true
		lines = append(lines, e.applyUnlockTrigger("first_win")...)
	}

	lines = append(lines, e.randomLogLine())
	e.Run.PendingRewardTitle = fmt.Sprintf("%s complete", node.Label)
	e.Run.PendingRewardLines = append([]string(nil), lines...)
	e.Run.Combat = nil
}

func (e *Engine) applyGrowth(daemon *Daemon) {
	growth := e.Content.Archetypes[daemon.ArchetypeID].Growth
	daemon.MaxIntegrity += growth.Integrity
	daemon.Integrity = min(daemon.MaxIntegrity, daemon.Integrity+growth.Integrity)
	daemon.Speed += growth.Speed
	daemon.Stability += growth.Stability
}

func (e *Engine) randomLogLine() string {
	return e.Content.Data.Logs[e.rng.Intn(len(e.Content.Data.Logs))]
}

func (e *Engine) refreshQueue() {
	player := e.activeDaemon()
	if player == nil || e.Run.Combat == nil {
		return
	}
	playerSpeed := e.effectiveSpeed(player)
	enemySpeed := e.effectiveSpeed(&e.Run.Combat.Enemy)
	if enemySpeed > playerSpeed {
		e.Run.Combat.Queue = []Actor{ActorEnemy, ActorPlayer}
		return
	}
	e.Run.Combat.Queue = []Actor{ActorPlayer, ActorEnemy}
}

func (e *Engine) consumeTurn() {
	if e.Run.Combat == nil || len(e.Run.Combat.Queue) == 0 {
		return
	}
	e.Run.Combat.Queue = e.Run.Combat.Queue[1:]
}

func (e *Engine) isPlayerTurn() bool {
	return e.Run.Phase == PhaseCombat && e.Run.Combat != nil && len(e.Run.Combat.Queue) > 0 && e.Run.Combat.Queue[0] == ActorPlayer
}

func (e *Engine) activeDaemon() *Daemon {
	if e.Run.ActiveIndex < 0 || e.Run.ActiveIndex >= len(e.Run.Roster) {
		return nil
	}
	return &e.Run.Roster[e.Run.ActiveIndex]
}

func (e *Engine) effectiveSpeed(d *Daemon) int {
	speed := d.Speed
	for statusID := range d.Statuses {
		speed += e.Content.Statuses[statusID].SpeedDelta
	}
	return max(1, speed)
}

func (e *Engine) effectiveStability(d *Daemon) int {
	stability := d.Stability
	for statusID := range d.Statuses {
		stability += e.Content.Statuses[statusID].StabilityDelta
	}
	return max(1, stability)
}

func (e *Engine) incomingResistance(d *Daemon) int {
	if d == nil {
		return 0
	}
	return e.Content.Traits[d.TraitID].IncomingEffectResistance
}

func (e *Engine) traitAccuracyBonus(d *Daemon) int {
	if d == nil {
		return 0
	}
	return e.Content.Traits[d.TraitID].AccuracyDelta
}

func (e *Engine) traitPowerJitter(d *Daemon) int {
	if d == nil {
		return 0
	}
	trait := e.Content.Traits[d.TraitID]
	if trait.GlitchMin == 0 && trait.GlitchMax == 0 {
		return 0
	}
	return trait.GlitchMin + e.rng.Intn(trait.GlitchMax-trait.GlitchMin+1)
}

func (e *Engine) statusAccuracyBonus(d *Daemon) int {
	if d == nil {
		return 0
	}
	total := 0
	for statusID := range d.Statuses {
		total += e.Content.Statuses[statusID].AccuracyDelta
	}
	return total
}

func (e *Engine) applyStatus(target *Daemon, statusID string, duration int) {
	if target == nil || statusID == "" || duration <= 0 {
		return
	}
	if target.Statuses == nil {
		target.Statuses = make(map[string]int)
	}
	target.Statuses[statusID] = max(target.Statuses[statusID], duration)
}

func (e *Engine) cleanseOneNegative(target *Daemon) string {
	if target == nil {
		return ""
	}
	statusIDs := make([]string, 0, len(target.Statuses))
	for statusID := range target.Statuses {
		def := e.Content.Statuses[statusID]
		if !def.IsPositive {
			statusIDs = append(statusIDs, statusID)
		}
	}
	if len(statusIDs) == 0 {
		return ""
	}
	slices.Sort(statusIDs)
	delete(target.Statuses, statusIDs[0])
	return statusIDs[0]
}

func (e *Engine) tickStatuses(target *Daemon, label string) []string {
	if target == nil {
		return nil
	}

	lines := []string{}
	for statusID, duration := range target.Statuses {
		def := e.Content.Statuses[statusID]
		if def.DOTDamage > 0 {
			target.Integrity = max(0, target.Integrity-def.DOTDamage)
			lines = append(lines, fmt.Sprintf("%s suffered %d damage from %s.", label, def.DOTDamage, def.Name))
		}
		if duration <= 1 {
			delete(target.Statuses, statusID)
		} else {
			target.Statuses[statusID] = duration - 1
		}
	}
	return lines
}

func (e *Engine) applyDeathEffects(dead *Daemon, other *Daemon, actor Actor) []string {
	if dead == nil || dead.Integrity > 0 {
		return nil
	}
	label := e.combatantLabel(actor, dead)
	lines := []string{fmt.Sprintf("%s crashed.", label)}
	trait := e.Content.Traits[dead.TraitID]
	if trait.DeathDamage > 0 && other != nil && other.Integrity > 0 {
		other.Integrity = max(0, other.Integrity-trait.DeathDamage)
		lines = append(lines, fmt.Sprintf("%s triggered %s for %d damage.", label, trait.Name, trait.DeathDamage))
	}
	return lines
}

func (e *Engine) applyUnlockTrigger(trigger string) []string {
	lines := []string{}
	for _, rule := range e.Content.Data.UnlockRules {
		if rule.Trigger != trigger || e.Meta.HasMilestone(rule.ID) {
			continue
		}
		e.Meta.AddMilestone(rule.ID)
		e.MetaDirty = true
		for _, id := range rule.UnlockStarters {
			if e.Meta.UnlockStarter(id) {
				lines = append(lines, fmt.Sprintf("Unlocked starter: %s.", e.Content.Archetypes[id].Name))
			}
		}
		for _, id := range rule.UnlockModifiers {
			if e.Meta.UnlockModifier(id) {
				lines = append(lines, fmt.Sprintf("Unlocked modifier: %s.", titleCase(id)))
			}
		}
		for _, id := range rule.UnlockTraits {
			if e.Meta.UnlockTrait(id) {
				lines = append(lines, fmt.Sprintf("Unlocked trait: %s.", e.Content.Traits[id].Name))
			}
		}
	}
	return lines
}

func (e *Engine) setCombatLog(lines []string) {
	if e.Run.Combat == nil {
		return
	}
	e.Run.Combat.LastLog = append([]string(nil), lines...)
}

func (e *Engine) pickAbilityEffects(pool []string) []string {
	unique := uniqueStrings(append([]string(nil), pool...))
	if len(unique) == 0 {
		return nil
	}

	offensive := make([]string, 0, len(unique))
	for _, effectID := range unique {
		if e.isOffensiveEffect(effectID) {
			offensive = append(offensive, effectID)
		}
	}

	if len(offensive) == 0 {
		return unique[:min(2, len(unique))]
	}

	first := offensive[e.rng.Intn(len(offensive))]
	remaining := make([]string, 0, len(unique)-1)
	for _, effectID := range unique {
		if effectID != first {
			remaining = append(remaining, effectID)
		}
	}
	if len(remaining) == 0 {
		return []string{first, first}
	}

	second := remaining[e.rng.Intn(len(remaining))]
	return []string{first, second}
}

func (e *Engine) isOffensiveEffect(effectID string) bool {
	effect, ok := e.Content.Effects[effectID]
	if !ok {
		return false
	}
	return effect.Kind == "damage" || effect.Kind == "hybrid"
}

func (e *Engine) shouldUsePatch(enemy *Daemon) bool {
	if enemy == nil || enemy.MaxIntegrity == 0 {
		return false
	}
	healthPercent := enemy.Integrity * 100 / enemy.MaxIntegrity
	if healthPercent <= 45 {
		return true
	}
	if _, leaking := enemy.Statuses["leaking"]; leaking {
		return true
	}
	if healthPercent <= 70 && e.hasNegativeStatus(enemy) {
		return true
	}
	return false
}

func (e *Engine) hasNegativeStatus(daemon *Daemon) bool {
	if daemon == nil {
		return false
	}
	for statusID := range daemon.Statuses {
		if !e.Content.Statuses[statusID].IsPositive {
			return true
		}
	}
	return false
}

func (e *Engine) combatantLabel(actor Actor, daemon *Daemon) string {
	if daemon == nil {
		if actor == ActorPlayer {
			return "Your Daemon"
		}
		return "Enemy Daemon"
	}
	if actor == ActorPlayer {
		return "Your " + daemon.Name
	}
	return "Enemy " + daemon.Name
}

func oppositeActor(actor Actor) Actor {
	if actor == ActorPlayer {
		return ActorEnemy
	}
	return ActorPlayer
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, item := range in {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func titleCase(in string) string {
	parts := strings.Split(in, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
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
