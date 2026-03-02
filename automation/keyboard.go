package automation

import (
	"fmt"
	"sync"
	"time"
	"github.com/go-vgo/robotgo"
)

// KeyboardManager는 키보드 자동화 기능을 관리합니다
type KeyboardManager struct {
	Running    bool
	Paused     bool
	ResumeCh   chan struct{}
	Mutex      sync.Mutex
	StopReason string
	Sequences  map[string]KeySequence // 런타임 커스텀 시퀀스
}

// NewKeyboardManager는 새로운 키보드 관리자를 생성합니다
func NewKeyboardManager() *KeyboardManager {
	return &KeyboardManager{
		Running:    false,
		Paused:     false,
		ResumeCh:   make(chan struct{}, 1),
		Mutex:      sync.Mutex{},
		StopReason: "",
	}
}

// Pause 매크로 루프를 일시정지합니다
func (km *KeyboardManager) Pause() {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()
	km.Paused = true
}

// Resume 매크로 루프를 재개합니다
func (km *KeyboardManager) Resume() {
	km.Mutex.Lock()
	km.Paused = false
	km.Mutex.Unlock()
	// 비블로킹 신호 전송
	select {
	case km.ResumeCh <- struct{}{}:
	default:
	}
}

// IsPaused 일시정지 상태 확인
func (km *KeyboardManager) IsPaused() bool {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()
	return km.Paused
}

// SendKeyPress는 키 입력을 시뮬레이션합니다
func (km *KeyboardManager) SendKeyPress(key string) error {
	// 키 입력 전에 짧은 지연 추가
	time.Sleep(300 * time.Millisecond)

	// 일시정지 상태면 키 입력 스킵
	if km.IsPaused() {
		return nil
	}

	// robotgo를 사용하여 키 입력
	func() {
		defer func() {
			if r := recover(); r != nil {
				km.StopOperation(fmt.Sprintf("키 입력 중 오류 발생: %v", r))
			}
		}()
		robotgo.KeyTap(key)
	}()

	return nil
}

// StopOperation은 작업을 중지하고 이유를 기록합니다
func (km *KeyboardManager) StopOperation(reason string) {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()

	if km.Running {
		km.Running = false
		km.StopReason = reason
	}
}

// IsRunning은 키보드 관리자가 실행 중인지 확인합니다
func (km *KeyboardManager) IsRunning() bool {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()
	return km.Running
}

// SetRunning은 실행 상태를 설정합니다
func (km *KeyboardManager) SetRunning(running bool) {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()
	km.Running = running
}