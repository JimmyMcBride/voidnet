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
	EventAlert           Event = Event(sfx.PresetAlert)
	EventLevelClear      Event = Event(sfx.PresetLevelClear)
	EventSystemBoot      Event = Event(sfx.PresetSystemBoot)
	EventGlitchStinger   Event = Event(sfx.PresetGlitchStinger)
)

func GenerateEventSFX(event Event, seed int64) ([]int16, int, error) {
	return sfx.GeneratePreset(sfx.Preset(event), seed)
}
