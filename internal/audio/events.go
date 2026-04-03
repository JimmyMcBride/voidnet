package audio

import "voidnet/internal/audio/sfx"

type Event string

const (
	EventCursorMove      Event = Event(sfx.PresetCursorMove)
	EventSelect          Event = Event(sfx.PresetSelect)
	EventScan            Event = Event(sfx.PresetScan)
	EventHackStart       Event = Event(sfx.PresetHackStart)
	EventHackSuccess     Event = Event(sfx.PresetHackSuccess)
	EventHackFail        Event = Event(sfx.PresetHackFail)
	EventDaemonAppears   Event = Event(sfx.PresetDaemonAppears)
	EventDaemonCaptured  Event = Event(sfx.PresetDaemonCaptured)
	EventCorruptionBurst Event = Event(sfx.PresetCorruptionBurst)
	EventPatchRestore    Event = Event(sfx.PresetPatchRestore)
	EventBackfire        Event = Event(sfx.PresetBackfire)
	EventCrash           Event = Event(sfx.PresetCrash)
	EventAlert           Event = Event(sfx.PresetAlert)
	EventLevelClear      Event = Event(sfx.PresetLevelClear)
	EventUnlock          Event = Event(sfx.PresetUnlock)
	EventSystemBoot      Event = Event(sfx.PresetSystemBoot)
	EventGlitchStinger   Event = Event(sfx.PresetGlitchStinger)
	EventRunVictory      Event = Event(sfx.PresetRunVictory)
	EventRunDefeat       Event = Event(sfx.PresetRunDefeat)
)

func GenerateEventSFX(event Event, seed int64) ([]int16, int, error) {
	return sfx.GeneratePreset(sfx.Preset(event), seed)
}
