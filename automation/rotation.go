package automation

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// RotationState 자동 사냥 상태
type RotationState int

const (
	RotationIdle       RotationState = iota // 대기
	RotationActivating                      // 창 활성화 중
	RotationStarting                        // 사냥 시작 중
	RotationHunting                         // 사냥 중 (타이머 대기)
	RotationSwitching                       // 다음 캐릭터로 전환 중
	RotationComplete                        // 모든 캐릭터 완료
)

// RotationCharacter 자동 사냥에 참여하는 캐릭터 정보
type RotationCharacter struct {
	ID             string
	Name           string
	HuntingArea    string
	DropdownIndex  int
	DurationMins   int
	Order          int
	WindowHWND     uint64
	PeachType      string // "" / "silla" / "king" / "india"
	CompanionMode  string // "" / "kanchen" / "daeya" — 설정 시 자동사냥 순환에서 빠지고 DurationMins 동안 메인화면 자동화를 병행 실행
	HuntAfterMins  int    // 동시실행 캐릭 전용: 다른 캐릭 사냥이 다 끝난 뒤 이 시간(분)만큼 자동사냥 (0 = 동시실행만)
}

// RotationStatus 현재 자동 사냥 상태 정보
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

// CompanionController 자동사냥 도는 동안 동시실행 캐릭터 창에서
// 메인화면 자동화(칸첸/대야)를 병행 실행하기 위한 훅.
// Start는 지정 창에서 메인화면 로직을 시작하고, Stop은 완전히 멈출 때까지
// (잔여 키 입력 소진 포함) 블로킹해야 한다 — 이후 로테이션이 창을 전환하므로.
type CompanionController interface {
	StartCompanion(mode string, hwnd uint64)
	StopCompanion()
}

// RotationManager 자동 사냥 관리자
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
	watcher        *DisconnectWatcher

	companionCtl CompanionController

	// 동시실행(메인화면 병행) 상태 — 동시실행 캐릭은 자동사냥 순환에서 빠지고,
	// 자기 DurationMins 동안 칸첸/대야를 돈다. 로테이션이 포그라운드를 쓰는
	// 전환 구간에만 일시정지했다가 누적 시간 이어서 재개. 여러 명이면 순서대로.
	compMu        sync.Mutex
	compChars     []RotationCharacter
	compIdx       int           // 현재 동시실행 캐릭 인덱스
	compRemaining time.Duration // 현재 캐릭의 남은 실행 시간
	compSegStart  time.Time     // 현재 세그먼트 시작 시각
	compRunning   bool          // 세그먼트 실행 중 여부
	compTimer     *time.Timer   // 남은 시간 만료 타이머
	compDone      chan struct{} // 모든 동시실행 완료 시 close
	compStopped   bool          // 종료됨 (재시작 방지)
}

// NewRotationManager 새로운 자동 사냥 관리자 생성
func NewRotationManager(wm *WindowManager, ma *MouseAutomation) *RotationManager {
	rm := &RotationManager{
		state:   RotationIdle,
		running: false,
		window:  wm,
		mouse:   ma,
	}
	rm.watcher = NewDisconnectWatcher(wm, ma)
	rm.watcher.SetOnReconnected(func(hwnd uint64) {
		// 재접속 후 현재 사냥 중인 캐릭터의 사냥 시퀀스 재실행
		rm.mu.RLock()
		coords := rm.coords
		var dropdownIndex int
		var peachType string
		for _, c := range rm.characters {
			if c.WindowHWND == hwnd {
				dropdownIndex = c.DropdownIndex
				peachType = c.PeachType
				break
			}
		}
		rm.mu.RUnlock()

		stopChan := make(chan struct{}) // 워처용 임시 stopChan
		_ = ma.StartHunting(hwnd, coords, dropdownIndex, peachType, func(msg string) {
			rm.emitEvent("rotationLog", map[string]string{"message": "[재접속] " + msg})
		}, stopChan)
	})
	return rm
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

// SetCompanionController 병행 메인화면 컨트롤러 주입 (main.go 배선)
func (rm *RotationManager) SetCompanionController(ctl CompanionController) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.companionCtl = ctl
}

// ===== 동시실행(메인화면 병행) 엔진 =====
//
// 동시실행 캐릭은 자동사냥 순환에서 빠지고, 자기 DurationMins 동안 메인화면
// 자동화(칸첸/대야)를 돈다. 자동사냥 캐릭 전환(창 활성화 + 사냥 시작) 동안만
// suspend로 일시정지하고, 끝나면 resume으로 누적 시간 이어서 재개한다.
// 캐릭의 시간이 다 차면 다음 동시실행 캐릭으로 넘어간다.

// initCompanions Start()에서 동시실행 캐릭 목록 초기화
func (rm *RotationManager) initCompanions(comp []RotationCharacter) {
	rm.compMu.Lock()
	defer rm.compMu.Unlock()
	rm.compChars = comp
	rm.compIdx = 0
	rm.compRunning = false
	rm.compStopped = false
	rm.compTimer = nil
	rm.compDone = make(chan struct{})
	if len(comp) == 0 {
		close(rm.compDone) // 동시실행 캐릭 없음 — 이미 완료 상태
	} else {
		rm.compRemaining = time.Duration(comp[0].DurationMins) * time.Minute
	}
}

// companionResume 현재 동시실행 캐릭의 세그먼트 시작/재개 (포그라운드가 빌 때 호출)
func (rm *RotationManager) companionResume() {
	rm.compMu.Lock()
	defer rm.compMu.Unlock()
	rm.companionResumeLocked()
}

func (rm *RotationManager) companionResumeLocked() {
	if rm.compStopped || rm.compRunning || rm.compIdx >= len(rm.compChars) || rm.companionCtl == nil {
		return
	}
	c := rm.compChars[rm.compIdx]
	modeName := "칸첸"
	if c.CompanionMode == "daeya" {
		modeName = "대야"
	}
	rm.emitEvent("rotationLog", map[string]string{
		"message": fmt.Sprintf("[동시실행] %s — 메인화면 %s 실행 (남은 %.0f분)",
			c.Name, modeName, rm.compRemaining.Minutes()),
	})
	rm.compRunning = true
	rm.compSegStart = time.Now()
	rm.compTimer = time.AfterFunc(rm.compRemaining, rm.companionTimeUp)
	rm.companionCtl.StartCompanion(c.CompanionMode, c.WindowHWND)
}

// companionSuspend 세그먼트 일시정지 + 경과 시간 차감 (로테이션이 포그라운드 쓰기 직전 호출).
// 완전히 멈출 때까지 블로킹 — 이후 창 전환 시 키 입력이 새지 않도록.
func (rm *RotationManager) companionSuspend() {
	rm.compMu.Lock()
	defer rm.compMu.Unlock()
	if !rm.compRunning {
		return
	}
	if rm.compTimer != nil {
		rm.compTimer.Stop()
		rm.compTimer = nil
	}
	rm.companionCtl.StopCompanion()
	rm.compRemaining -= time.Since(rm.compSegStart)
	rm.compRunning = false
	if rm.compRemaining <= 0 {
		rm.companionFinishCurrentLocked()
		return
	}
	c := rm.compChars[rm.compIdx]
	rm.emitEvent("rotationLog", map[string]string{
		"message": fmt.Sprintf("[동시실행] %s 일시정지 (캐릭터 전환) — 남은 %.0f분, 전환 후 재개",
			c.Name, rm.compRemaining.Minutes()),
	})
}

// companionTimeUp 현재 캐릭의 시간이 다 참 (타이머 콜백) — 다음 캐릭으로
func (rm *RotationManager) companionTimeUp() {
	rm.compMu.Lock()
	defer rm.compMu.Unlock()
	if !rm.compRunning || rm.compStopped {
		return
	}
	rm.compTimer = nil
	rm.companionCtl.StopCompanion()
	rm.compRunning = false
	rm.compRemaining = 0
	rm.companionFinishCurrentLocked()
	// 포그라운드가 비어 있는 상태(사냥 대기 중)이므로 다음 캐릭 바로 시작
	rm.companionResumeLocked()
}

// companionFinishCurrentLocked 현재 캐릭 완료 처리 + 다음 캐릭 준비 (compMu 보유 상태에서 호출)
func (rm *RotationManager) companionFinishCurrentLocked() {
	if rm.compIdx >= len(rm.compChars) {
		return
	}
	c := rm.compChars[rm.compIdx]
	rm.emitEvent("rotationLog", map[string]string{
		"message": fmt.Sprintf("[동시실행] %s 완료 (%d분)", c.Name, c.DurationMins),
	})
	rm.compIdx++
	if rm.compIdx >= len(rm.compChars) {
		select {
		case <-rm.compDone:
		default:
			close(rm.compDone)
		}
		return
	}
	rm.compRemaining = time.Duration(rm.compChars[rm.compIdx].DurationMins) * time.Minute
}

// companionYieldFor 사냥 턴이 온 캐릭이 현재 동시실행 중인 캐릭과 같으면
// 동시실행을 종료 처리한다 (남은 시간은 버리고 자동사냥으로 전환).
func (rm *RotationManager) companionYieldFor(hwnd uint64) {
	rm.compMu.Lock()
	defer rm.compMu.Unlock()
	if rm.compStopped || rm.compIdx >= len(rm.compChars) {
		return
	}
	cur := rm.compChars[rm.compIdx]
	if cur.WindowHWND != hwnd {
		return
	}
	if rm.compRunning {
		if rm.compTimer != nil {
			rm.compTimer.Stop()
			rm.compTimer = nil
		}
		if rm.companionCtl != nil {
			rm.companionCtl.StopCompanion()
		}
		rm.compRemaining -= time.Since(rm.compSegStart)
		rm.compRunning = false
	}
	if rm.compRemaining > 0 {
		rm.emitEvent("rotationLog", map[string]string{
			"message": fmt.Sprintf("[동시실행] %s — 자기 자동사냥 차례가 되어 동시실행 종료 (남은 %.0f분 생략)",
				cur.Name, rm.compRemaining.Minutes()),
		})
	}
	rm.compRemaining = 0
	rm.companionFinishCurrentLocked()
}

// companionShutdown 동시실행 완전 종료 (로테이션 종료/중지 시)
func (rm *RotationManager) companionShutdown() {
	rm.compMu.Lock()
	defer rm.compMu.Unlock()
	rm.compStopped = true
	if rm.compTimer != nil {
		rm.compTimer.Stop()
		rm.compTimer = nil
	}
	if rm.compRunning && rm.companionCtl != nil {
		rm.companionCtl.StopCompanion()
	}
	rm.compRunning = false
}

// companionDoneChan 완료 대기용 채널 반환
func (rm *RotationManager) companionDoneChan() chan struct{} {
	rm.compMu.Lock()
	defer rm.compMu.Unlock()
	return rm.compDone
}

// Start 자동 사냥 시작
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

	// 동시실행 캐릭(CompanionMode 설정) 분리 — 자동사냥 순환에는 나머지만 참여.
	// 단, HuntAfterMins가 설정된 동시실행 캐릭은 다른 캐릭 사냥이 모두 끝난 뒤
	// 그 시간만큼 자동사냥 턴을 받는다 (사냥 큐 맨 뒤에 추가).
	var huntChars, compChars, afterHunts []RotationCharacter
	for _, c := range characters {
		if c.CompanionMode == "kanchen" || c.CompanionMode == "daeya" {
			compChars = append(compChars, c)
			if c.HuntAfterMins > 0 {
				h := c
				h.CompanionMode = "" // 사냥 턴 엔트리
				h.DurationMins = c.HuntAfterMins
				afterHunts = append(afterHunts, h)
			}
		} else {
			huntChars = append(huntChars, c)
		}
	}
	huntChars = append(huntChars, afterHunts...)

	rm.characters = huntChars
	rm.coords = coords
	rm.currentIndex = 0
	rm.completedCount = 0
	rm.running = true
	rm.state = RotationIdle
	rm.stopChan = make(chan struct{})

	rm.mu.Unlock()

	rm.initCompanions(compChars)

	// 팅김 감지 워처 시작 (자동사냥 캐릭터의 hwnd 감시 — 동시실행 캐릭은
	// 재접속 시 StartHunting을 잘못 실행하면 안 되므로 제외)
	if rm.watcher != nil {
		hwnds := make([]uint64, 0, len(huntChars))
		for _, c := range huntChars {
			hwnds = append(hwnds, c.WindowHWND)
		}
		rm.watcher.SetLogFunc(func(msg string) {
			rm.emitEvent("rotationLog", map[string]string{"message": "[팅김감지] " + msg})
		})
		rm.watcher.Start(hwnds)
	}

	// 자동 사냥 고루틴 시작
	go rm.runRotation()

	return nil
}

// Stop 자동 사냥 중지
func (rm *RotationManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.running {
		return
	}

	rm.running = false
	close(rm.stopChan)
	rm.state = RotationIdle
	rm.emitEvent("rotationLog", map[string]string{"message": "자동 사냥이 중지되었습니다."})
	rm.emitEvent("rotationStatus", rm.buildStatusLocked())

	// 팅김 감지 워처도 함께 중지
	if rm.watcher != nil {
		go rm.watcher.Stop()
	}
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

// runRotation 자동 사냥 메인 루프
func (rm *RotationManager) runRotation() {
	log.Println("[자동사냥] 자동 사냥 시작")
	rm.emitEvent("rotationLog", map[string]string{"message": "자동 사냥을 시작합니다."})

	// 종료(정상/중지/에러) 시 동시실행도 반드시 정리
	defer rm.companionShutdown()

	// 시작 시 모든 캐릭터 창을 최소화 (옵션 ON일 때만, 리소스 절감)
	rm.mu.RLock()
	minimizeOn := rm.coords.MinimizeAfterStart
	allHwnds := make([]uint64, 0, len(rm.characters))
	for _, c := range rm.characters {
		allHwnds = append(allHwnds, c.WindowHWND)
	}
	rm.mu.RUnlock()
	if minimizeOn {
		for _, h := range allHwnds {
			if err := rm.window.MinimizeWindow(h); err != nil {
				log.Printf("[자동사냥] 초기 최소화 실패 (무시): %v", err)
			}
		}
		rm.emitEvent("rotationLog", map[string]string{"message": fmt.Sprintf("모든 캐릭터 창 최소화 완료 (%d개)", len(allHwnds))})
		time.Sleep(500 * time.Millisecond)
	} else {
		rm.emitEvent("rotationLog", map[string]string{"message": "최소화 옵션 OFF — 기존 방식으로 진행"})
	}

	for rm.currentIndex < len(rm.characters) {
		rm.mu.RLock()
		if !rm.running {
			rm.mu.RUnlock()
			return
		}
		char := rm.characters[rm.currentIndex]
		rm.mu.RUnlock()

		// 0. 이 캐릭이 현재 동시실행 중이면 종료 처리 (자기 사냥 턴 — HuntAfterMins 케이스),
		//    그 외 동시실행은 일시정지 — 창 전환/사냥 시작 동안 키 입력이 새지 않도록
		//    (잔여 키 입력 소진까지 블로킹). 남은 시간은 전환 후 재개.
		rm.companionYieldFor(char.WindowHWND)
		rm.companionSuspend()

		// 1. 창 활성화
		rm.setState(RotationActivating)
		msg := fmt.Sprintf("[%d/%d] %s - %s 창 활성화 중...",
			rm.currentIndex+1, len(rm.characters), char.Name, char.HuntingArea)
		log.Printf("[자동사냥] %s", msg)
		rm.emitEvent("rotationLog", map[string]string{"message": msg})

		if err := rm.window.ActivateWindow(char.WindowHWND); err != nil {
			errMsg := fmt.Sprintf("%s 창 활성화 실패: %v", char.Name, err)
			log.Printf("[자동사냥] 오류: %s", errMsg)
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
		log.Printf("[자동사냥] %s", msg)
		rm.emitEvent("rotationLog", map[string]string{"message": msg})

		logFn := func(msg string) {
			rm.emitEvent("rotationLog", map[string]string{"message": fmt.Sprintf("[%s] %s", char.Name, msg)})
		}
		if err := rm.mouse.StartHunting(char.WindowHWND, rm.coords, char.DropdownIndex, char.PeachType, logFn, rm.stopChan); err != nil {
			if rm.isStopped() {
				return
			}
			errMsg := fmt.Sprintf("%s 사냥 시작 실패: %v", char.Name, err)
			log.Printf("[자동사냥] 오류: %s", errMsg)
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
		log.Printf("[자동사냥] %s", msg)
		rm.emitEvent("rotationLog", map[string]string{"message": msg})

		// 사냥은 게임 내 기능이라 포그라운드 불필요 → 대기 동안
		// 동시실행 캐릭의 메인화면(칸첸/대야)을 재개 (누적 시간 이어서)
		rm.companionResume()

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
		log.Printf("[자동사냥] %s", msg)
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

	// 자동사냥 캐릭 모두 완료 — 동시실행 시간이 남았으면 마저 채운다
	done := rm.companionDoneChan()
	select {
	case <-done:
		// 동시실행도 이미 완료 (또는 없음)
	default:
		rm.companionResume() // 자동사냥 캐릭이 0명이었거나 마지막 전환 직후면 여기서 시작
		rm.emitEvent("rotationLog", map[string]string{"message": "자동사냥 캐릭 완료 — 동시실행 남은 시간을 마저 채웁니다."})
		select {
		case <-done:
		case <-rm.stopChan:
			return
		}
	}

	// 모든 캐릭터 완료
	rm.mu.Lock()
	rm.state = RotationComplete
	rm.running = false
	rm.mu.Unlock()

	log.Println("[자동사냥] 모든 캐릭터 자동 사냥 완료")
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
