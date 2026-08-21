package automation

import (
	"log"
	"sync"
	"time"
)

var procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")

// EscStopWatcher ESC 빠른 연타(2회)로 자동화 전체를 중지하는 비상 정지 워처.
// gohook(키매핑)은 필요할 때만 켜지고 전역 훅은 1개만 가능하므로,
// 훅 대신 GetAsyncKeyState 폴링(30ms)으로 ESC 눌림을 감지한다.
// 자동화 스스로도 ESC를 누르므로(입장 시퀀스 등) 오발동을 줄이기 위해
// 짧은 간격(450ms 이내) 연속 2회만 인정하고, 발동 후 2초간은 무시한다.
type EscStopWatcher struct {
	mu        sync.Mutex
	running   bool
	stopChan  chan struct{}
	onTrigger func() // 연타 감지 시 호출 (별도 고루틴에서)
}

// NewEscStopWatcher 생성. onTrigger는 ESC 연타 감지 시 호출된다.
func NewEscStopWatcher(onTrigger func()) *EscStopWatcher {
	return &EscStopWatcher{onTrigger: onTrigger}
}

// Start 감시 시작 (앱 켜져 있는 동안 상시 동작)
func (w *EscStopWatcher) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	w.running = true
	w.stopChan = make(chan struct{})
	go w.run(w.stopChan)
	log.Println("[ESC중지] 감시 시작 (ESC 빠르게 2번 → 전체 중지)")
}

// Stop 감시 중지
func (w *EscStopWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.running = false
	close(w.stopChan)
}

func (w *EscStopWatcher) run(stop chan struct{}) {
	const vkEscape = 0x1B
	const doubleTapWindow = 450 * time.Millisecond
	const cooldown = 2 * time.Second

	var lastPress time.Time
	var lastTrigger time.Time
	wasDown := false

	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ret, _, _ := procGetAsyncKeyState.Call(uintptr(vkEscape))
			down := ret&0x8000 != 0
			pressed := down && !wasDown // 새로 눌린 순간만
			wasDown = down
			if !pressed {
				continue
			}

			now := time.Now()
			// 발동 직후 쿨다운 — 연타 잔여 입력으로 재발동 방지
			if !lastTrigger.IsZero() && now.Sub(lastTrigger) < cooldown {
				lastPress = time.Time{}
				continue
			}

			if !lastPress.IsZero() && now.Sub(lastPress) <= doubleTapWindow {
				// 연타 2회 감지
				lastPress = time.Time{}
				lastTrigger = now
				log.Println("[ESC중지] ESC 연타 감지 — 전체 중지 실행")
				if w.onTrigger != nil {
					go w.onTrigger()
				}
			} else {
				lastPress = now
			}
		}
	}
}
