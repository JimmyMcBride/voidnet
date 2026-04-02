package content

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed data/*.json
var dataFS embed.FS

type StatsDef struct {
	Integrity int `json:"integrity"`
	Speed     int `json:"speed"`
	Stability int `json:"stability"`
}

type ArchetypeDef struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Role       string   `json:"role"`
	Base       StatsDef `json:"base"`
	Growth     StatsDef `json:"growth"`
	EffectPool []string `json:"effect_pool"`
	TraitPool  []string `json:"trait_pool"`
}

type EffectDef struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Kind              string `json:"kind"`
	Accuracy          int    `json:"accuracy"`
	Power             int    `json:"power"`
	Status            string `json:"status"`
	StatusDuration    int    `json:"status_duration"`
	CleanseNegative   bool   `json:"cleanse_negative"`
	ApplySelfStatus   string `json:"apply_self_status"`
	ApplySelfDuration int    `json:"apply_self_duration"`
}

type ModifierDef struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	AccuracyDelta      int     `json:"accuracy_delta"`
	PowerMultiplier    float64 `json:"power_multiplier"`
	SelfBackfireChance int     `json:"self_backfire_chance"`
	SelfBackfireDamage int     `json:"self_backfire_damage"`
}

type TraitDef struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	Description              string `json:"description"`
	SpeedDelta               int    `json:"speed_delta"`
	StabilityDelta           int    `json:"stability_delta"`
	AccuracyDelta            int    `json:"accuracy_delta"`
	IncomingEffectResistance int    `json:"incoming_effect_resistance"`
	DeathDamage              int    `json:"death_damage"`
	GlitchMin                int    `json:"glitch_min"`
	GlitchMax                int    `json:"glitch_max"`
}

type StatusDef struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	AccuracyDelta  int    `json:"accuracy_delta"`
	SpeedDelta     int    `json:"speed_delta"`
	StabilityDelta int    `json:"stability_delta"`
	DOTDamage      int    `json:"dot_damage"`
	IsPositive     bool   `json:"is_positive"`
}

type UnlockRuleDef struct {
	ID              string   `json:"id"`
	Trigger         string   `json:"trigger"`
	UnlockStarters  []string `json:"unlock_starters"`
	UnlockModifiers []string `json:"unlock_modifiers"`
	UnlockTraits    []string `json:"unlock_traits"`
}

type BossDef struct {
	Archetype      string `json:"archetype"`
	Modifier       string `json:"modifier"`
	IntegrityBonus int    `json:"integrity_bonus"`
	SpeedBonus     int    `json:"speed_bonus"`
	StabilityBonus int    `json:"stability_bonus"`
}

type Data struct {
	Archetypes  []ArchetypeDef  `json:"archetypes"`
	Effects     []EffectDef     `json:"effects"`
	Modifiers   []ModifierDef   `json:"modifiers"`
	Traits      []TraitDef      `json:"traits"`
	Statuses    []StatusDef     `json:"statuses"`
	UnlockRules []UnlockRuleDef `json:"unlock_rules"`
	Boss        BossDef         `json:"boss"`
	Logs        []string        `json:"logs"`
	NodeNames   []string        `json:"node_names"`
}

type Registry struct {
	Data       Data
	Archetypes map[string]ArchetypeDef
	Effects    map[string]EffectDef
	Modifiers  map[string]ModifierDef
	Traits     map[string]TraitDef
	Statuses   map[string]StatusDef
}

func Load() (*Registry, error) {
	raw, err := dataFS.ReadFile("data/game.json")
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode content: %w", err)
	}

	reg := &Registry{
		Data:       data,
		Archetypes: make(map[string]ArchetypeDef, len(data.Archetypes)),
		Effects:    make(map[string]EffectDef, len(data.Effects)),
		Modifiers:  make(map[string]ModifierDef, len(data.Modifiers)),
		Traits:     make(map[string]TraitDef, len(data.Traits)),
		Statuses:   make(map[string]StatusDef, len(data.Statuses)),
	}

	for _, item := range data.Archetypes {
		if item.ID == "" {
			return nil, fmt.Errorf("archetype with empty id")
		}
		if _, exists := reg.Archetypes[item.ID]; exists {
			return nil, fmt.Errorf("duplicate archetype id %q", item.ID)
		}
		reg.Archetypes[item.ID] = item
	}

	for _, item := range data.Effects {
		if item.ID == "" {
			return nil, fmt.Errorf("effect with empty id")
		}
		if _, exists := reg.Effects[item.ID]; exists {
			return nil, fmt.Errorf("duplicate effect id %q", item.ID)
		}
		reg.Effects[item.ID] = item
	}

	for _, item := range data.Modifiers {
		if item.ID == "" {
			return nil, fmt.Errorf("modifier with empty id")
		}
		if _, exists := reg.Modifiers[item.ID]; exists {
			return nil, fmt.Errorf("duplicate modifier id %q", item.ID)
		}
		reg.Modifiers[item.ID] = item
	}

	for _, item := range data.Traits {
		if item.ID == "" {
			return nil, fmt.Errorf("trait with empty id")
		}
		if _, exists := reg.Traits[item.ID]; exists {
			return nil, fmt.Errorf("duplicate trait id %q", item.ID)
		}
		reg.Traits[item.ID] = item
	}

	for _, item := range data.Statuses {
		if item.ID == "" {
			return nil, fmt.Errorf("status with empty id")
		}
		if _, exists := reg.Statuses[item.ID]; exists {
			return nil, fmt.Errorf("duplicate status id %q", item.ID)
		}
		reg.Statuses[item.ID] = item
	}

	if err := reg.validate(); err != nil {
		return nil, err
	}

	return reg, nil
}

func (r *Registry) validate() error {
	if len(r.Data.NodeNames) < 5 {
		return fmt.Errorf("need at least 5 node names, got %d", len(r.Data.NodeNames))
	}
	if len(r.Data.Logs) == 0 {
		return fmt.Errorf("need at least one log line")
	}

	for _, archetype := range r.Data.Archetypes {
		hasOffense := false
		for _, effectID := range archetype.EffectPool {
			effect, ok := r.Effects[effectID]
			if !ok {
				return fmt.Errorf("archetype %q references unknown effect %q", archetype.ID, effectID)
			}
			if effect.Kind == "damage" || effect.Kind == "hybrid" {
				hasOffense = true
			}
		}
		for _, traitID := range archetype.TraitPool {
			if _, ok := r.Traits[traitID]; !ok {
				return fmt.Errorf("archetype %q references unknown trait %q", archetype.ID, traitID)
			}
		}
		if !hasOffense {
			return fmt.Errorf("archetype %q must provide at least one offensive effect", archetype.ID)
		}
	}

	for _, effect := range r.Data.Effects {
		if effect.Status != "" {
			if _, ok := r.Statuses[effect.Status]; !ok {
				return fmt.Errorf("effect %q references unknown status %q", effect.ID, effect.Status)
			}
		}
		if effect.ApplySelfStatus != "" {
			if _, ok := r.Statuses[effect.ApplySelfStatus]; !ok {
				return fmt.Errorf("effect %q references unknown self status %q", effect.ID, effect.ApplySelfStatus)
			}
		}
	}

	for _, rule := range r.Data.UnlockRules {
		for _, starterID := range rule.UnlockStarters {
			if _, ok := r.Archetypes[starterID]; !ok {
				return fmt.Errorf("unlock rule %q references unknown starter %q", rule.ID, starterID)
			}
		}
		for _, modifierID := range rule.UnlockModifiers {
			if _, ok := r.Modifiers[modifierID]; !ok {
				return fmt.Errorf("unlock rule %q references unknown modifier %q", rule.ID, modifierID)
			}
		}
		for _, traitID := range rule.UnlockTraits {
			if _, ok := r.Traits[traitID]; !ok {
				return fmt.Errorf("unlock rule %q references unknown trait %q", rule.ID, traitID)
			}
		}
	}

	if _, ok := r.Archetypes[r.Data.Boss.Archetype]; !ok {
		return fmt.Errorf("boss references unknown archetype %q", r.Data.Boss.Archetype)
	}
	if _, ok := r.Modifiers[r.Data.Boss.Modifier]; !ok {
		return fmt.Errorf("boss references unknown modifier %q", r.Data.Boss.Modifier)
	}

	return nil
}

func (r *Registry) SortedStarterIDs(ids []string) []string {
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := r.Archetypes[id]; ok {
			filtered = append(filtered, id)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return r.Archetypes[filtered[i]].Name < r.Archetypes[filtered[j]].Name
	})
	return filtered
}
