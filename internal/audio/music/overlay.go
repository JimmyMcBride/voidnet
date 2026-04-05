package music

func patternForLoopState(loop LoopID, state ReactiveState) (Pattern, bool) {
	if loop != LoopBattle {
		return PatternForLoop(loop)
	}

	state = state.normalized()
	base, ok := battlePatternForTheme(state.BattleTheme)
	if !ok {
		return Pattern{}, false
	}
	base.BPM = scaledBattleBPM(base.BPM, state.Intensity)
	return base, true
}

func PatternForState(loop LoopID, state ReactiveState) (Pattern, bool) {
	return patternForLoopState(loop, state)
}

func battlePatternForTheme(theme BattleTheme) (Pattern, bool) {
	switch theme {
	case "", BattleThemeStandard:
		return battleStandardPattern(), true
	case BattleThemeCorrupted:
		return battleCorruptedPattern(), true
	case BattleThemeBoss:
		return battleBossPattern(), true
	default:
		return Pattern{}, false
	}
}

func scaledBattleBPM(base float64, intensity int) float64 {
	switch intensity {
	case 1:
		return base * 1.12
	case 2:
		return base * 1.24
	default:
		return base
	}
}
