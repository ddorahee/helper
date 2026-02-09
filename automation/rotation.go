package automation

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// RotationState 순환 상태
type RotationState int

const (
	RotationIdle       RotationState = iota // 대기
	RotationActivating                      // 창 활성화 중
	RotationStarting                        // 사냥 시작 중
	RotationHunting                         // 사냥 중 (타이머 대기)
	RotationSwitching                       // 다음 캐릭터로 전환 중
	RotationComplete                        // 모든 캐릭터 완료
)

// RotationCharacter 순환에 참여하는 캐릭터 정보
type RotationCharacter struct {
	ID             string
	Name           string
	HuntingArea    string
	DropdownIndex  int
	DurationMins   int
	Order          int
	WindowHWND     uint64
}

// RotationStatus 현재 순환 상태 정보
type RotationStatus struct {
	State            string `json:"state"`
	Running          bool   `json:"running"`
	CurrentIndex     int    `json:"currentIndex"`
	CurrentCharacter string `json:"currentCharacter"`
	CurrentArea      string `json:"currentArea"`
	RemainingSeconds int    `json:"remainingSeconds"`
	TotalCharacters  int    `json:"totalCharacters"`
	CompletedCount   int    `json:"completedCount"`
	Message          string `json:"message"`
}

// RotationEvent 프론트엔드에 전송할 이벤트
type RotationEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// EventCallback 이벤트 콜백 함수 타입
type EventCallback func(eventType string, payload interface{})

// RotationManager 순환 사냥 관리자
type RotationManager struct {
	mu             sync.RWMutex
	state          RotationState
	running        bool
	characters     []RotationCharacter
	currentIndex   int
	completedCount int
	remainingSecs  int
	coords         GameUICoords
	mouse          *MouseAutomation
	window         *WindowManager
	stopChan       chan struct{}
	eventCallback  EventCallback
}

// NewRotationManager 새로운 순환 관리자 생성
func NewRotationManager(wm *WindowManager, ma *MouseAutomation) *RotationManager {
	return &RotationManager{
		state:   RotationIdle,
		running: false,
		window:  wm,
		mouse:   ma,
	}
}

// SetEventCallback 이벤트 콜백 설정
func (rm *RotationManager) SetEventCallback(cb EventCallback) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.eventCallback = cb
}

// SetCoordinates UI 좌표 설정
func (rm *RotationManager) SetCoordinates(coords GameUICoords) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.coords = coords
}

// Start 순환 시작
func (rm *RotationManager) Start(characters []RotationCharacter, coords GameUICoords) error {
	rm.mu.Lock()

	if rm.running {
		rm.mu.Unlock()
		return fmt.Errorf("이미 실행 중입니다")
	}

	if len(characters) == 0 {
		rm.mu.Unlock()
		return fmt.Errorf("등록된 캐릭터가 없습니다")
	}

	// 모든 캐릭터에 윈도우 할당 확인
	for _, c := range characters {
		if c.WindowHWND == 0 {
			rm.mu.Unlock()
			return fmt.Errorf("캐릭터 '%s'에 윈도우가 할당되지 않았습니다", c.Name)
		}
	}

	rm.characters = characters
	rm.coords = coords
	rm.currentIndex = 0
	rm.completedCount = 0
	rm.running = true
	rm.state = RotationIdle
	rm.stopChan = make(chan struct{})

	rm.mu.Unlock()

	// 순환 고루틴 시작
	go rm.runRotation()

	return nil
}

// Stop 순환 중지
func (rm *RotationManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.running {
		return
	}

	rm.running = false
	close(rm.stopChan)
	rm.state = RotationIdle
	rm.emitEvent("rotationLog", map[string]string{"message": "순환 사냥이 중지되었습니다."})
	rm.emitEvent("rotationStatus", rm.buildStatusLocked())
}

// IsRunning 실행 중 여부
func (rm *RotationManager) IsRunning() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.running
}

// GetStatus 현재 상태 반환
func (rm *RotationManager) GetStatus() RotationStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.buildStatusLocked()
}

func (rm *RotationManager) buildStatusLocked() RotationStatus {
	status := RotationStatus{
		State:            rm.stateString(),
		Running:          rm.running,
		CurrentIndex:     rm.currentIndex,
		TotalCharacters:  len(rm.characters),
		CompletedCount:   rm.completedCount,
		RemainingSeconds: rm.remainingSecs,
	}

	if rm.currentIndex < len(rm.characters) {
		status.CurrentCharacter = rm.characters[rm.currentIndex].Name
		status.CurrentArea = rm.characters[rm.currentIndex].HuntingArea
	}

	return status
}

func (rm *RotationManager) stateString() string {
	switch rm.state {
	case RotationIdle:
		return "idle"
	case RotationActivating:
		return "activating"
	case RotationStarting:
		return "starting"
	case RotationHunting:
		return "hunting"
	case RotationSwitching:
		return "switching"
	case RotationComplete:
		return "complete"
	default:
		return "unknown"
	}
}

// runRotation 순환 메인 루프
func (rm *RotationManager) runRotation() {
	log.Println("[순환] 순환 사냥 시작")
	rm.emitEvent("rotationLog", map[string]string{"message": "순환 사냥을 시작합니다."})

	for rm.currentIndex < len(rm.characters) {
		rm.mu.RLock()
		if !rm.running {
			rm.mu.RUnlock()
			return
		}
		char := rm.characters[rm.currentIndex]
		rm.mu.RUnlock()

		// 1. 창 활성화
		rm.setState(RotationActivating)
		msg := fmt.Sprintf("[%d/%d] %s - %s 창 활성화 중...",
			rm.currentIndex+1, len(rm.characters), char.Name, char.HuntingArea)
		log.Printf("[순환] %s", msg)
		rm.emitEvent("rotationLog", map[string]string{"message": msg})

		if err := rm.window.ActivateWindow(char.WindowHWND); err != nil {
			errMsg := fmt.Sprintf("%s 창 활성화 실패: %v", char.Name, err)
			log.Printf("[순환] 오류: %s", errMsg)
			rm.emitEvent("rotationError", map[string]string{"message": errMsg})
			rm.stopWithError(errMsg)
			return
		}
		time.Sleep(1 * time.Second)

		// 중단 확인
		if rm.isStopped() {
			return
		}

		// 2. 사냥 시작
		rm.setState(RotationStarting)
		msg = fmt.Sprintf("%s - 사냥 시작 중 (사냥터: %s)...", char.Name, char.HuntingArea)
		log.Printf("[순환] %s", msg)
		rm.emitEvent("rotationLog", map[string]string{"message": msg})

		logFn := func(msg string) {
			rm.emitEvent("rotationLog", map[string]string{"message": fmt.Sprintf("[%s] %s", char.Name, msg)})
		}
		if err := rm.mouse.StartHunting(char.WindowHWND, rm.coords, char.DropdownIndex, logFn); err != nil {
			errMsg := fmt.Sprintf("%s 사냥 시작 실패: %v", char.Name, err)
			log.Printf("[순환] 오류: %s", errMsg)
			rm.emitEvent("rotationError", map[string]string{"message": errMsg})
			rm.stopWithError(errMsg)
			return
		}

		// 중단 확인
		if rm.isStopped() {
			return
		}

		// 3. 사냥 대기
		rm.setState(RotationHunting)
		durationSecs := char.DurationMins * 60
		msg = fmt.Sprintf("%s - 사냥 중 (%d분 대기)...", char.Name, char.DurationMins)
		log.Printf("[순환] %s", msg)
		rm.emitEvent("rotationLog", map[string]string{"message": msg})

		if !rm.waitForDuration(durationSecs) {
			return // 중단됨
		}

		// 4. 완료 처리
		rm.mu.Lock()
		rm.completedCount++
		rm.currentIndex++
		rm.mu.Unlock()

		msg = fmt.Sprintf("%s - 사냥 시간 완료! (%d/%d)",
			char.Name, rm.completedCount, len(rm.characters))
		log.Printf("[순환] %s", msg)
		rm.emitEvent("rotationLog", map[string]string{"message": msg})

		// 다음 캐릭터가 있으면 전환
		if rm.currentIndex < len(rm.characters) {
			rm.setState(RotationSwitching)
			rm.emitEvent("rotationLog", map[string]string{
				"message": fmt.Sprintf("다음 캐릭터로 전환합니다: %s", rm.characters[rm.currentIndex].Name),
			})
			time.Sleep(2 * time.Second)
		}
	}

	// 모든 캐릭터 완료
	rm.mu.Lock()
	rm.state = RotationComplete
	rm.running = false
	rm.mu.Unlock()

	log.Println("[순환] 모든 캐릭터 순환 완료")
	rm.emitEvent("rotationLog", map[string]string{"message": "모든 캐릭터의 사냥이 완료되었습니다!"})
	rm.emitEvent("rotationComplete", nil)
	rm.emitEvent("rotationStatus", rm.GetStatus())
}

// waitForDuration 지정된 시간만큼 대기 (매초 상태 업데이트)
func (rm *RotationManager) waitForDuration(totalSeconds int) bool {
	for remaining := totalSeconds; remaining > 0; remaining-- {
		rm.mu.Lock()
		if !rm.running {
			rm.mu.Unlock()
			return false
		}
		rm.remainingSecs = remaining
		rm.mu.Unlock()

		// 상태 업데이트 (30초마다 + 마지막 10초)
		if remaining%30 == 0 || remaining <= 10 {
			rm.emitEvent("rotationStatus", rm.GetStatus())
		}

		select {
		case <-rm.stopChan:
			return false
		case <-time.After(1 * time.Second):
		}
	}

	rm.mu.Lock()
	rm.remainingSecs = 0
	rm.mu.Unlock()

	return true
}

// setState 상태 변경 및 이벤트 발송
func (rm *RotationManager) setState(state RotationState) {
	rm.mu.Lock()
	rm.state = state
	rm.mu.Unlock()
	rm.emitEvent("rotationStatus", rm.GetStatus())
}

// isStopped 중단 여부 확인
func (rm *RotationManager) isStopped() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return !rm.running
}

// stopWithError 오류로 인한 중지
func (rm *RotationManager) stopWithError(errMsg string) {
	rm.mu.Lock()
	rm.running = false
	rm.state = RotationIdle
	rm.mu.Unlock()
	rm.emitEvent("rotationStatus", rm.GetStatus())
}

// emitEvent 이벤트 발송
func (rm *RotationManager) emitEvent(eventType string, payload interface{}) {
	if rm.eventCallback != nil {
		rm.eventCallback(eventType, payload)
	}
}
