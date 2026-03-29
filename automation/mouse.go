package automation

import (
	"fmt"
	"log"
	"time"

	"github.com/go-vgo/robotgo"
)

// MouseAutomation 마우스 자동화
type MouseAutomation struct {
	wm        *WindowManager
	clickDelay time.Duration
}

// NewMouseAutomation 새로운 마우스 자동화 생성
func NewMouseAutomation(wm *WindowManager) *MouseAutomation {
	return &MouseAutomation{
		wm:        wm,
		clickDelay: 500 * time.Millisecond,
	}
}

// ClickAt 절대 좌표로 클릭
func (ma *MouseAutomation) ClickAt(x, y int) {
	log.Printf("[마우스] ClickAt 절대좌표: (%d, %d)", x, y)
	robotgo.MoveMouse(x, y)
	time.Sleep(300 * time.Millisecond)
	robotgo.Click("left")
	time.Sleep(ma.clickDelay)
}

// DoubleClickAt 절대 좌표로 더블클릭
func (ma *MouseAutomation) DoubleClickAt(x, y int) {
	log.Printf("[마우스] DoubleClickAt 절대좌표: (%d, %d)", x, y)
	robotgo.MoveMouse(x, y)
	time.Sleep(300 * time.Millisecond)
	robotgo.Click("left", true) // true = double click
	time.Sleep(ma.clickDelay)
}

// DoubleClickRelative 창 상대좌표로 더블클릭
func (ma *MouseAutomation) DoubleClickRelative(hwnd uint64, relX, relY int) error {
	rect, err := ma.wm.GetWindowRect(hwnd)
	if err != nil {
		return fmt.Errorf("창 위치 가져오기 실패: %v", err)
	}

	absX := int(rect.Left) + relX
	absY := int(rect.Top) + relY

	log.Printf("[마우스] DoubleClickRelative hwnd=%d 상대=(%d,%d) → 절대=(%d,%d)",
		hwnd, relX, relY, absX, absY)

	ma.DoubleClickAt(absX, absY)
	return nil
}

// ClickRelative 창 상대좌표로 클릭
func (ma *MouseAutomation) ClickRelative(hwnd uint64, relX, relY int) error {
	rect, err := ma.wm.GetWindowRect(hwnd)
	if err != nil {
		return fmt.Errorf("창 위치 가져오기 실패: %v", err)
	}

	absX := int(rect.Left) + relX
	absY := int(rect.Top) + relY

	log.Printf("[마우스] ClickRelative hwnd=%d 창Rect=(%d,%d,%d,%d) 상대=(%d,%d) → 절대=(%d,%d)",
		hwnd, rect.Left, rect.Top, rect.Right, rect.Bottom, relX, relY, absX, absY)

	ma.ClickAt(absX, absY)
	return nil
}

// StartHunting 사냥 시작 시퀀스 실행 (칼 클릭 → 사냥터 선택 → 시작 → 확인)
// logFn: 로그에 메시지를 전송하는 콜백 (nil이면 로그만)
// stopChan: 중단 신호 채널 (nil이면 중단 불가)
func (ma *MouseAutomation) StartHunting(hwnd uint64, coords GameUICoords, dropdownIndex int, logFn func(string), stopChan <-chan struct{}) error {
	emit := func(msg string) {
		log.Printf("[사냥시작] %s", msg)
		if logFn != nil {
			logFn(msg)
		}
	}

	// 중단 체크 헬퍼
	stopped := func() bool {
		if stopChan == nil {
			return false
		}
		select {
		case <-stopChan:
			return true
		default:
			return false
		}
	}

	// 대기 + 중단 체크 헬퍼
	waitOrStop := func(d time.Duration) bool {
		if stopChan == nil {
			time.Sleep(d)
			return false
		}
		select {
		case <-stopChan:
			return true
		case <-time.After(d):
			return false
		}
	}

	emit(fmt.Sprintf("=== 시퀀스 시작 === dropdownIndex=%d", dropdownIndex))

	// 1. 창 활성화
	emit("① 창 활성화...")
	if err := ma.wm.ActivateWindow(hwnd); err != nil {
		return fmt.Errorf("창 활성화 실패: %v", err)
	}
	if waitOrStop(500 * time.Millisecond) {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}

	// 1-1. ESC 입력 (기존 팝업/다이얼로그 닫기)
	emit("①-1 ESC 입력 (기존 팝업 닫기)")
	robotgo.KeyTap("escape")
	if waitOrStop(1 * time.Second) {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}

	// 1-2. 채굴 확인 버튼 클릭
	if coords.AlertConfirmX != 0 || coords.AlertConfirmY != 0 {
		emit(fmt.Sprintf("①-2 채굴 확인 버튼 클릭 (%d, %d)", coords.AlertConfirmX, coords.AlertConfirmY))
		if err := ma.ClickRelative(hwnd, coords.AlertConfirmX, coords.AlertConfirmY); err != nil {
			log.Printf("[사냥시작] 채굴 확인 버튼 클릭 실패 (무시): %v", err)
		}
		// 채굴 타이머 완료 대기 (15초)
		emit("①-2 채굴 확인 후 15초 대기...")
		if waitOrStop(15 * time.Second) {
			emit("=== 중단됨 ===")
			return fmt.Errorf("중단됨")
		}
	}

	// 1-3. ESC 한번 더 (혹시 남은 팝업 닫기)
	robotgo.KeyTap("escape")
	if waitOrStop(500 * time.Millisecond) {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}

	// 2. 칼 버튼 클릭 (자동사냥 대화창 열기)
	if stopped() {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}
	emit(fmt.Sprintf("② 칼 버튼 클릭 (%d, %d)", coords.SwordButtonX, coords.SwordButtonY))
	if err := ma.ClickRelative(hwnd, coords.SwordButtonX, coords.SwordButtonY); err != nil {
		return fmt.Errorf("칼 버튼 클릭 실패: %v", err)
	}
	if waitOrStop(2 * time.Second) {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}

	// 3. 드롭다운 화살표 클릭 (사냥터 목록 펼치기)
	if stopped() {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}
	emit(fmt.Sprintf("③ 드롭다운 화살표 클릭 (%d, %d)", coords.DropdownArrowX, coords.DropdownArrowY))
	if err := ma.ClickRelative(hwnd, coords.DropdownArrowX, coords.DropdownArrowY); err != nil {
		return fmt.Errorf("드롭다운 클릭 실패: %v", err)
	}
	if waitOrStop(2 * time.Second) {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}

	// 4. 사냥터 항목 클릭 (인덱스 기반: 첫 항목 Y + 인덱스 × 항목 높이)
	if stopped() {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}
	itemY := coords.DropdownFirstItemY + (dropdownIndex * coords.DropdownItemHeight)
	emit(fmt.Sprintf("④ 사냥터 항목 클릭 (X=%d, Y=%d) [인덱스%d]", coords.DropdownArrowX, itemY, dropdownIndex))
	if err := ma.ClickRelative(hwnd, coords.DropdownArrowX, itemY); err != nil {
		return fmt.Errorf("사냥터 선택 실패: %v", err)
	}
	if waitOrStop(2 * time.Second) {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}

	// 5. 시작 버튼 클릭
	if stopped() {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}
	emit(fmt.Sprintf("⑤ 시작 버튼 클릭 (%d, %d)", coords.StartButtonX, coords.StartButtonY))
	if err := ma.ClickRelative(hwnd, coords.StartButtonX, coords.StartButtonY); err != nil {
		return fmt.Errorf("시작 버튼 클릭 실패: %v", err)
	}
	if waitOrStop(2 * time.Second) {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}

	// 6. 확인 버튼 클릭 ("자동사냥을 시작하시겠습니까?" 대화창)
	if stopped() {
		emit("=== 중단됨 ===")
		return fmt.Errorf("중단됨")
	}
	if coords.ConfirmButtonX != 0 || coords.ConfirmButtonY != 0 {
		emit(fmt.Sprintf("⑥ 확인 버튼 클릭 (%d, %d)", coords.ConfirmButtonX, coords.ConfirmButtonY))
		if err := ma.ClickRelative(hwnd, coords.ConfirmButtonX, coords.ConfirmButtonY); err != nil {
			return fmt.Errorf("확인 버튼 클릭 실패: %v", err)
		}
		if waitOrStop(2 * time.Second) {
			emit("=== 중단됨 ===")
			return fmt.Errorf("중단됨")
		}
	} else {
		emit("⑥ 확인 버튼 좌표 미설정 - 건너뜀")
	}

	emit("=== 시퀀스 완료 ===")
	return nil
}

// GetMousePosition 현재 마우스 절대좌표 반환
func (ma *MouseAutomation) GetMousePosition() (int, int) {
	x, y := robotgo.Location()
	return x, y
}

// CaptureClickPosition 지정된 시간 후 마우스 클릭 위치를 캡처하여 창 상대좌표로 반환
func (ma *MouseAutomation) CaptureClickPosition(hwnd uint64, delaySec int) (int, int, error) {
	// 대기
	time.Sleep(time.Duration(delaySec) * time.Second)

	// 현재 마우스 위치 가져오기
	absX, absY := robotgo.Location()

	// 창 위치 가져오기
	rect, err := ma.wm.GetWindowRect(hwnd)
	if err != nil {
		return 0, 0, fmt.Errorf("창 위치 가져오기 실패: %v", err)
	}

	// 상대좌표 계산
	relX := absX - int(rect.Left)
	relY := absY - int(rect.Top)

	return relX, relY, nil
}

// GameUICoords 게임 UI 좌표 (config 패키지 순환 참조 방지용)
// 모든 좌표는 게임 창 기준 상대좌표 (ClickRelative가 매번 GetWindowRect로 절대좌표 재계산)
type GameUICoords struct {
	SwordButtonX       int
	SwordButtonY       int
	DropdownArrowX     int
	DropdownArrowY     int
	DropdownItemHeight int // 드롭다운 항목 간 높이 (픽셀)
	DropdownFirstItemY int // 첫 번째 드롭다운 항목의 Y좌표
	StartButtonX       int
	StartButtonY       int
	ConfirmButtonX     int // "자동사냥을 시작하시겠습니까?" 확인 버튼 X
	ConfirmButtonY     int // "자동사냥을 시작하시겠습니까?" 확인 버튼 Y
	AlertConfirmX      int // 채굴 확인 버튼 X (ESC 후 나타나는 팝업)
	AlertConfirmY      int // 채굴 확인 버튼 Y
}
