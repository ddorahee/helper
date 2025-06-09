package automation

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
)

type KeyboardManager struct {
	Running    bool
	Mutex      sync.Mutex
	StopReason string
}

func NewKeyboardManager() *KeyboardManager {
	return &KeyboardManager{
		Running:    false,
		Mutex:      sync.Mutex{},
		StopReason: "",
	}
}

func (km *KeyboardManager) SendKeyPress(key string) error {
	time.Sleep(300 * time.Millisecond)

	defer func() {
		if r := recover(); r != nil {
			km.StopOperation(fmt.Sprintf("키 입력 중 오류 발생: %v", r))
			log.Printf("키 입력 패닉 복구: %v", r)
		}
	}()

	robotgo.KeyTap(key)
	log.Printf("키 입력: %s", key)

	return nil
}

func (km *KeyboardManager) StopOperation(reason string) {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()

	if km.Running {
		km.Running = false
		km.StopReason = reason
		log.Printf("키보드 매니저 중지: %s", reason)
	}
}

func (km *KeyboardManager) IsRunning() bool {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()
	return km.Running
}

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

func (km *KeyboardManager) GetStopReason() string {
	km.Mutex.Lock()
	defer km.Mutex.Unlock()
	return km.StopReason
}
