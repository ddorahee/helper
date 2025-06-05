package automation

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
)

// KeyboardManager는 키보드 자동화 기능을 관리합니다
type KeyboardManager struct {
	Running    bool
	Mutex      sync.Mutex
	StopReason string
}

// NewKeyboardManager는 새로운 키보드 관리자를 생성합니다
func NewKeyboardManager() *KeyboardManager {
	return &KeyboardManager{
		Running:    false,
		Mutex:      sync.Mutex{},
		StopReason: "",
	}
}

// SendKeyPress는 키 입력을 시뮬레이션합니다
func (km *KeyboardManager) SendKeyPress(key string) error {
	// 키 입력 전에 짧은 지연 추가
	time.Sleep(300 * time.Millisecond)

	// 안전한 키 입력을 위한 recover 처리
	defer func() {
		if r := recover(); r != nil {
			km.StopOperation(fmt.Sprintf("키 입력 중 오류 발생: %v", r))
			log.Printf("키 입력 패닉 복구: %v", r)
		}
	}()

	// robotgo를 사용하여 키 입력
	robotgo.KeyTap(key)
	log.Printf("키 입력: %s", key)

	return nil
}

// StopOperation은 작업을 중지하고 이유를 기록합니다
func (km *KeyboardManager) StopOperation(reason string) {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()

	if km.Running {
		km.Running = false
		km.StopReason = reason
		log.Printf("키보드 매니저 중지: %s", reason)
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
	if running {
		km.StopReason = ""
		log.Println("키보드 매니저 시작")
	} else {
		log.Println("키보드 매니저 중지")
	}
}

// GetStopReason은 중지 이유를 반환합니다
func (km *KeyboardManager) GetStopReason() string {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()
	return km.StopReason
}
