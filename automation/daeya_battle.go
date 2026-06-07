package automation

import (
	"fmt"
	"image"
	"log"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
)

// DaeyaBattleConfig 대야전투 자동화 설정
type DaeyaBattleConfig struct {
	SkillKeys []string // 스킬 키 목록 (랜덤 순서로 사용)
	TargetX   int      // 목표 좌표 X
	TargetY   int      // 목표 좌표 Y
	Tolerance int      // 좌표 허용 오차
}

// DefaultDaeyaBattleConfig 기본 대야전투 설정
func DefaultDaeyaBattleConfig() DaeyaBattleConfig {
	return DaeyaBattleConfig{
		SkillKeys: []string{"d", "x", "5"},
		TargetX:   29,
		TargetY:   32,
		Tolerance: 1,
	}
}

// DaeyaBattle 대야전투 자동화 관리자
type DaeyaBattle struct {
	om            *OCRManager
	km            *KeyboardManager
	wm            *WindowManager
	config        DaeyaBattleConfig
	hwnd          uint64
	stopChan      chan struct{}
	running       bool
	lastCtrlDTime time.Time
	mutex         sync.Mutex
	logFunc       func(string)
}

// NewDaeyaBattle 새로운 대야전투 관리자 생성
func NewDaeyaBattle(om *OCRManager, km *KeyboardManager, wm *WindowManager) *DaeyaBattle {
	return &DaeyaBattle{
		om:     om,
		km:     km,
		wm:     wm,
		config: DefaultDaeyaBattleConfig(),
	}
}

// SetConfig 설정 업데이트
func (db *DaeyaBattle) SetConfig(cfg DaeyaBattleConfig) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.config = cfg
}

// SetLogFunc 로그 콜백 설정
func (db *DaeyaBattle) SetLogFunc(f func(string)) {
	db.logFunc = f
}

func (db *DaeyaBattle) log(msg string) {
	log.Printf("[대야전투] %s", msg)
	if db.logFunc != nil {
		db.logFunc(msg)
	}
}

// IsRunning 실행 중 여부
func (db *DaeyaBattle) IsRunning() bool {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	return db.running
}

func (db *DaeyaBattle) isStopped() bool {
	select {
	case <-db.stopChan:
		return true
	default:
		return false
	}
}

// Start 대야전투 자동화 시작
func (db *DaeyaBattle) Start(hwnd uint64) {
	db.mutex.Lock()
	if db.running {
		db.log("이미 실행 중 — 스킵")
		db.mutex.Unlock()
		return
	}
	db.running = true
	db.hwnd = hwnd
	db.stopChan = make(chan struct{})
	db.mutex.Unlock()

	db.log(fmt.Sprintf("대야전투 자동화 시작 (스킬: %v, 목표: (%d,%d), 오차: ±%d)",
		db.config.SkillKeys, db.config.TargetX, db.config.TargetY, db.config.Tolerance))

	go db.runLoop()
}

// Stop 대야전투 자동화 중지
func (db *DaeyaBattle) Stop() {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	if db.running {
		close(db.stopChan)
		db.running = false
	}
}

// runLoop 메인 루프: OCR로 맵 감지 → 분기 처리
func (db *DaeyaBattle) runLoop() {
	// 초기 안정화 대기
	db.log("초기 안정화 대기 3초...")
	select {
	case <-db.stopChan:
		return
	case <-time.After(3 * time.Second):
	}

	for {
		if db.isStopped() || !db.km.IsRunning() {
			db.log("대야전투 자동화 종료")
			db.mutex.Lock()
			db.running = false
			db.mutex.Unlock()
			return
		}

		// 매크로 일시정지 중이면 대기
		if db.km.IsPaused() {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		db.processOnce()

		// 루프 간격
		select {
		case <-db.stopChan:
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// processOnce 한 사이클 처리: 맵 감지 → 분기
func (db *DaeyaBattle) processOnce() {
	hwnd := db.hwnd

	// 게임 화면 캡처
	rawImg, _, err := db.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		db.log(fmt.Sprintf("화면 캡처 실패: %v", err))
		return
	}

	// 맵 이름 OCR
	mapName := db.detectMapName(hwnd, rawImg)
	if mapName == "" {
		db.log("맵 이름 인식 실패 — 기본 스킬 사용")
		db.useRandomSkills()
		return
	}

	db.log(fmt.Sprintf("[맵감지] OCR='%s'", mapName))

	// 맵별 분기
	entranceMap := "대야산기슭"
	battleMap := "대야전투"

	entranceRunes := []rune(strings.ReplaceAll(entranceMap, " ", ""))
	battleRunes := []rune(strings.ReplaceAll(battleMap, " ", ""))
	mapRunes := []rune(mapName)

	entranceDist := levenshteinRunes(mapRunes, entranceRunes)
	battleDist := levenshteinRunes(mapRunes, battleRunes)

	entranceMaxDist := len(entranceRunes) * 30 / 100
	if entranceMaxDist < 1 {
		entranceMaxDist = 1
	}
	battleMaxDist := len(battleRunes) * 30 / 100
	if battleMaxDist < 1 {
		battleMaxDist = 1
	}

	// 허용 오차를 50%로 상향 (짧은 맵 이름 OCR 부정확 보완)
	entranceMaxDist = len(entranceRunes) * 50 / 100
	if entranceMaxDist < 1 {
		entranceMaxDist = 1
	}
	battleMaxDist = len(battleRunes) * 50 / 100
	if battleMaxDist < 1 {
		battleMaxDist = 1
	}

	entranceMatch := entranceDist <= entranceMaxDist
	battleMatch := battleDist <= battleMaxDist

	// 키워드 보조 매칭: "기슭" 또는 "산기" 포함 → 입구, "전투" 또는 "투" 포함 → 전투맵
	if !entranceMatch && (strings.Contains(mapName, "기슭") || strings.Contains(mapName, "산기") || strings.Contains(mapName, "기슭")) {
		entranceMatch = true
	}
	if !battleMatch {
		// "전투" 포함, 또는 "투"로 끝나고 3~5글자 (OCR이 "이현투","대현투" 등으로 인식)
		if strings.Contains(mapName, "전투") ||
			(strings.HasSuffix(mapName, "투") && len(mapRunes) >= 3 && len(mapRunes) <= 5 && !strings.Contains(mapName, "기슭")) {
			battleMatch = true
		}
	}

	db.log(fmt.Sprintf("[매칭] 입구='%s' dist=%d (max=%d, match=%v) | 전투='%s' dist=%d (max=%d, match=%v)",
		entranceMap, entranceDist, entranceMaxDist, entranceMatch,
		battleMap, battleDist, battleMaxDist, battleMatch))

	// 둘 다 매칭되면 입구맵 우선 (입구에서 절대 이동하면 안 됨)
	if entranceMatch && battleMatch {
		battleMatch = false
		db.log("[매칭] 둘 다 매칭 → 입구맵 우선 (이동 방지)")
	}

	// "기슭" 또는 "산기" 포함이면 절대 전투맵 아님 (이동 방지)
	if strings.Contains(mapName, "기슭") || strings.Contains(mapName, "산기") {
		entranceMatch = true
		battleMatch = false
	}

	if entranceMatch {
		db.log("[입구맵] 대야산기슭 감지 — 사냥터 입장 시퀀스 실행")
		db.enterBattle()
	} else if battleMatch {
		db.log("[전투맵] 대야전투 감지 — 스킬 사용")
		db.useRandomSkills()
	} else {
		db.log(fmt.Sprintf("[알수없는맵] '%s' — 스킬만 사용", mapName))
		db.useRandomSkills()
	}
}

// detectMapName 게임 화면 상단 중앙에서 맵 이름을 OCR로 읽기
func (db *DaeyaBattle) detectMapName(hwnd uint64, img *image.RGBA) string {
	bounds := img.Bounds()
	w := bounds.Dx()

	clientOffX, clientOffY, err := db.wm.GetClientOffset(hwnd)
	if err != nil {
		clientOffX = 8
		clientOffY = 31
	}
	clientW := w - clientOffX*2

	cropW := 350
	cropH := 28
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

	cropped := img.SubImage(image.Rect(
		bounds.Min.X+cropX, bounds.Min.Y+cropY,
		bounds.Min.X+cropX+cropW, bounds.Min.Y+cropY+cropH,
	))

	text, err := db.om.RecognizeText(cropped)
	if err != nil {
		db.log(fmt.Sprintf("[맵OCR] 인식 오류: %v", err))
		return ""
	}
	if text == "" {
		db.log("[맵OCR] 인식 결과 없음 (빈 문자열)")
		return ""
	}
	db.log(fmt.Sprintf("[맵OCR] 원본='%s' (길이=%d)", text, len([]rune(text))))
	return strings.TrimSpace(text)
}

// enterBattle 입구맵에서 사냥터 입장: o → enter → enter → esc (딜레이 2~4초 랜덤)
func (db *DaeyaBattle) enterBattle() {
	keys := []string{"o", "enter", "enter", "esc"}

	for _, key := range keys {
		if db.isStopped() || !db.km.IsRunning() {
			return
		}
		robotgo.KeyTap(key)
		// 1~3초 랜덤 딜레이
		delay := 1*time.Second + time.Duration(rand.Intn(2001))*time.Millisecond
		db.log(fmt.Sprintf("[입장] '%s' 키 입력 (대기 %.1f초)", key, delay.Seconds()))
		time.Sleep(delay)
	}
}

// useRandomSkills 스킬 목록을 랜덤 순서로 사용
func (db *DaeyaBattle) useRandomSkills() {
	db.mutex.Lock()
	skills := make([]string, len(db.config.SkillKeys))
	copy(skills, db.config.SkillKeys)
	db.mutex.Unlock()

	if len(skills) == 0 {
		return
	}

	// 랜덤 셔플
	rand.Shuffle(len(skills), func(i, j int) {
		skills[i], skills[j] = skills[j], skills[i]
	})

	for _, key := range skills {
		if db.isStopped() || !db.km.IsRunning() {
			return
		}
		if db.km.IsPaused() {
			return
		}
		robotgo.KeyTap(key)
		db.log(fmt.Sprintf("[스킬] '%s' 사용", key))
		time.Sleep(300 * time.Millisecond)
	}
}

// walkToTarget 현재 좌표를 OCR로 읽고 화살표 키로 걸어서 목표 좌표로 이동
func (db *DaeyaBattle) walkToTarget(hwnd uint64) {
	db.mutex.Lock()
	targetX := db.config.TargetX
	targetY := db.config.TargetY
	tolerance := db.config.Tolerance
	db.mutex.Unlock()

	if targetX == 0 && targetY == 0 {
		return
	}

	// 현재 좌표 읽기
	coords, err := db.om.ReadCoordinates(hwnd)
	if err != nil {
		db.log(fmt.Sprintf("[걷기] 좌표 읽기 실패: %v", err))
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

	if absDX <= tolerance && absDY <= tolerance {
		db.log(fmt.Sprintf("[걷기] 목표 범위 내: 현재=(%d,%d) 목표=(%d,%d)", coords.X, coords.Y, targetX, targetY))
		return
	}

	db.log(fmt.Sprintf("[걷기] 현재=(%d,%d) → 목표=(%d,%d) diff=(%+d,%+d)", coords.X, coords.Y, targetX, targetY, diffX, diffY))

	// 화살표 키로 걸어서 이동 (X: left/right, Y: up/down)
	if diffX != 0 {
		dir := "right"
		count := diffX
		if diffX < 0 {
			dir = "left"
			count = -diffX
		}
		for i := 0; i < count; i++ {
			if db.isStopped() || !db.km.IsRunning() {
				return
			}
			robotgo.KeyTap(dir)
			delay := 300 + rand.Intn(200)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	if diffY != 0 {
		dir := "down"
		count := diffY
		if diffY < 0 {
			dir = "up"
			count = -diffY
		}
		for i := 0; i < count; i++ {
			if db.isStopped() || !db.km.IsRunning() {
				return
			}
			robotgo.KeyTap(dir)
			delay := 300 + rand.Intn(200)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	db.log(fmt.Sprintf("[걷기] 이동 완료 (%s %d칸, %s %d칸)",
		func() string { if diffX >= 0 { return "→" }; return "←" }(), absDX,
		func() string { if diffY >= 0 { return "↓" }; return "↑" }(), absDY))
}

// checkAndMoveToTarget 현재 좌표를 확인하고 목표 좌표로 이동
func (db *DaeyaBattle) checkAndMoveToTarget(hwnd uint64) {
	db.mutex.Lock()
	targetX := db.config.TargetX
	targetY := db.config.TargetY
	tolerance := db.config.Tolerance
	db.mutex.Unlock()

	if targetX == 0 && targetY == 0 {
		return // 좌표 미설정
	}

	// 현재 좌표 읽기
	coords, err := db.om.ReadCoordinates(hwnd)
	if err != nil {
		db.log(fmt.Sprintf("[좌표] 좌표 읽기 실패: %v", err))
		return
	}

	diffX := targetX - coords.X
	diffY := targetY - coords.Y

	// 허용 오차 내면 이동 불필요
	absDX := diffX
	absDY := diffY
	if absDX < 0 {
		absDX = -absDX
	}
	if absDY < 0 {
		absDY = -absDY
	}
	if absDX <= tolerance && absDY <= tolerance {
		db.log(fmt.Sprintf("[좌표] 목표 범위 내: 현재=(%d,%d) 목표=(%d,%d)", coords.X, coords.Y, targetX, targetY))
		return
	}

	db.log(fmt.Sprintf("[좌표] 이동 필요: 현재=(%d,%d) → 목표=(%d,%d)", coords.X, coords.Y, targetX, targetY))
	db.moveToTarget(hwnd, targetX, targetY)
}

// moveToTarget Ctrl+D 기반 좌표 이동 (item_scanner.go의 moveToOrigin 로직 재활용)
func (db *DaeyaBattle) moveToTarget(hwnd uint64, targetX, targetY int) {
	db.log(fmt.Sprintf("[이동] 시작: 목표=(%d,%d)", targetX, targetY))

	db.waitCtrlDCooldown()

	// Ctrl+D 좌표 창 열기 + OCR (최대 3회 재시도)
	var currentCoords GameCoords
	coordsOK := false
	for retry := 0; retry < 3; retry++ {
		db.pressCtrlD()
		time.Sleep(800 * time.Millisecond)

		rawImg, _, captErr := db.wm.CaptureWindowRaw(hwnd)
		if captErr != nil {
			db.log(fmt.Sprintf("[이동] 캡처 실패: %v", captErr))
			continue
		}
		coords, debugTexts, err := db.om.ReadCoordinatesFromImage(rawImg)
		for _, dt := range debugTexts {
			db.log(fmt.Sprintf("[이동][OCR] %s", dt))
		}
		if err == nil {
			currentCoords = coords
			coordsOK = true
			break
		}
		db.log(fmt.Sprintf("[이동] OCR 실패 (시도 %d/3) — Ctrl+D 재시도", retry+1))
	}
	if !coordsOK {
		db.log("[이동] 좌표 인식 3회 실패 — 이동 포기")
		robotgo.KeyTap("escape")
		time.Sleep(300 * time.Millisecond)
		return
	}

	db.log(fmt.Sprintf("[이동] 현재=(%d,%d) → 목표=(%d,%d)",
		currentCoords.X, currentCoords.Y, targetX, targetY))

	diffX := targetX - currentCoords.X
	diffY := targetY - currentCoords.Y

	if diffX == 0 && diffY == 0 {
		db.log("[이동] 이미 목표 좌표")
		return
	}

	// 최대 3회 시도
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if db.isStopped() || !db.km.IsRunning() {
			return
		}

		// 맨해튼 거리 7칸 이내로 클램프
		moveX, moveY := db.clampMovement(diffX, diffY)

		db.log(fmt.Sprintf("[이동] 시도 %d/%d: diff=(%+d,%+d) → 이동=(%+d,%+d)",
			attempt, maxRetries, diffX, diffY, moveX, moveY))

		db.pressCtrlD()
		time.Sleep(800 * time.Millisecond)
		db.moveByArrowKeys(moveX, moveY)
		time.Sleep(2 * time.Second)

		// Enter 재시도 (몬스터가 있으면 Enter가 무시됨)
		for retry := 0; retry < 5; retry++ {
			quickCoords, qErr := db.om.ReadCoordinates(hwnd)
			if qErr != nil {
				break
			}
			newDX := targetX - quickCoords.X
			newDY := targetY - quickCoords.Y
			if newDX != diffX || newDY != diffY {
				break
			}
			db.log(fmt.Sprintf("[이동] 이동 안됨 — Enter 재시도 %d/5", retry+1))
			robotgo.KeyTap("enter")
			time.Sleep(1500 * time.Millisecond)
		}

		// 쿨타임 대기
		db.waitCtrlDCooldown()

		// OCR로 현재 위치 확인
		newCoords, err := db.om.ReadCoordinates(hwnd)
		if err != nil {
			db.log(fmt.Sprintf("[이동] OCR 실패: %v — 이동 중단", err))
			break
		}

		db.log(fmt.Sprintf("[이동] 이동 후=(%d,%d) 목표=(%d,%d)",
			newCoords.X, newCoords.Y, targetX, targetY))

		absDX := targetX - newCoords.X
		absDY := targetY - newCoords.Y
		if absDX < 0 {
			absDX = -absDX
		}
		if absDY < 0 {
			absDY = -absDY
		}
		tolerance := db.config.Tolerance
		if absDX <= tolerance && absDY <= tolerance {
			db.log(fmt.Sprintf("[이동] 성공! (%d,%d)", newCoords.X, newCoords.Y))
			robotgo.KeyTap("escape")
			time.Sleep(300 * time.Millisecond)
			return
		}

		diffX = targetX - newCoords.X
		diffY = targetY - newCoords.Y
	}

	robotgo.KeyTap("escape")
	time.Sleep(300 * time.Millisecond)
}

// clampMovement 맨해튼 거리 7칸 이내로 클램프
func (db *DaeyaBattle) clampMovement(diffX, diffY int) (int, int) {
	absDX := diffX
	absDY := diffY
	if absDX < 0 {
		absDX = -absDX
	}
	if absDY < 0 {
		absDY = -absDY
	}

	moveX := diffX
	moveY := diffY
	manhattan := absDX + absDY
	if manhattan <= 7 {
		return moveX, moveY
	}

	ratio := 7.0 / float64(manhattan)
	moveX = int(math.Round(float64(diffX) * ratio))
	moveY = int(math.Round(float64(diffY) * ratio))
	if moveX == 0 && diffX != 0 {
		if diffX > 0 {
			moveX = 1
		} else {
			moveX = -1
		}
	}
	if moveY == 0 && diffY != 0 {
		if diffY > 0 {
			moveY = 1
		} else {
			moveY = -1
		}
	}

	am := moveX
	if am < 0 {
		am = -am
	}
	bm := moveY
	if bm < 0 {
		bm = -bm
	}
	for am+bm > 7 {
		if bm > am {
			if moveY > 0 {
				moveY--
			} else {
				moveY++
			}
		} else {
			if moveX > 0 {
				moveX--
			} else {
				moveX++
			}
		}
		am = moveX
		if am < 0 {
			am = -am
		}
		bm = moveY
		if bm < 0 {
			bm = -bm
		}
	}

	return moveX, moveY
}

// pressCtrlD Ctrl+D 전송
func (db *DaeyaBattle) pressCtrlD() {
	db.log("[키입력] Ctrl+D 전송")
	robotgo.KeyToggle("ctrl", "down")
	time.Sleep(100 * time.Millisecond)
	robotgo.KeyTap("d")
	time.Sleep(100 * time.Millisecond)
	robotgo.KeyToggle("ctrl", "up")
	time.Sleep(100 * time.Millisecond)
	db.lastCtrlDTime = time.Now()
}

// waitCtrlDCooldown Ctrl+D 쿨타임 잔여 시간 대기 (8초)
func (db *DaeyaBattle) waitCtrlDCooldown() {
	if db.lastCtrlDTime.IsZero() {
		return
	}
	elapsed := time.Since(db.lastCtrlDTime)
	remaining := 8*time.Second - elapsed
	if remaining > 0 {
		db.log(fmt.Sprintf("[쿨타임] Ctrl+D 잔여 %.1f초 대기...", remaining.Seconds()))
		time.Sleep(remaining)
	}
}

// moveByArrowKeys 화살표키로 이동 후 엔터
func (db *DaeyaBattle) moveByArrowKeys(diffX, diffY int) {
	db.log(fmt.Sprintf("[화살표이동] diffX=%d, diffY=%d", diffX, diffY))

	if diffX != 0 {
		dir := "right"
		count := diffX
		if diffX < 0 {
			dir = "left"
			count = -diffX
		}
		for i := 0; i < count; i++ {
			robotgo.KeyTap(dir)
			time.Sleep(50 * time.Millisecond)
		}
	}

	if diffY != 0 {
		dir := "down"
		count := diffY
		if diffY < 0 {
			dir = "up"
			count = -diffY
		}
		for i := 0; i < count; i++ {
			robotgo.KeyTap(dir)
			time.Sleep(50 * time.Millisecond)
		}
	}

	time.Sleep(200 * time.Millisecond)
	robotgo.KeyTap("enter")
	db.log("[화살표이동] 엔터 입력 — 이동 확정")
}
