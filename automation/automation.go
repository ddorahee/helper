package automation

import (
	"time"
)

// KeySequence는 키 시퀀스 구성을 정의합니다
type KeySequence struct {
	Name       string
	StartKey   string
	KeyPresses []string
	Delays     []time.Duration
}

// 사전 정의된 키 시퀀스
var (
	// 대야 모드 - 입장
	DaeyaEnterSequence = KeySequence{
		Name:       "대야 (입장)",
		StartKey:   "o",
		KeyPresses: []string{"o", "enter", "enter", "esc", "d", "x", "5"},
		Delays:     []time.Duration{3 * time.Second, 1 * time.Second, 1 * time.Second, 1 * time.Second, 0 * time.Second, 0 * time.Second},
	}

	// 대야 모드 - 파티
	DaeyaPartySequence = KeySequence{
		Name:       "대야 (파티)",
		StartKey:   "x",
		KeyPresses: []string{"x", "d"},
		Delays:     []time.Duration{1 * time.Second, 1 * time.Second},
	}

	// 칸첸 모드 - 입장
	KanchenEnterSequence = KeySequence{
		Name:       "칸첸 (입장)",
		StartKey:   "o",
		KeyPresses: []string{"o", "enter", "enter", "esc", "d"},
		Delays:     []time.Duration{3 * time.Second, 1 * time.Second, 1 * time.Second, 1 * time.Second},
	}

	// 칸첸 모드 - 파티
	KanchenPartySequence = KeySequence{
		Name:       "칸첸 (파티)",
		StartKey:   "x",
		KeyPresses: []string{"x", "d"},
		Delays:     []time.Duration{1 * time.Second, 1 * time.Second},
	}
)

// 대야 모드 (입장) 자동화 시퀀스를 실행합니다
func (km *KeyboardManager) DaeyaEnter() {
	km.RunKeySequence(DaeyaEnterSequence)
}

// 대야 모드 (파티) 자동화 시퀀스를 실행합니다
func (km *KeyboardManager) DaeyaParty() {
	km.RunKeySequence(DaeyaPartySequence)
}

// 칸첸 모드 (입장) 자동화 시퀀스를 실행합니다
func (km *KeyboardManager) KanchenEnter() {
	km.RunKeySequence(KanchenEnterSequence)
}

// 칸첸 모드 (파티) 자동화 시퀀스를 실행합니다
func (km *KeyboardManager) KanchenParty() {
	km.RunKeySequence(KanchenPartySequence)
}

// RunKeySequence는 지정된 키 시퀀스를 실행합니다
func (km *KeyboardManager) RunKeySequence(sequence KeySequence) {
	// 매크로 실행 중 실수로 버튼을 누를 수 없도록 간단한 딜레이
	time.Sleep(300 * time.Millisecond)

	// 무한 루프로 키 시퀀스 실행
	for {
		// 계속 실행 중인지 확인
		if !km.IsRunning() {
			break
		}

		// 각 키 처리
		for i, key := range sequence.KeyPresses {
			err := km.SendKeyPress(key)
			if err != nil {
				return
			}

			// 긴 대기 시간이 필요한 경우 카운트다운 표시
			if i < len(sequence.Delays) && sequence.Delays[i] >= 2*time.Second {
				seconds := int(sequence.Delays[i].Seconds())

				for j := seconds; j > 0; j-- {
					// 중간에 중지되었는지 확인
					if !km.IsRunning() {
						return
					}

					if j < seconds { // 첫 번째 메시지는 이미 출력했으므로 건너뜀
					}

					time.Sleep(1 * time.Second)
				}
			} else if i < len(sequence.Delays) {
				// 짧은 대기
				time.Sleep(sequence.Delays[i])
			}

			// 계속 실행 중인지 확인
			if !km.IsRunning() {
				break
			}
		}

		// 루프 계속 진행 전 짧은 대기
		if km.IsRunning() {
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
}

// formatKeySequence는 키 시퀀스를 포맷팅합니다
func formatKeySequence(keys []string) string {
	result := ""
	for i, key := range keys {
		if i > 0 {
			result += ", "
		}
		result += key
	}
	return result
}
