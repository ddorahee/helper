package automation

import (
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
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
// peachType: "" / "silla" / "king" / "india" (복숭아 사용 옵션)
func (ma *MouseAutomation) StartHunting(hwnd uint64, coords GameUICoords, dropdownIndex int, peachType string, logFn func(string), stopChan <-chan struct{}) error {
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

	// 1-4. 복숭아 시퀀스 (peachType이 설정되고 좌표가 있을 때만)
	if peachType != "" && (coords.SectButtonX != 0 || coords.SectButtonY != 0) {
		if stopped() {
			emit("=== 중단됨 ===")
			return fmt.Errorf("중단됨")
		}
		if err := ma.runPeachSequence(hwnd, coords, peachType, emit, waitOrStop, stopped); err != nil {
			emit(fmt.Sprintf("복숭아 시퀀스 실패 (무시): %v", err))
		}
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

	// 4-1. 부활 버튼 클릭 (설정된 경우, 시작 직전)
	if coords.ReviveX != 0 || coords.ReviveY != 0 {
		emit(fmt.Sprintf("④-1 부활 버튼 클릭 (%d, %d)", coords.ReviveX, coords.ReviveY))
		if err := ma.ClickRelative(hwnd, coords.ReviveX, coords.ReviveY); err != nil {
			log.Printf("[사냥시작] 부활 버튼 클릭 실패 (무시): %v", err)
		}
		if waitOrStop(2 * time.Second) {
			emit("=== 중단됨 ===")
			return fmt.Errorf("중단됨")
		}
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

	// 7. 자동사냥 시작 완료 → 창 최소화 (옵션 ON일 때만, 리소스 절감)
	if coords.MinimizeAfterStart {
		if waitOrStop(1 * time.Second) {
			emit("=== 중단됨 ===")
			return fmt.Errorf("중단됨")
		}
		emit("⑦ 자동사냥 시작됨 → 창 최소화 (옵션 ON)")
		if err := ma.wm.MinimizeWindow(hwnd); err != nil {
			log.Printf("[사냥시작] 창 최소화 실패 (무시): %v", err)
		}
	}

	emit("=== 시퀀스 완료 ===")
	return nil
}

// runPeachSequence 복숭아 자동 사용 시퀀스
// peachType: "silla" / "king" / "india" — 인벤토리에서 복숭아 더블클릭 후 누를 위쪽 화살표 횟수 결정
func (ma *MouseAutomation) runPeachSequence(hwnd uint64, coords GameUICoords, peachType string,
	emit func(string), waitOrStop func(time.Duration) bool, stopped func() bool) error {

	emit(fmt.Sprintf("==== 복숭아 시퀀스 시작 (타입=%s) ====", peachType))

	// 1. Alt+X (문파창 열기)
	emit("①-4-1 Alt+X (문파창 열기)")
	robotgo.KeyTap("x", "alt")
	if waitOrStop(1 * time.Second) {
		return fmt.Errorf("중단됨")
	}

	// 2. 문파 버튼 클릭
	if coords.SectButtonX != 0 || coords.SectButtonY != 0 {
		emit(fmt.Sprintf("①-4-2 문파 버튼 클릭 (%d, %d)", coords.SectButtonX, coords.SectButtonY))
		if err := ma.ClickRelative(hwnd, coords.SectButtonX, coords.SectButtonY); err != nil {
			log.Printf("[복숭아] 문파 클릭 실패 (무시): %v", err)
		}
		if waitOrStop(1 * time.Second) {
			return fmt.Errorf("중단됨")
		}
	}

	// 3. 복숭아 받기 클릭
	if coords.PeachReceiveX != 0 || coords.PeachReceiveY != 0 {
		emit(fmt.Sprintf("①-4-3 복숭아 받기 클릭 (%d, %d)", coords.PeachReceiveX, coords.PeachReceiveY))
		if err := ma.ClickRelative(hwnd, coords.PeachReceiveX, coords.PeachReceiveY); err != nil {
			log.Printf("[복숭아] 복숭아 받기 클릭 실패 (무시): %v", err)
		}
		if waitOrStop(1 * time.Second) {
			return fmt.Errorf("중단됨")
		}
	}

	// 4. 받기 클릭
	if coords.ReceiveAcceptX != 0 || coords.ReceiveAcceptY != 0 {
		emit(fmt.Sprintf("①-4-4 받기 클릭 (%d, %d)", coords.ReceiveAcceptX, coords.ReceiveAcceptY))
		if err := ma.ClickRelative(hwnd, coords.ReceiveAcceptX, coords.ReceiveAcceptY); err != nil {
			log.Printf("[복숭아] 받기 클릭 실패 (무시): %v", err)
		}
		if waitOrStop(1 * time.Second) {
			return fmt.Errorf("중단됨")
		}
	}

	// 4-1. 받기 종료: ESC → Enter → ESC (수령 알림 닫기)
	emit("①-4-4-1 받기 종료 시퀀스: ESC → Enter → ESC")
	robotgo.KeyTap("escape")
	if waitOrStop(400 * time.Millisecond) {
		return fmt.Errorf("중단됨")
	}
	robotgo.KeyTap("enter")
	if waitOrStop(400 * time.Millisecond) {
		return fmt.Errorf("중단됨")
	}
	robotgo.KeyTap("escape")
	if waitOrStop(500 * time.Millisecond) {
		return fmt.Errorf("중단됨")
	}

	// 5. ESC (받기 창 닫기 — 잔여 팝업 대비)
	emit("①-4-5 ESC (받기 창 닫기)")
	robotgo.KeyTap("escape")
	if waitOrStop(500 * time.Millisecond) {
		return fmt.Errorf("중단됨")
	}

	// 6. i (인벤토리 열기)
	emit("①-4-6 i (인벤토리 열기)")
	robotgo.KeyTap("i")
	if waitOrStop(1 * time.Second) {
		return fmt.Errorf("중단됨")
	}

	// 7. 인벤토리에서 복숭아 검색 + 더블클릭
	if !ma.findAndDoubleClickPeach(hwnd, emit, waitOrStop, stopped) {
		emit("①-4-7 복숭아를 인벤토리에서 찾지 못함 — 시퀀스 종료")
		robotgo.KeyTap("escape")
		return nil
	}

	if waitOrStop(1 * time.Second) {
		return fmt.Errorf("중단됨")
	}

	// 8. 캐릭터 타입별 화살표 + 엔터
	upCount := 0
	switch peachType {
	case "silla":
		upCount = 2
	case "king":
		upCount = 3
	case "india":
		upCount = 4
	}
	if upCount > 0 {
		emit(fmt.Sprintf("①-4-8 위쪽 화살표 %d회 + Enter + Enter (타입=%s)", upCount, peachType))
		for i := 0; i < upCount; i++ {
			if stopped() {
				return fmt.Errorf("중단됨")
			}
			robotgo.KeyTap("up")
			time.Sleep(150 * time.Millisecond)
		}
		time.Sleep(300 * time.Millisecond)
		robotgo.KeyTap("enter")
		time.Sleep(500 * time.Millisecond)
		robotgo.KeyTap("enter")
		if waitOrStop(1 * time.Second) {
			return fmt.Errorf("중단됨")
		}
	}

	// 9. ESC (인벤토리 닫기)
	emit("①-4-9 ESC (인벤토리 닫기)")
	robotgo.KeyTap("escape")
	if waitOrStop(500 * time.Millisecond) {
		return fmt.Errorf("중단됨")
	}

	emit("==== 복숭아 시퀀스 완료 ====")
	return nil
}

// findAndDoubleClickPeach 인벤토리 내에서 복숭아 아이콘을 찾아 더블클릭
// 매번 캡처 + 다중 스케일 이미지 매칭 후 못 찾으면 PageUp/PageDown으로 페이지 넘기며 탐색
// PageUp 15회 → PageDown 30회 시도 후 미발견시 false 반환
func (ma *MouseAutomation) findAndDoubleClickPeach(hwnd uint64, emit func(string), waitOrStop func(time.Duration) bool, stopped func() bool) bool {
	peachPath := "config/peach.png"
	needle, err := LoadPNG(peachPath)
	if err != nil {
		emit(fmt.Sprintf("①-4-7 %s 로드 실패: %v", peachPath, err))
		return false
	}
	emit(fmt.Sprintf("①-4-7 needle 로드 완료 %dx%d (다중 스케일 매칭)",
		needle.Bounds().Dx(), needle.Bounds().Dy()))

	debugDir := filepath.Join(os.Getenv("USERPROFILE"), "Downloads", "Temp")
	os.MkdirAll(debugDir, 0755)
	debugCounter := 0

	tryMatch := func() (bool, int, int) {
		raw, _, err := ma.wm.CaptureWindowRaw(hwnd)
		if err != nil {
			return false, 0, 0
		}
		// 다중 스케일 + 완화된 임계값 (작은 아이콘은 픽셀 변동에 민감)
		x, y, scale, found := FindImageMultiScale(raw, needle, nil, 80, 0.75)
		if !found {
			// 디버그: 처음 3회 매칭 실패 시 게임 캡처 저장
			if debugCounter < 3 {
				debugCounter++
				debugPath := filepath.Join(debugDir, fmt.Sprintf("peach_miss_%d.png", debugCounter))
				if f, e := os.Create(debugPath); e == nil {
					_ = png.Encode(f, raw)
					f.Close()
					emit(fmt.Sprintf("①-4-7 매칭 실패 디버그 이미지: %s", debugPath))
				}
			}
			return false, 0, 0
		}
		nb := needle.Bounds()
		// 스케일된 아이콘의 중심 좌표
		cx := x + int(float64(nb.Dx())*scale)/2
		cy := y + int(float64(nb.Dy())*scale)/2
		emit(fmt.Sprintf("①-4-7 매칭 성공 scale=%.2f at (%d,%d) → 중심 (%d,%d)", scale, x, y, cx, cy))
		return true, cx, cy
	}

	doubleClickAt := func(cx, cy int) bool {
		emit(fmt.Sprintf("①-4-7 복숭아 발견 → 더블클릭 (상대=%d,%d)", cx, cy))
		if err := ma.DoubleClickRelative(hwnd, cx, cy); err != nil {
			emit(fmt.Sprintf("①-4-7 더블클릭 실패: %v", err))
			return false
		}
		return true
	}

	// 0회: 현재 위치에서 매칭
	if found, cx, cy := tryMatch(); found {
		return doubleClickAt(cx, cy)
	}

	// PageUp 15회 탐색 (위쪽 페이지)
	emit("①-4-7 인벤토리 PageUp 탐색 (최대 15회)")
	for i := 1; i <= 15; i++ {
		if stopped() {
			return false
		}
		robotgo.KeyTap("pageup")
		if waitOrStop(250 * time.Millisecond) {
			return false
		}
		if found, cx, cy := tryMatch(); found {
			return doubleClickAt(cx, cy)
		}
	}

	// PageDown 30회 탐색 (위로 올라간 만큼 + 아래 페이지)
	emit("①-4-7 인벤토리 PageDown 탐색 (최대 30회)")
	for i := 1; i <= 30; i++ {
		if stopped() {
			return false
		}
		robotgo.KeyTap("pagedown")
		if waitOrStop(250 * time.Millisecond) {
			return false
		}
		if found, cx, cy := tryMatch(); found {
			return doubleClickAt(cx, cy)
		}
	}

	return false
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
	ReviveX            int // 부활 버튼 X
	ReviveY            int // 부활 버튼 Y
	SectButtonX        int // 문파 버튼 X
	SectButtonY        int // 문파 버튼 Y
	PeachReceiveX      int // 복숭아 받기 X
	PeachReceiveY      int // 복숭아 받기 Y
	ReceiveAcceptX     int // 받기/수락 X
	ReceiveAcceptY     int // 받기/수락 Y

	// 사냥 시작 후 창 최소화 (리소스 절감)
	MinimizeAfterStart bool
}
