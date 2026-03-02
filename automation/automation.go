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

// 기본 키 시퀀스 (초기값)
var (
	DefaultDaeyaEnterSequence = KeySequence{
		Name:       "대야 (입장)",
		StartKey:   "o",
		KeyPresses: []string{"o", "enter", "enter", "esc", "d", "x", "5"},
		Delays:     []time.Duration{3 * time.Second, 1 * time.Second, 1 * time.Second, 1 * time.Second, 0 * time.Second, 0 * time.Second},
	}

	DefaultDaeyaPartySequence = KeySequence{
		Name:       "대야 (파티)",
		StartKey:   "x",
		KeyPresses: []string{"x", "d"},
		Delays:     []time.Duration{1 * time.Second, 1 * time.Second},
	}

	DefaultKanchenEnterSequence = KeySequence{
		Name:       "칸첸 (입장)",
		StartKey:   "o",
		KeyPresses: []string{"o", "enter", "enter", "esc", "d"},
		Delays:     []time.Duration{3 * time.Second, 1 * time.Second, 1 * time.Second, 1 * time.Second},
	}

	DefaultKanchenPartySequence = KeySequence{
		Name:       "칸첸 (파티)",
		StartKey:   "x",
		KeyPresses: []string{"x", "d"},
		Delays:     []time.Duration{1 * time.Second, 1 * time.Second},
	}
)

// SetSequence 키보드 매니저의 시퀀스를 런타임에 업데이트
func (km *KeyboardManager) SetSequence(mode string, keys []string, delays []time.Duration) {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()

	if km.Sequences == nil {
		km.Sequences = make(map[string]KeySequence)
	}

	name := mode
	startKey := ""
	if len(keys) > 0 {
		startKey = keys[0]
	}

	switch mode {
	case "daeya-enter":
		name = "대야 (입장)"
	case "daeya-party":
		name = "대야 (파티)"
	case "kanchen-enter":
		name = "칸첸 (입장)"
	case "kanchen-party":
		name = "칸첸 (파티)"
	}

	km.Sequences[mode] = KeySequence{
		Name:       name,
		StartKey:   startKey,
		KeyPresses: keys,
		Delays:     delays,
	}
}

// getSequence 시퀀스 가져오기 (설정된 것이 있으면 사용, 없으면 기본값)
func (km *KeyboardManager) getSequence(mode string, defaultSeq KeySequence) KeySequence {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()

	if km.Sequences != nil {
		if seq, ok := km.Sequences[mode]; ok {
			return seq
		}
	}
	return defaultSeq
}

// 대야 모드 (입장) 자동화 시퀀스를 실행합니다
func (km *KeyboardManager) DaeyaEnter() {
	km.RunKeySequence(km.getSequence("daeya-enter", DefaultDaeyaEnterSequence))
}

// 대야 모드 (파티) 자동화 시퀀스를 실행합니다
func (km *KeyboardManager) DaeyaParty() {
	km.RunKeySequence(km.getSequence("daeya-party", DefaultDaeyaPartySequence))
}

// 칸첸 모드 (입장) 자동화 시퀀스를 실행합니다
func (km *KeyboardManager) KanchenEnter() {
	km.RunKeySequence(km.getSequence("kanchen-enter", DefaultKanchenEnterSequence))
}

// 칸첸 모드 (파티) 자동화 시퀀스를 실행합니다
func (km *KeyboardManager) KanchenParty() {
	km.RunKeySequence(km.getSequence("kanchen-party", DefaultKanchenPartySequence))
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
			// 일시정지 상태면 재개될 때까지 대기
			for km.IsPaused() {
				if !km.IsRunning() {
					return
				}
				select {
				case <-km.ResumeCh:
				case <-time.After(500 * time.Millisecond):
				}
			}

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
