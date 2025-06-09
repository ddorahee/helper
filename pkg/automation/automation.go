package automation

import (
	"time"
)

type KeySequence struct {
	Name       string
	StartKey   string
	KeyPresses []string
	Delays     []time.Duration
}

var (
	DaeyaEnterSequence = KeySequence{
		Name:       "대야 (입장)",
		StartKey:   "o",
		KeyPresses: []string{"o", "enter", "enter", "esc", "d", "x", "5"},
		Delays:     []time.Duration{0, 1 * time.Second, 1 * time.Second, 1 * time.Second, 0, 0, 0},
	}

	DaeyaPartySequence = KeySequence{
		Name:       "대야 (파티)",
		StartKey:   "x",
		KeyPresses: []string{"x", "d"},
		Delays:     []time.Duration{1 * time.Second, 1 * time.Second},
	}

	KanchenEnterSequence = KeySequence{
		Name:       "칸첸 (입장)",
		StartKey:   "o",
		KeyPresses: []string{"o", "enter", "enter", "esc", "d"},
		Delays:     []time.Duration{0, 1 * time.Second, 1 * time.Second, 1 * time.Second, 0},
	}

	KanchenPartySequence = KeySequence{
		Name:       "칸첸 (파티)",
		StartKey:   "x",
		KeyPresses: []string{"x", "d"},
		Delays:     []time.Duration{1 * time.Second, 1 * time.Second},
	}
)

func (km *KeyboardManager) DaeyaEnter() {
	km.RunKeySequence(DaeyaEnterSequence)
}

func (km *KeyboardManager) DaeyaParty() {
	km.RunKeySequence(DaeyaPartySequence)
}

func (km *KeyboardManager) KanchenEnter() {
	km.RunKeySequence(KanchenEnterSequence)
}

func (km *KeyboardManager) KanchenParty() {
	km.RunKeySequence(KanchenPartySequence)
}

func (km *KeyboardManager) RunKeySequence(sequence KeySequence) {
	time.Sleep(300 * time.Millisecond)

	for {
		if !km.IsRunning() {
			break
		}

		for i, key := range sequence.KeyPresses {
			err := km.SendKeyPress(key)
			if err != nil {
				km.StopOperation("키 입력 오류 발생")
				return
			}

			if i < len(sequence.Delays) && sequence.Delays[i] >= 2*time.Second {
				seconds := int(sequence.Delays[i].Seconds())
				for j := seconds; j > 0; j-- {
					if !km.IsRunning() {
						return
					}
					time.Sleep(1 * time.Second)
				}
			} else if i < len(sequence.Delays) {
				time.Sleep(sequence.Delays[i])
			}

			if !km.IsRunning() {
				break
			}
		}

		if km.IsRunning() {
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
}
