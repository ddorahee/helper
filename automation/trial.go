package automation

import (
	"fmt"
	"image"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
)

// TrialConfig 시련 자동화 설정
type TrialConfig struct {
	MaxRuns int // 최대 반복 횟수 (기본 10)
	TargetX int // 목표 좌표 X
	TargetY int // 목표 좌표 Y
}

// DefaultTrialConfig 기본 시련 설정
func DefaultTrialConfig() TrialConfig {
	return TrialConfig{
		MaxRuns: 10,
		TargetX: 29,
		TargetY: 32,
	}
}

// Trial 시련 자동화 관리자
type Trial struct {
	om     *OCRManager
	km     *KeyboardManager
	wm     *WindowManager
	config TrialConfig

	hwnd       uint64 // 솔로
	leaderHwnd uint64 // 그룹
	memberHwnd uint64
	isGroup    bool

	stopChan chan struct{}
	running  bool
	runCount int
	mutex    sync.Mutex
	logFunc  func(string)
}

// NewTrial 새로운 시련 관리자 생성
func NewTrial(om *OCRManager, km *KeyboardManager, wm *WindowManager) *Trial {
	return &Trial{
		om:     om,
		km:     km,
		wm:     wm,
		config: DefaultTrialConfig(),
	}
}

// GetConfig 현재 설정 반환
func (t *Trial) GetConfig() TrialConfig {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.config
}

// SetConfig 설정 업데이트
func (t *Trial) SetConfig(cfg TrialConfig) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.config = cfg
}

// SetLogFunc 로그 콜백 설정
func (t *Trial) SetLogFunc(f func(string)) {
	t.logFunc = f
}

func (t *Trial) log(msg string) {
	log.Printf("[시련] %s", msg)
	if t.logFunc != nil {
		t.logFunc(msg)
	}
}

// IsRunning 실행 중 여부
func (t *Trial) IsRunning() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.running
}

func (t *Trial) isStopped() bool {
	select {
	case <-t.stopChan:
		return true
	default:
		return false
	}
}

// Start 솔로 시련 시작
func (t *Trial) Start(hwnd uint64) {
	t.mutex.Lock()
	if t.running {
		t.mutex.Unlock()
		return
	}
	t.running = true
	t.hwnd = hwnd
	t.isGroup = false
	t.runCount = 0
	t.stopChan = make(chan struct{})
	t.mutex.Unlock()

	t.log(fmt.Sprintf("솔로 시련 시작 (최대 %d회, 목표: (%d,%d), hwnd=0x%X)",
		t.config.MaxRuns, t.config.TargetX, t.config.TargetY, hwnd))

	go t.runLoopSolo()
}

// StartGroup 그룹 시련 시작
func (t *Trial) StartGroup(leaderHwnd, memberHwnd uint64) {
	t.mutex.Lock()
	if t.running {
		t.mutex.Unlock()
		return
	}
	t.running = true
	t.leaderHwnd = leaderHwnd
	t.memberHwnd = memberHwnd
	t.isGroup = true
	t.runCount = 0
	t.stopChan = make(chan struct{})
	t.mutex.Unlock()

	t.log(fmt.Sprintf("그룹 시련 시작 (최대 %d회, 장=0x%X, 원=0x%X)",
		t.config.MaxRuns, leaderHwnd, memberHwnd))

	go t.runLoopGroup()
}

// Stop 시련 중지
func (t *Trial) Stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.running {
		close(t.stopChan)
		t.running = false
	}
}

func (t *Trial) sleep(d time.Duration) bool {
	select {
	case <-t.stopChan:
		return false
	case <-time.After(d):
		return true
	}
}

func (t *Trial) randomDelay() time.Duration {
	return time.Duration(1500+rand.Intn(1500)) * time.Millisecond
}

func (t *Trial) pressKey(hwnd uint64, key string, label string) {
	t.wm.ActivateWindow(hwnd)
	time.Sleep(300 * time.Millisecond)
	robotgo.KeyTap(key)
	t.log(fmt.Sprintf("[키입력] %s: '%s'", label, key))
}

// ========== 솔로 루프 ==========

func (t *Trial) runLoopSolo() {
	defer func() {
		t.mutex.Lock()
		t.running = false
		t.mutex.Unlock()
		t.log("솔로 시련 종료")
	}()

	t.log("초기 안정화 3초 대기...")
	if !t.sleep(3 * time.Second) {
		return
	}

	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}
		if t.runCount >= t.config.MaxRuns {
			t.log(fmt.Sprintf("최대 횟수 %d회 도달 — 종료", t.config.MaxRuns))
			return
		}

		t.log(fmt.Sprintf("===== 솔로 시련 %d/%d회 시작 =====", t.runCount+1, t.config.MaxRuns))

		// 1. 시련장 대기
		t.log("[1] 환상의시련장 대기 중...")
		if !t.waitForTrialLobby(t.hwnd) {
			return
		}

		// 2. 입장: o → enter → enter (+ 첫 사이클만 enter 한 번 더)
		firstCycle := t.runCount == 0
		if firstCycle {
			t.log("[2] 입장 시퀀스 (첫 사이클): o → enter → enter → enter")
		} else {
			t.log("[2] 입장 시퀀스: o → enter → enter")
		}
		t.pressKey(t.hwnd, "o", "솔로")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.pressKey(t.hwnd, "enter", "솔로")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.pressKey(t.hwnd, "enter", "솔로")
		if firstCycle {
			if !t.sleep(3 * time.Second) {
				return
			}
			t.pressKey(t.hwnd, "enter", "솔로-첫사이클")
		}
		// 마무리 ESC (남은 NPC 대화창 닫기)
		if !t.sleep(800 * time.Millisecond) {
			return
		}
		t.pressKey(t.hwnd, "escape", "솔로-마무리")

		// 3. 맵 전환 대기 (7초)
		t.log("[3] 맵 전환 대기 7초...")
		if !t.sleep(7 * time.Second) {
			return
		}

		// 4+5. 전투맵에서 주기적 이동 + 시련장 복귀 감지
		t.log("[4] 전투 중 주기적 이동 + 복귀 감지 시작")
		t.battleLoopSolo()

		t.runCount++
		t.log(fmt.Sprintf("===== 솔로 시련 %d회 완료 =====", t.runCount))

		if !t.sleep(3 * time.Second) {
			return
		}
	}
}

// ========== 그룹 루프 ==========

func (t *Trial) runLoopGroup() {
	defer func() {
		t.mutex.Lock()
		t.running = false
		t.mutex.Unlock()
		t.log("그룹 시련 종료")
	}()

	t.log("초기 안정화 3초 대기...")
	if !t.sleep(3 * time.Second) {
		return
	}

	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}
		if t.runCount >= t.config.MaxRuns {
			t.log(fmt.Sprintf("최대 횟수 %d회 도달 — 종료", t.config.MaxRuns))
			return
		}

		t.log(fmt.Sprintf("===== 그룹 시련 %d/%d회 시작 =====", t.runCount+1, t.config.MaxRuns))

		// 1. 시련장 대기 (그룹장 기준)
		t.log("[1] 환상의시련장 대기 중...")
		if !t.waitForTrialLobby(t.leaderHwnd) {
			return
		}

		// 2. 그룹장 입장: o → enter → enter (+ 첫 사이클만 enter 한 번 더)
		firstCycle := t.runCount == 0
		if firstCycle {
			t.log("[2] 그룹장 입장 (첫 사이클): o → enter → enter → enter")
		} else {
			t.log("[2] 그룹장 입장: o → enter → enter")
		}
		t.pressKey(t.leaderHwnd, "o", "그룹장")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.pressKey(t.leaderHwnd, "enter", "그룹장")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.pressKey(t.leaderHwnd, "enter", "그룹장")
		if firstCycle {
			if !t.sleep(3 * time.Second) {
				return
			}
			t.pressKey(t.leaderHwnd, "enter", "그룹장-첫사이클")
		}
		// 그룹장 마무리 ESC (남은 NPC 대화창 닫기)
		if !t.sleep(800 * time.Millisecond) {
			return
		}
		t.pressKey(t.leaderHwnd, "escape", "그룹장-마무리")

		// 3. 그룹원 5초 대기 → enter (+ 첫 사이클만 enter 한 번 더)
		if firstCycle {
			t.log("[3] 그룹원 (첫 사이클) 5초 대기 → enter → 3초 → enter")
		} else {
			t.log("[3] 그룹원 5초 대기 → enter")
		}
		if !t.sleep(5 * time.Second) {
			return
		}
		t.pressKey(t.memberHwnd, "enter", "그룹원")
		if firstCycle {
			if !t.sleep(3 * time.Second) {
				return
			}
			t.pressKey(t.memberHwnd, "enter", "그룹원-첫사이클")
		}
		// 그룹원 마무리 ESC (남은 NPC 대화창 닫기)
		if !t.sleep(800 * time.Millisecond) {
			return
		}
		t.pressKey(t.memberHwnd, "escape", "그룹원-마무리")

		// 4. 맵 전환 대기 (7초)
		t.log("[4] 맵 전환 대기 7초...")
		if !t.sleep(7 * time.Second) {
			return
		}

		// 5+6. 전투맵에서 주기적 이동 (와리가리) + 시련장 복귀 감지
		t.log("[5] 전투 중 주기적 이동 + 복귀 감지 시작")
		t.battleLoopGroup()

		// 7. 능력 선택 — 그룹장
		t.log("[7] 그룹장 능력 선택: o → enter")
		if !t.sleep(2 * time.Second) {
			return
		}
		t.pressKey(t.leaderHwnd, "o", "그룹장-능력")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.pressKey(t.leaderHwnd, "enter", "그룹장-능력")

		// 8. 그룹원 능력 선택: 3초 대기 → enter
		t.log("[8] 그룹원 능력 선택: 3초 대기 → enter")
		if !t.sleep(3 * time.Second) {
			return
		}
		t.pressKey(t.memberHwnd, "enter", "그룹원-능력")

		t.runCount++
		t.log(fmt.Sprintf("===== 그룹 시련 %d회 완료 =====", t.runCount))

		if !t.sleep(3 * time.Second) {
			return
		}
	}
}

// ========== 공통 유틸 ==========

// detectMap 맵 이름 OCR 감지
func (t *Trial) detectMap(hwnd uint64) string {
	rawImg, _, err := t.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		t.log(fmt.Sprintf("[맵OCR] 캡처 실패: %v", err))
		return ""
	}

	bounds := rawImg.Bounds()
	w := bounds.Dx()

	clientOffX, clientOffY, err := t.wm.GetClientOffset(hwnd)
	if err != nil {
		clientOffX = 8
		clientOffY = 31
	}
	clientW := w - clientOffX*2

	cropW := 300
	cropH := 25
	cropX := clientOffX + clientW/2 - cropW/2
	cropY := clientOffY
	if cropX < 0 {
		cropX = 0
	}
	if cropX+cropW > w {
		cropW = w - cropX
	}
	if cropW <= 0 || cropH <= 0 {
		return ""
	}

	cropped := rawImg.SubImage(image.Rect(
		bounds.Min.X+cropX, bounds.Min.Y+cropY,
		bounds.Min.X+cropX+cropW, bounds.Min.Y+cropY+cropH,
	))

	text, err := t.om.RecognizeText(cropped)
	if err != nil || text == "" {
		return ""
	}

	result := strings.TrimSpace(text)
	t.log(fmt.Sprintf("[맵OCR] '%s'", result))
	return result
}

// isTrialLobby "환상의시련장" 매칭 (OCR 부정확 대응 강화)
// 1) 키워드: "시련", "련장", "환상의", "환상시" 중 하나만 들어가도 true
// 2) Levenshtein 60% 허용 (7글자 → 최대 4글자 차이까지 허용)
// 3) 시련장은 "전투" 단어가 절대 안 들어가므로 "전투" 포함이면 false
func (t *Trial) isTrialLobby(mapName string) bool {
	if mapName == "" {
		return false
	}
	// 전투맵 키워드가 있으면 시련장 아님
	if strings.Contains(mapName, "전투") {
		return false
	}
	// 시련장 키워드 매칭 (부분 일치)
	keywords := []string{"시련", "련장", "환상의", "환상시", "상의시", "의시련"}
	for _, kw := range keywords {
		if strings.Contains(mapName, kw) {
			return true
		}
	}
	// Levenshtein 60% 허용
	target := []rune("환상의시련장")
	mapRunes := []rune(mapName)
	dist := levenshteinRunes(mapRunes, target)
	maxDist := len(target) * 60 / 100
	if maxDist < 2 {
		maxDist = 2
	}
	return dist <= maxDist
}

// isBattleMap 전투맵 매칭 ("[환상]대야전투" 등)
func (t *Trial) isBattleMap(mapName string) bool {
	if mapName == "" {
		return false
	}
	if strings.Contains(mapName, "전투") {
		return true
	}
	if strings.Contains(mapName, "환상") && !strings.Contains(mapName, "시련") {
		return true
	}
	mapRunes := []rune(mapName)
	if strings.HasSuffix(mapName, "투") && len(mapRunes) >= 3 && len(mapRunes) <= 8 {
		return true
	}
	return false
}

// walkTo 목표 좌표로 걸어서 이동
func (t *Trial) walkTo(hwnd uint64) {
	targetX := t.config.TargetX
	targetY := t.config.TargetY
	if targetX == 0 && targetY == 0 {
		t.log("[걷기] 목표 좌표 미설정 — 스킵")
		return
	}

	// 좌표 읽기 (최대 3회 시도)
	var coords GameCoords
	var readErr error
	for retry := 0; retry < 3; retry++ {
		t.wm.ActivateWindow(hwnd)
		time.Sleep(500 * time.Millisecond)
		coords, readErr = t.om.ReadCoordinates(hwnd)
		if readErr == nil && (coords.X > 0 || coords.Y > 0) {
			break
		}
		t.log(fmt.Sprintf("[걷기] 좌표 읽기 시도 %d/3 실패: %v (좌표=%d,%d)", retry+1, readErr, coords.X, coords.Y))
		if !t.sleep(2 * time.Second) {
			return
		}
	}
	if readErr != nil || (coords.X == 0 && coords.Y == 0) {
		t.log(fmt.Sprintf("[걷기] 좌표 읽기 최종 실패 — 이동 포기"))
		return
	}

	diffX := targetX - coords.X
	diffY := targetY - coords.Y
	absDX := diffX
	absDY := diffY
	if absDX < 0 {
		absDX = -absDX
	}
	if absDY < 0 {
		absDY = -absDY
	}

	if absDX <= 1 && absDY <= 1 {
		// 목표 근처 — 로그 생략 (2초마다 호출되므로)
		return
	}

	t.log(fmt.Sprintf("[걷기] (%d,%d) → (%d,%d) diff=(%+d,%+d)", coords.X, coords.Y, targetX, targetY, diffX, diffY))

	// X 이동
	if diffX != 0 {
		dir := "right"
		count := diffX
		if diffX < 0 {
			dir = "left"
			count = -diffX
		}
		for i := 0; i < count; i++ {
			if t.isStopped() {
				return
			}
			robotgo.KeyTap(dir)
			time.Sleep(time.Duration(300+rand.Intn(200)) * time.Millisecond)
		}
		t.log(fmt.Sprintf("[걷기] %s %d칸", dir, count))
	}

	// Y 이동
	if diffY != 0 {
		dir := "down"
		count := diffY
		if diffY < 0 {
			dir = "up"
			count = -diffY
		}
		for i := 0; i < count; i++ {
			if t.isStopped() {
				return
			}
			robotgo.KeyTap(dir)
			time.Sleep(time.Duration(300+rand.Intn(200)) * time.Millisecond)
		}
		t.log(fmt.Sprintf("[걷기] %s %d칸", dir, count))
	}

	t.log("[걷기] 이동 완료")
}

// battleLoopSolo 솔로 전투 루프: 2초마다 맵 확인 + 좌표 이동 + 시련장 복귀 감지
func (t *Trial) battleLoopSolo() {
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}

		t.wm.ActivateWindow(t.hwnd)
		time.Sleep(300 * time.Millisecond)

		mapName := t.detectMap(t.hwnd)
		if t.isTrialLobby(mapName) {
			t.log("[전투] 환상의시련장 복귀 감지!")
			return
		}

		// 전투맵일 때만 이동 (OCR 실패=빈 문자열, 시련장=절대 이동 금지)
		if t.isBattleMap(mapName) {
			t.walkTo(t.hwnd)
		} else {
			t.log(fmt.Sprintf("[전투] 맵='%s' — 전투맵 미확인, 이동 스킵", mapName))
		}

		if !t.sleep(2 * time.Second) {
			return
		}
	}
}

// battleLoopGroup 그룹 전투 루프: 2초마다 그룹장/그룹원 와리가리 이동 + 시련장 복귀 감지
// 각 캐릭터의 맵을 개별 OCR하여 시련장이면 절대 이동하지 않음 (이동은 전투맵에서만)
func (t *Trial) battleLoopGroup() {
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}

		// === 그룹장 맵 확인 ===
		t.wm.ActivateWindow(t.leaderHwnd)
		time.Sleep(300 * time.Millisecond)
		leaderMap := t.detectMap(t.leaderHwnd)
		if t.isTrialLobby(leaderMap) {
			t.log("[전투] 그룹장 환상의시련장 복귀 감지!")
			return
		}

		// 그룹장: 전투맵일 때만 이동
		if t.isBattleMap(leaderMap) {
			t.walkTo(t.leaderHwnd)
		} else {
			t.log(fmt.Sprintf("[전투] 그룹장 맵='%s' — 전투맵 미확인, 이동 스킵", leaderMap))
		}

		// === 그룹원 맵 확인 (별도 OCR) ===
		t.wm.ActivateWindow(t.memberHwnd)
		time.Sleep(300 * time.Millisecond)
		memberMap := t.detectMap(t.memberHwnd)

		// 그룹원: 시련장이면 절대 이동 금지, 전투맵일 때만 이동
		if t.isTrialLobby(memberMap) {
			t.log(fmt.Sprintf("[전투] 그룹원 시련장(%s) — 이동 안 함", memberMap))
		} else if t.isBattleMap(memberMap) {
			t.walkTo(t.memberHwnd)
		} else {
			t.log(fmt.Sprintf("[전투] 그룹원 맵='%s' — 전투맵 미확인, 이동 스킵", memberMap))
		}

		if !t.sleep(2 * time.Second) {
			return
		}
	}
}

// waitForTrialLobby 시련장에 있을 때까지 대기 (OCR 인식 실패에도 견고하도록)
//  - 30회 시도(약 90초) 이상 인식 실패 시 경고 로그
//  - 빈 문자열(OCR 실패)이 연속 5회 이상이면 캐릭터 위치가 시련장이라고 가정하고 진입
//    (사용자가 의도적으로 시련장에 두고 시작했을 가능성 + OCR이 영역을 못 잡는 경우 대응)
func (t *Trial) waitForTrialLobby(hwnd uint64) bool {
	attempts := 0
	emptyStreak := 0
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return false
		}
		t.wm.ActivateWindow(hwnd)
		time.Sleep(500 * time.Millisecond)
		mapName := t.detectMap(hwnd)
		attempts++

		if t.isTrialLobby(mapName) {
			t.log(fmt.Sprintf("[대기] 환상의시련장 확인! (시도 %d회)", attempts))
			return true
		}

		if mapName == "" {
			emptyStreak++
			t.log(fmt.Sprintf("[대기] OCR 인식 실패 (%d회 연속)", emptyStreak))
			// OCR이 5회 연속 빈 결과면 사용자가 시련장에 두고 시작했다고 가정하고 진입
			if emptyStreak >= 5 {
				t.log("[대기] OCR 5회 연속 실패 — 시련장으로 가정하고 진입")
				return true
			}
		} else {
			emptyStreak = 0
			t.log(fmt.Sprintf("[대기] 현재 맵='%s' — 시련장 아님 (시도 %d회)", mapName, attempts))
		}

		// 30회(약 90초)마다 경고
		if attempts%30 == 0 {
			t.log(fmt.Sprintf("⚠ [대기] %d회 시도 동안 시련장 미인식 — 캐릭터가 환상의시련장에 있는지 확인하세요", attempts))
		}

		if !t.sleep(3 * time.Second) {
			return false
		}
	}
}

// waitForLobby 전투 끝나고 시련장 복귀 대기
func (t *Trial) waitForLobby(hwnd uint64) {
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}
		t.wm.ActivateWindow(hwnd)
		time.Sleep(500 * time.Millisecond)
		mapName := t.detectMap(hwnd)
		if t.isTrialLobby(mapName) {
			t.log("[대기] 환상의시련장 복귀 감지!")
			return
		}
		t.log(fmt.Sprintf("[대기] 아직 전투 중... 맵='%s' (2초 후 재확인)", mapName))
		if !t.sleep(2 * time.Second) {
			return
		}
	}
}
