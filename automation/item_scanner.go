package automation

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
)

// levenshteinRunes 두 rune 슬라이스 간 편집 거리
func levenshteinRunes(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			dp[i][j] = dp[i-1][j] + 1
			if dp[i][j-1]+1 < dp[i][j] {
				dp[i][j] = dp[i][j-1] + 1
			}
			if dp[i-1][j-1]+cost < dp[i][j] {
				dp[i][j] = dp[i-1][j-1] + cost
			}
		}
	}
	return dp[la][lb]
}

// fuzzyMatchItem OCR 단어가 대상 아이템명과 유사한지 판단
// 한글만 추출 후 편집거리 ≤ 1 이면 매칭
// FuzzyMatchItemPublic 외부에서 접근 가능한 유사 매칭
func FuzzyMatchItemPublic(ocrText string, targetName string) bool {
	return fuzzyMatchItem(ocrText, targetName)
}

func fuzzyMatchItem(ocrText string, targetName string) bool {
	// 정확 매칭
	if strings.Contains(ocrText, targetName) {
		return true
	}
	// 한글만 추출하여 비교
	ocrKorean := extractKoreanFromText(ocrText)
	targetKorean := extractKoreanFromText(targetName)
	if len(targetKorean) == 0 {
		return false
	}
	// 정확 포함
	if strings.Contains(ocrKorean, targetKorean) {
		return true
	}
	// 슬라이딩 윈도우 유사 매칭
	ocrRunes := []rune(ocrKorean)
	targetRunes := []rune(targetKorean)
	targetLen := len(targetRunes)
	if len(ocrRunes) >= targetLen {
		for start := 0; start <= len(ocrRunes)-targetLen; start++ {
			sub := ocrRunes[start : start+targetLen]
			if levenshteinRunes(sub, targetRunes) <= 1 {
				return true
			}
		}
	}
	// 전체 비교 (짧은 OCR 결과)
	if len(ocrRunes) < targetLen && len(ocrRunes) >= targetLen-1 {
		if levenshteinRunes(ocrRunes, targetRunes) <= 1 {
			return true
		}
	}
	return false
}

// extractKoreanFromText 한글 완성형만 추출
func extractKoreanFromText(s string) string {
	var filtered []rune
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			filtered = append(filtered, r)
		}
	}
	return string(filtered)
}

// TargetItem 감지 대상 아이템
type TargetItem struct {
	Name  string `json:"name"`  // 아이템명 (예: "설산의보석")
	Color string `json:"color"` // 색상 힌트: "green", "yellow" (UI 표시용)
}

// ItemScannerConfig 아이템 자동 습득 설정
type ItemScannerConfig struct {
	Enabled      bool         `json:"enabled"`
	Items        []TargetItem `json:"items"`        // 감지할 아이템 리스트
	ScanInterval int          `json:"scanInterval"` // 스캔 주기 (초)
	TilePixelW   int          `json:"tilePixelW"`   // 타일 1칸 가로 픽셀
	TilePixelH   int          `json:"tilePixelH"`   // 타일 1칸 세로 픽셀
	OriginX      int          `json:"originX"`      // 복귀 좌표 X (사용자 설정)
	OriginY      int          `json:"originY"`      // 복귀 좌표 Y (사용자 설정)
	TargetMap    string       `json:"targetMap"`    // 아이템 스캔할 맵 이름 (예: "칸첸중가설산")
	WrongMap     string       `json:"wrongMap"`     // 자동이동 해야하는 맵 (예: "칸첸중가설산초입")
	SkillKeys    []string     `json:"skillKeys"`    // 스킬 키 (예: ["1","2","8"]) — 스캔마다 자동 입력
}

// DefaultItemScannerConfig 기본 설정
func DefaultItemScannerConfig() ItemScannerConfig {
	return ItemScannerConfig{
		Enabled: false,
		Items: []TargetItem{
			{Name: "설산의보석", Color: "green"},
			{Name: "찬란한설산의보석", Color: "green"},
			{Name: "찬란한아그니의적영", Color: "yellow"},
		},
		ScanInterval: 1,
		TilePixelW:   48,
		TilePixelH:   24,
		OriginX:      0,
		OriginY:      0,
		TargetMap:    "칸첸중가설산",
		WrongMap:     "칸첸중가설산초입",
	}
}

// ignoredItem 타일 범위 밖 등으로 이동 실패한 아이템 (재시도 방지)
type ignoredItem struct {
	name string
	x, y int
}

// ItemScanner 아이템 자동 감지 및 습득 관리자
type ItemScanner struct {
	om              *OCRManager
	km              *KeyboardManager
	ma              *MouseAutomation
	config          ItemScannerConfig
	hwnd            uint64
	stopChan        chan struct{}
	running         bool
	mapConfirmed    bool           // TargetMap 확인 완료 — 원점 이동 스킵 (맵 OCR은 매회 실행)
	wrongMapRetries int            // 연속 wrongMap 감지 횟수 (무한루프 방지)
	lastCtrlDTime   time.Time      // 마지막 Ctrl+D 사용 시각 (쿨타임 계산용)
	ignoredItems    []ignoredItem  // 이동 실패한 아이템 목록 (타일 밖)
	mutex           sync.Mutex
	logFunc         func(string) // 로그 콜백
}

// NewItemScanner 새로운 아이템 스캐너 생성
func NewItemScanner(om *OCRManager, km *KeyboardManager, ma *MouseAutomation) *ItemScanner {
	return &ItemScanner{
		om:     om,
		km:     km,
		ma:     ma,
		config: DefaultItemScannerConfig(),
	}
}

// SetConfig 설정 업데이트
func (is *ItemScanner) SetConfig(cfg ItemScannerConfig) {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	is.config = cfg
}

// GetConfig 현재 설정 반환
func (is *ItemScanner) GetConfig() ItemScannerConfig {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	return is.config
}

// SetLogFunc 로그 콜백 설정
func (is *ItemScanner) SetLogFunc(f func(string)) {
	is.logFunc = f
}

func (is *ItemScanner) log(msg string) {
	log.Printf("[아이템스캐너] %s", msg)
	if is.logFunc != nil {
		is.logFunc(msg)
	}
}

// isStopped 중지 여부 확인 (non-blocking)
func (is *ItemScanner) isStopped() bool {
	select {
	case <-is.stopChan:
		return true
	default:
		return false
	}
}

// IsRunning 실행 중 여부
func (is *ItemScanner) IsRunning() bool {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	return is.running
}

// Start 주기적 아이템 스캔 시작
func (is *ItemScanner) Start(hwnd uint64) {
	is.mutex.Lock()
	itemNames := make([]string, len(is.config.Items))
	for i, it := range is.config.Items {
		itemNames[i] = it.Name
	}
	is.log(fmt.Sprintf("Start() 호출됨: enabled=%v, running=%v, items=%v, hwnd=%d",
		is.config.Enabled, is.running, itemNames, hwnd))
	if is.running {
		is.log("이미 실행 중 — 스킵")
		is.mutex.Unlock()
		return
	}
	if !is.config.Enabled {
		is.log("비활성화 상태 — 스캔 시작하지 않음")
		is.mutex.Unlock()
		return
	}
	is.running = true
	is.hwnd = hwnd
	is.stopChan = make(chan struct{})
	is.mapConfirmed = false
	is.ignoredItems = nil
	is.mutex.Unlock()

	interval := is.config.ScanInterval
	if interval < 1 {
		interval = 1
	}

	is.log(fmt.Sprintf("아이템 스캔 시작 (아이템: %v, 주기: %d초, 원점: X=%d Y=%d)", itemNames, interval, is.config.OriginX, is.config.OriginY))

	go func() {
		// 첫 스캔 전 대기 — 맵 감지가 있으므로 최소 안정화 시간만
		is.log("초기 안정화 대기 3초...")
		select {
		case <-is.stopChan:
			is.log("아이템 스캔 중지 (대기 중)")
			return
		case <-time.After(3 * time.Second):
		}

		sleepDuration := time.Duration(interval) * time.Second

		for {
			select {
			case <-is.stopChan:
				is.log("아이템 스캔 중지")
				return
			default:
			}

			// 매크로가 일시정지 중이면 스캔 스킵
			if is.km.IsPaused() {
				time.Sleep(sleepDuration)
				continue
			}
			// 매크로가 중지되면 스캐너도 종료
			if !is.km.IsRunning() {
				is.log("매크로 중지됨 — 아이템 스캔 종료")
				is.mutex.Lock()
				is.running = false
				is.mutex.Unlock()
				return
			}

			// 스킬 키 자동 입력 (스캔 전)
			if len(is.config.SkillKeys) > 0 {
				for _, key := range is.config.SkillKeys {
					if key != "" {
						robotgo.KeyTap(key)
						time.Sleep(150 * time.Millisecond)
					}
				}
			}

			is.scanOnce()
			time.Sleep(sleepDuration)
		}
	}()
}

// Stop 아이템 스캔 중지
func (is *ItemScanner) Stop() {
	is.mutex.Lock()
	defer is.mutex.Unlock()
	if is.running {
		close(is.stopChan)
		is.running = false
	}
}

// detectMapName 게임 화면 상단 중앙에서 맵 이름을 OCR로 읽어 반환
func (is *ItemScanner) detectMapName(hwnd uint64, img *image.RGBA) string {
	bounds := img.Bounds()
	w := bounds.Dx()

	// GetClientOffset으로 정확한 클라이언트 영역 오프셋 계산
	clientOffX, clientOffY, err := is.om.wm.GetClientOffset(hwnd)
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

	cropped := img.SubImage(image.Rect(
		bounds.Min.X+cropX, bounds.Min.Y+cropY,
		bounds.Min.X+cropX+cropW, bounds.Min.Y+cropY+cropH,
	))

	text, err := is.om.RecognizeText(cropped)
	if err != nil || text == "" {
		return ""
	}
	return strings.TrimSpace(text)
}

// checkMapStillTarget 현재 맵이 TargetMap인지 확인 (moveToOrigin 중 맵 변경 감지용)
// false 반환 시 mapConfirmed를 리셋하므로 다음 스캔에서 맵을 재확인함
func (is *ItemScanner) checkMapStillTarget(hwnd uint64) bool {
	is.mutex.Lock()
	config := is.config
	is.mutex.Unlock()

	if config.TargetMap == "" {
		return true
	}
	rawImg, _, err := is.om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		return true // 캡처 실패 시 계속 진행
	}
	mapName := is.detectMapName(hwnd, rawImg)
	if mapName == "" {
		return true // OCR 실패 시 계속 진행
	}
	targetRunes := []rune(strings.ReplaceAll(config.TargetMap, " ", ""))
	dist := levenshteinRunes([]rune(mapName), targetRunes)
	maxDist := len(targetRunes) * 30 / 100
	if maxDist < 1 {
		maxDist = 1
	}
	if dist > maxDist {
		is.log(fmt.Sprintf("[원점이동] 맵 변경 감지! OCR='%s' (dist=%d) — 이동 중단", mapName, dist))
		is.mapConfirmed = false
		return false
	}
	return true
}

// moveToOrigin 원점 좌표로 이동 (Ctrl+D → 화살표 → Enter 반복, 최대 5회)
// 매크로는 호출자가 일시정지/재개 관리. 성공 여부 반환.
// skipInitialCooldown=true면 첫 Ctrl+D 쿨타임을 건너뜀 (맵 입장 직후 등)
func (is *ItemScanner) moveToOrigin(hwnd uint64, originX, originY int, skipInitialCooldown ...bool) bool {
	is.log(fmt.Sprintf("[원점이동] 시작: 목표=(%d,%d)", originX, originY))

	// Ctrl+D 잔여 쿨타임 대기 (lastCtrlDTime 기준)
	skip := len(skipInitialCooldown) > 0 && skipInitialCooldown[0]
	if !skip {
		is.waitCtrlDCooldown()
	}

	// 맵 변경 감지 (3분 타이머로 자동 퇴장 등)
	if !is.checkMapStillTarget(hwnd) {
		robotgo.KeyTap("escape")
		time.Sleep(300 * time.Millisecond)
		return false
	}

	// Ctrl+D 좌표 창 열기 + OCR (최대 3회 재시도)
	// 맵 입장 직후 등 Ctrl+D 토글 상태가 불확실하므로 재시도 필요
	var currentCoords GameCoords
	coordsOK := false
	for coordRetry := 0; coordRetry < 3; coordRetry++ {
		is.pressCtrlD()
		time.Sleep(800 * time.Millisecond)

		rawImg, _, captErr := is.om.wm.CaptureWindowRaw(hwnd)
		if captErr != nil {
			is.log(fmt.Sprintf("[원점이동] 캡처 실패: %v", captErr))
			continue
		}
		coords, debugTexts, err := is.om.ReadCoordinatesFromImage(rawImg)
		for _, dt := range debugTexts {
			is.log(fmt.Sprintf("[원점이동][OCR] %s", dt))
		}
		if err == nil {
			currentCoords = coords
			coordsOK = true
			break
		}
		is.log(fmt.Sprintf("[원점이동] OCR 실패 (시도 %d/3) — Ctrl+D 재시도", coordRetry+1))
	}
	if !coordsOK {
		is.log("[원점이동] 좌표 인식 3회 실패 — 이동 포기")
		robotgo.KeyTap("escape")
		time.Sleep(300 * time.Millisecond)
		return false
	}

	is.log(fmt.Sprintf("[원점이동] 현재=(%d,%d) → 목표=(%d,%d)",
		currentCoords.X, currentCoords.Y, originX, originY))

	diffX := originX - currentCoords.X
	diffY := originY - currentCoords.Y

	if diffX == 0 && diffY == 0 {
		is.log("[원점이동] 이미 원점 — 이동 불필요")
		return true
	}

	// 반복 이동 (최대 5회)
	maxRetries := 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if is.isStopped() {
			is.log("[원점이동] 중지 요청 — 이동 취소")
			return false
		}

		absDX := diffX
		absDY := diffY
		if absDX < 0 {
			absDX = -absDX
		}
		if absDY < 0 {
			absDY = -absDY
		}

		// 맨해튼 거리 7칸 이내로 클램프
		moveX := diffX
		moveY := diffY
		manhattan := absDX + absDY
		if manhattan > 7 {
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
		}

		is.log(fmt.Sprintf("[원점이동] 시도 %d/%d: diff=(%+d,%+d) → 이동=(%+d,%+d)",
			attempt, maxRetries, diffX, diffY, moveX, moveY))

		is.pressCtrlD()
		time.Sleep(800 * time.Millisecond)
		is.moveByArrowKeys(moveX, moveY)
		time.Sleep(2 * time.Second)

		// 이동 실패 대비: 몬스터가 있으면 Enter가 무시됨
		// 이동 실패 시 Ctrl+D 쿨타임이 안 생기므로 Enter만 재시도
		for retry := 0; retry < 5; retry++ {
			quickCoords, qErr := is.om.ReadCoordinates(hwnd)
			if qErr != nil {
				break
			}
			newDX := originX - quickCoords.X
			newDY := originY - quickCoords.Y
			// 좌표가 바뀌었으면 이동 성공 (이전 diff와 다름)
			if newDX != diffX || newDY != diffY {
				break
			}
			is.log(fmt.Sprintf("[원점이동] 이동 안됨 — Enter 재시도 %d/5", retry+1))
			robotgo.KeyTap("enter")
			time.Sleep(1500 * time.Millisecond)
		}

		// Ctrl+D 잔여 쿨타임 대기
		is.waitCtrlDCooldown()

		// 맵 변경 감지 (쿨타임 중 맵이 바뀔 수 있음)
		if !is.checkMapStillTarget(hwnd) {
			robotgo.KeyTap("escape")
			time.Sleep(300 * time.Millisecond)
			return false
		}

		// OCR로 현재 위치 확인
		newCoords, err := is.om.ReadCoordinates(hwnd)
		if err != nil {
			is.log(fmt.Sprintf("[원점이동] OCR 실패: %v — 이동 중단", err))
			break
		}

		is.log(fmt.Sprintf("[원점이동] 이동 후=(%d,%d) 목표=(%d,%d)",
			newCoords.X, newCoords.Y, originX, originY))

		if newCoords.X == originX && newCoords.Y == originY {
			is.log(fmt.Sprintf("[원점이동] 성공! (%d,%d)", originX, originY))
			is.ignoredItems = nil // 이동 완료 → 무시 목록 초기화
			robotgo.KeyTap("escape")
			time.Sleep(300 * time.Millisecond)
			return true
		}

		diffX = originX - newCoords.X
		diffY = originY - newCoords.Y

		if diffX == 0 && diffY == 0 {
			is.log("[원점이동] 도착 확인")
			is.ignoredItems = nil // 이동 완료 → 무시 목록 초기화
			robotgo.KeyTap("escape")
			time.Sleep(300 * time.Millisecond)
			return true
		}

		if attempt == maxRetries {
			is.log(fmt.Sprintf("[원점이동] %d회 시도 후 미도착: 현재=(%d,%d)",
				maxRetries, newCoords.X, newCoords.Y))
		}
	}

	robotgo.KeyTap("escape")
	time.Sleep(300 * time.Millisecond)
	return false
}

// handleWrongMap 잘못된 맵에서 o → Enter로 이동 후, TargetMap이면 원점까지 이동
func (is *ItemScanner) handleWrongMap() {
	is.wrongMapRetries++
	if is.wrongMapRetries > 3 {
		is.log(fmt.Sprintf("[맵이동] %d회 연속 실패 — 맵 이동 포기, 매크로 재개", is.wrongMapRetries-1))
		is.wrongMapRetries = 0
		is.km.Resume()
		return
	}
	is.log(fmt.Sprintf("[맵이동] 잘못된 맵 감지 — o → Enter로 이동 시작 (시도 %d/3)", is.wrongMapRetries))
	is.ignoredItems = nil // 맵 이동 → 무시 목록 초기화
	is.km.Pause()
	time.Sleep(500 * time.Millisecond)
	robotgo.KeyTap("o")
	time.Sleep(500 * time.Millisecond)
	robotgo.KeyTap("enter")
	is.log("[맵이동] o → Enter 입력 완료 — 5초 대기 (맵 로딩)")
	time.Sleep(5 * time.Second)

	// 맵 전환 후 재확인 — Resume 전에 원점 이동까지 처리
	is.mutex.Lock()
	hwnd := is.hwnd
	config := is.config
	is.mutex.Unlock()

	img, _, err := is.om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		is.log(fmt.Sprintf("[맵이동] 캡처 실패: %v — 매크로 재개", err))
		robotgo.KeyTap("escape")
		time.Sleep(300 * time.Millisecond)
		is.km.Resume()
		return
	}

	mapName := is.detectMapName(hwnd, img)
	is.log(fmt.Sprintf("[맵이동] 맵 전환 후 재확인: OCR='%s'", mapName))

	if mapName != "" && config.TargetMap != "" {
		targetRunes := []rune(strings.ReplaceAll(config.TargetMap, " ", ""))
		dist := levenshteinRunes([]rune(mapName), targetRunes)
		maxDist := len(targetRunes) * 30 / 100
		if maxDist < 1 {
			maxDist = 1
		}
		is.log(fmt.Sprintf("[맵이동] TargetMap 매칭: dist=%d max=%d", dist, maxDist))

		if dist <= maxDist {
			is.log("[맵이동] TargetMap 확인 — 원점 이동 시작")
			// 원점 이동 (Ctrl+D 쿨타임 스킵 — 맵 입장 직후)
			if config.OriginX > 0 || config.OriginY > 0 {
				is.moveToOrigin(hwnd, config.OriginX, config.OriginY, true)
			}
			is.mapConfirmed = true
			is.log("[맵이동] 원점 이동 완료 — 매크로 재개")
			robotgo.KeyTap("escape")
			time.Sleep(300 * time.Millisecond)
			is.km.Resume()
			return
		}
	}

	// TargetMap 아님 — 그냥 재개
	is.log("[맵이동] TargetMap 미확인 — ESC → 매크로 재개")
	robotgo.KeyTap("escape")
	time.Sleep(300 * time.Millisecond)
	is.km.Resume()
}

// scanOnce 1회 스캔: 화면 캡처 → 맵+아이템 OCR 배치 1회 → 결과 처리
func (is *ItemScanner) scanOnce() {
	t0 := time.Now()

	is.mutex.Lock()
	hwnd := is.hwnd
	config := is.config
	is.mutex.Unlock()

	if len(config.Items) == 0 {
		return
	}

	// 게임 화면 캡처 (1회)
	img, _, err := is.om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		is.log(fmt.Sprintf("캡처 실패: %v", err))
		return
	}
	tCapture := time.Since(t0).Milliseconds()

	needMapCheck := config.WrongMap != "" || config.TargetMap != ""

	// === 맵 이미지 준비 (8x 스케일 1장) ===
	var mapImages []image.Image
	if needMapCheck {
		bounds := img.Bounds()
		w := bounds.Dx()
		clientOffX, clientOffY, err := is.om.wm.GetClientOffset(hwnd)
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
		if cropW > 0 && cropH > 0 {
			mapCropped := img.SubImage(image.Rect(
				bounds.Min.X+cropX, bounds.Min.Y+cropY,
				bounds.Min.X+cropX+cropW, bounds.Min.Y+cropY+cropH,
			))
			mapScaled := scaleImage(mapCropped, 8) // 8x 맵 이름 최적
			mapImages = append(mapImages, mapScaled)
		}
	}

	// === 아이템 이미지 준비 (크롭 + 4x + 3변형) ===
	// 게임 경계선 안쪽만 크롭 (좌측/우측 UI 패널 제외)
	// 경계선은 게임 윈도우의 약 18%~69% 위치 (1602px 윈도우에서 x=288~1103)
	// 여유를 두어 17%~70%(X), 13%~70%(Y)로 설정
	fullW := img.Bounds().Dx()
	fullH := img.Bounds().Dy()
	itemCropX := fullW * 17 / 100
	itemCropY := fullH * 13 / 100
	itemCropW := fullW * 53 / 100 // 17%~70% = 53%
	itemCropH := fullH * 57 / 100 // 13%~70% = 57%

	var itemImages []image.Image
	if itemCropW > 0 && itemCropH > 0 {
		itemCropped := img.SubImage(image.Rect(itemCropX, itemCropY, itemCropX+itemCropW, itemCropY+itemCropH))
		scaled4 := scaleImage(itemCropped, 4)

		// 초록+노란 통합 필터 (Hue 60°~180° 커버 — 녹색/노란색 아이템 텍스트)
		s4B := scaled4.Bounds()
		w4, h4 := s4B.Dx(), s4B.Dy()
		greenResult := image.NewRGBA(image.Rect(0, 0, w4, h4))
		for y := 0; y < h4; y++ {
			for x := 0; x < w4; x++ {
				rv, gv, bv, _ := scaled4.At(x, y).RGBA()
				r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
				isGreen := false
				if g8 > r8+15 && g8 > b8+15 && g8 > 40 {
					isGreen = true
				}
				if !isGreen {
					maxC := r8
					if g8 > maxC {
						maxC = g8
					}
					if b8 > maxC {
						maxC = b8
					}
					minC := r8
					if g8 < minC {
						minC = g8
					}
					if b8 < minC {
						minC = b8
					}
					delta := maxC - minC
					if delta > 10 && maxC > 50 {
						sat := delta * 100 / maxC
						if sat > 15 {
							var hue int
							if maxC == g8 {
								hue = 120 + 60*(b8-r8)/delta
							} else if maxC == r8 {
								hue = 60 * (g8 - b8) / delta
								if hue < 0 {
									hue += 360
								}
							} else {
								hue = 240 + 60*(r8-g8)/delta
							}
							if hue >= 60 && hue <= 180 {
								isGreen = true
							}
						}
					}
				}
				if isGreen {
					greenResult.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
				} else {
					greenResult.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
				}
			}
		}
		itemImages = append(itemImages, greenResult)
	}

	tPreproc := time.Since(t0).Milliseconds()

	// === 배치 OCR: 맵(1장) + 아이템(3장) = 상주 PowerShell ===
	batchResult, err := is.om.RecognizeBatchAll(mapImages, itemImages)
	if err != nil {
		is.log(fmt.Sprintf("배치 OCR 실패: %v", err))
		return
	}
	tOCR := time.Since(t0).Milliseconds()

	// === 맵 결과 처리 ===
	if needMapCheck && batchResult.MapText != "" {
		mapName := extractKoreanFromText(batchResult.MapText)
		mapRunes := []rune(mapName)

		wrongMatch := false
		wrongDist := -1
		if config.WrongMap != "" {
			wrongRunes := []rune(strings.ReplaceAll(config.WrongMap, " ", ""))
			wrongDist = levenshteinRunes(mapRunes, wrongRunes)
			maxDist := len(wrongRunes) * 30 / 100
			if maxDist < 1 {
				maxDist = 1
			}
			wrongMatch = wrongDist <= maxDist
		}

		targetMatch := false
		targetDist := -1
		if config.TargetMap != "" {
			targetRunes := []rune(strings.ReplaceAll(config.TargetMap, " ", ""))
			targetDist = levenshteinRunes(mapRunes, targetRunes)
			maxDist := len(targetRunes) * 30 / 100
			if maxDist < 1 {
				maxDist = 1
			}
			targetMatch = targetDist <= maxDist
		}

		// 둘 다 매칭되면 거리가 더 가까운 쪽 선택
		if wrongMatch && targetMatch {
			if targetDist >= 0 {
				wrongMatch = false
			}
		}

		is.log(fmt.Sprintf("[맵체크] mapConfirmed=%v, OCR='%s', wrong: dist=%d match=%v | target: dist=%d match=%v",
			is.mapConfirmed, mapName, wrongDist, wrongMatch, targetDist, targetMatch))

		if wrongMatch {
			is.log("잘못된 맵 감지 — o→Enter 이동")
			is.mapConfirmed = false
			is.handleWrongMap()
			return
		}

		if targetMatch {
			is.wrongMapRetries = 0 // 정상 맵 확인 — 연속 실패 카운터 리셋
			if !is.mapConfirmed {
				if config.OriginX > 0 || config.OriginY > 0 {
					is.km.Pause()
					time.Sleep(500 * time.Millisecond)
					robotgo.KeyTap("escape")
					time.Sleep(500 * time.Millisecond)
					currentCoords, err := is.om.ReadCoordinates(hwnd)
					if err == nil {
						is.log(fmt.Sprintf("[맵체크] 좌표: 현재=(%d,%d) 원점=(%d,%d)",
							currentCoords.X, currentCoords.Y, config.OriginX, config.OriginY))
					}
					if err == nil && (currentCoords.X != config.OriginX || currentCoords.Y != config.OriginY) {
						is.log(fmt.Sprintf("원점 아님 (%d,%d) → (%d,%d) 이동",
							currentCoords.X, currentCoords.Y, config.OriginX, config.OriginY))
						is.moveToOrigin(hwnd, config.OriginX, config.OriginY, true)
					}
					is.km.Resume()
				}
				is.mapConfirmed = true
				is.log("대상 맵 확인 완료")
				return
			}
		} else if config.TargetMap != "" {
			is.log(fmt.Sprintf("[맵체크] 알 수 없는 맵 '%s' — 대기", mapName))
			is.mapConfirmed = false
			return
		}
	} else if needMapCheck && batchResult.MapText == "" {
		is.log(fmt.Sprintf("[맵체크] OCR 실패 (mapConfirmed=%v)", is.mapConfirmed))
	}

	// === 아이템 결과 처리: 좌표 역산 + 중복 제거 + 매칭 ===
	scale := 4.0
	offsetX := float64(itemCropX)
	offsetY := float64(itemCropY)
	allWordsMap := make(map[string]OCRWord)
	for _, w := range batchResult.ItemWords {
		if w.Text == "" {
			continue
		}
		rw := OCRWord{
			Text:   w.Text,
			X:      w.X/scale + offsetX,
			Y:      w.Y/scale + offsetY,
			Width:  w.Width / scale,
			Height: w.Height / scale,
		}
		key := fmt.Sprintf("%s_%.0f_%.0f", rw.Text, rw.X/10, rw.Y/10)
		if _, exists := allWordsMap[key]; !exists {
			allWordsMap[key] = rw
		}
	}
	words := make([]OCRWord, 0, len(allWordsMap))
	for _, w := range allWordsMap {
		words = append(words, w)
	}

	// 아이템 매칭
	var matchX, matchY float64
	var matchedItemName string
	found := false

	for _, item := range config.Items {
		if item.Name == "" {
			continue
		}
		targetName := item.Name
		for _, w := range words {
			if fuzzyMatchItem(w.Text, targetName) {
				matchX = w.X + w.Width/2
				matchY = w.Y + w.Height/2
				matchedItemName = targetName
				found = true
				break
			}
		}
		if found {
			break
		}
		for i := 0; i < len(words); i++ {
			combined := words[i].Text
			combinedX := words[i].X
			combinedY := words[i].Y
			combinedW := words[i].Width
			combinedH := words[i].Height
			for j := i + 1; j < len(words) && j <= i+3; j++ {
				combined += words[j].Text
				endX := words[j].X + words[j].Width
				endY := words[j].Y + words[j].Height
				if endX > combinedX+combinedW {
					combinedW = endX - combinedX
				}
				if endY > combinedY+combinedH {
					combinedH = endY - combinedY
				}
				if fuzzyMatchItem(combined, targetName) {
					matchX = combinedX + combinedW/2
					matchY = combinedY + combinedH/2
					matchedItemName = targetName
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}

	tTotal := time.Since(t0).Milliseconds()

	if !found {
		names := make([]string, len(config.Items))
		for i, it := range config.Items {
			names[i] = it.Name
		}
		is.log(fmt.Sprintf("스캔: OCR %d개 | 미발견: %v [%dms: 캡처%d+전처리%d+OCR%d]",
			len(words), names, tTotal, tCapture, tPreproc-tCapture, tOCR-tPreproc))
		return
	}

	is.log(fmt.Sprintf("'%s' 발견! @ (%.0f,%.0f) [%dms]", matchedItemName, matchX, matchY, tTotal))

	// 무시 목록 체크 (타일 밖 이동 실패 아이템)
	for _, ig := range is.ignoredItems {
		if ig.name == matchedItemName &&
			math.Abs(float64(int(matchX)-ig.x)) < 30 &&
			math.Abs(float64(int(matchY)-ig.y)) < 30 {
			is.log(fmt.Sprintf("'%s' @ (%.0f,%.0f) — 타일 밖 무시 목록, 스킵", matchedItemName, matchX, matchY))
			return
		}
	}

	// 화면 중심과의 거리 확인
	windowW := float64(img.Bounds().Dx())
	windowH := float64(img.Bounds().Dy())
	centerX := windowW / 2
	centerY := windowH / 2
	diffX := matchX - centerX
	diffY := matchY - centerY

	// 화면 중심에서 너무 가까우면 (20px 이내) 같은 위치 판정
	if math.Abs(diffX) < 20 && math.Abs(diffY) < 20 {
		is.log(fmt.Sprintf("'%s' 발견! 같은 위치 — 바로 습득", matchedItemName))
		is.pickupInPlace()
		return
	}

	is.log(fmt.Sprintf("'%s' 발견! OCR위치=(%.0f,%.0f) 화면중심=(%.0f,%.0f) 차이=(%.0f,%.0f)",
		matchedItemName, matchX, matchY, centerX, centerY, diffX, diffY))
	is.pickupItem(matchedItemName, int(matchX), int(matchY), int(centerX), int(centerY))
}

// pickupInPlace 같은 위치에서 아이템 습득
func (is *ItemScanner) pickupInPlace() {
	is.km.Pause()
	time.Sleep(1000 * time.Millisecond) // 키 입력 완전 중지까지 대기

	robotgo.KeyTap(",")
	time.Sleep(300 * time.Millisecond)

	is.km.Resume()
	is.log("습득 완료, 매크로 재개")
}

// pressCtrlD Ctrl+D를 확실하게 입력 (KeyToggle 방식)
func (is *ItemScanner) pressCtrlD() {
	is.log("[키입력] Ctrl+D 전송 시작")
	robotgo.KeyToggle("ctrl", "down")
	time.Sleep(100 * time.Millisecond)
	robotgo.KeyTap("d")
	time.Sleep(100 * time.Millisecond)
	robotgo.KeyToggle("ctrl", "up")
	time.Sleep(100 * time.Millisecond)
	is.lastCtrlDTime = time.Now()
	is.log("[키입력] Ctrl+D 전송 완료")
}

// waitCtrlDCooldown Ctrl+D 쿨타임 잔여 시간만 대기 (8초 기준)
func (is *ItemScanner) waitCtrlDCooldown() {
	if is.lastCtrlDTime.IsZero() {
		return
	}
	elapsed := time.Since(is.lastCtrlDTime)
	remaining := 8*time.Second - elapsed
	if remaining > 0 {
		is.log(fmt.Sprintf("[쿨타임] Ctrl+D 잔여 %.1f초 대기...", remaining.Seconds()))
		time.Sleep(remaining)
	}
}

// moveByArrowKeys 타일모드에서 화살표키로 이동 후 엔터
// diffX>0: 오른쪽(right), diffX<0: 왼쪽(left)
// diffY>0: 아래(down), diffY<0: 위(up)
func (is *ItemScanner) moveByArrowKeys(diffX, diffY int) {
	is.log(fmt.Sprintf("[화살표이동] diffX=%d, diffY=%d", diffX, diffY))

	// X축 이동: diffX>0 → 오른쪽(right), diffX<0 → 왼쪽(left)
	if diffX != 0 {
		dir := "right"
		count := diffX
		if diffX < 0 {
			dir = "left"
			count = -diffX
		}
		is.log(fmt.Sprintf("[화살표이동] X축: %s x %d (diff=%+d)", dir, count, diffX))
		for i := 0; i < count; i++ {
			robotgo.KeyTap(dir)
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Y축 이동: diffY>0 → down키(Y증가), diffY<0 → up키(Y감소)
	if diffY != 0 {
		dir := "down"
		count := diffY
		if diffY < 0 {
			dir = "up"
			count = -diffY
		}
		is.log(fmt.Sprintf("[화살표이동] Y축: %s x %d (diff=%+d)", dir, count, diffY))
		for i := 0; i < count; i++ {
			robotgo.KeyTap(dir)
			time.Sleep(50 * time.Millisecond)
		}
	}

	// 엔터로 이동 확정
	time.Sleep(200 * time.Millisecond)
	robotgo.KeyTap("enter")
	is.log("[화살표이동] 엔터 입력 — 이동 확정")
}

// walkByArrowKeys 필드에서 방향키로 걸어가기 (Ctrl+D 없이)
// 바람의나라 이동: 방향키 1번=방향전환, 2번째부터=이동
// N칸 이동 = 같은 방향키 (N+1)번
func (is *ItemScanner) walkByArrowKeys(diffX, diffY int) {
	is.log(fmt.Sprintf("[걸어가기] diffX=%+d, diffY=%+d", diffX, diffY))

	// X축 이동
	if diffX != 0 {
		dir := "right"
		count := diffX
		if diffX < 0 {
			dir = "left"
			count = -diffX
		}
		// 방향전환 1번 + 이동 count번 = count+1번
		for i := 0; i < count+1; i++ {
			robotgo.KeyTap(dir)
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Y축 이동
	if diffY != 0 {
		dir := "down"
		count := diffY
		if diffY < 0 {
			dir = "up"
			count = -diffY
		}
		for i := 0; i < count+1; i++ {
			robotgo.KeyTap(dir)
			time.Sleep(200 * time.Millisecond)
		}
	}

	is.log("[걸어가기] 이동 완료")
}

// nearbyItem 추가 아이템 스캔 결과
type nearbyItem struct {
	name string
	x, y int
}

// scanForNearbyItem 주변 아이템 스캔 (맵 체크 생략, 아이템 OCR만)
func (is *ItemScanner) scanForNearbyItem(hwnd uint64, config ItemScannerConfig) *nearbyItem {
	img, _, err := is.om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		return nil
	}

	// 아이템 이미지 준비 (scanOnce와 동일한 크롭+필터)
	fullW := img.Bounds().Dx()
	fullH := img.Bounds().Dy()
	itemCropX := fullW * 17 / 100
	itemCropY := fullH * 13 / 100
	itemCropW := fullW * 53 / 100 // 17%~70% = 53%
	itemCropH := fullH * 57 / 100 // 13%~70% = 57%

	var itemImages []image.Image
	if itemCropW <= 0 || itemCropH <= 0 {
		return nil
	}

	itemCropped := img.SubImage(image.Rect(itemCropX, itemCropY, itemCropX+itemCropW, itemCropY+itemCropH))
	scaled4 := scaleImage(itemCropped, 4)

	// 초록+노란 통합 필터 (Green 필터 1개만 — 속도 최적화)
	s4B := scaled4.Bounds()
	w4, h4 := s4B.Dx(), s4B.Dy()
	greenResult := image.NewRGBA(image.Rect(0, 0, w4, h4))
	for y := 0; y < h4; y++ {
		for x := 0; x < w4; x++ {
			rv, gv, bv, _ := scaled4.At(x, y).RGBA()
			r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
			isGreen := false
			if g8 > r8+15 && g8 > b8+15 && g8 > 40 {
				isGreen = true
			}
			if !isGreen {
				maxC := r8
				if g8 > maxC {
					maxC = g8
				}
				if b8 > maxC {
					maxC = b8
				}
				minC := r8
				if g8 < minC {
					minC = g8
				}
				if b8 < minC {
					minC = b8
				}
				delta := maxC - minC
				if delta > 10 && maxC > 50 {
					sat := delta * 100 / maxC
					if sat > 15 {
						var hue int
						if maxC == g8 {
							hue = 120 + 60*(b8-r8)/delta
						} else if maxC == r8 {
							hue = 60 * (g8 - b8) / delta
							if hue < 0 {
								hue += 360
							}
						} else {
							hue = 240 + 60*(r8-g8)/delta
						}
						if hue >= 60 && hue <= 180 {
							isGreen = true
						}
					}
				}
			}
			if isGreen {
				greenResult.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				greenResult.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	itemImages = append(itemImages, greenResult)

	// 아이템만 OCR (맵 이미지 없음)
	batchResult, err := is.om.RecognizeBatchAll(nil, itemImages)
	if err != nil {
		return nil
	}

	// 좌표 역산 + 중복 제거
	scale := 4.0
	offsetX := float64(itemCropX)
	offsetY := float64(itemCropY)
	allWordsMap := make(map[string]OCRWord)
	for _, w := range batchResult.ItemWords {
		if w.Text == "" {
			continue
		}
		rw := OCRWord{
			Text:   w.Text,
			X:      w.X/scale + offsetX,
			Y:      w.Y/scale + offsetY,
			Width:  w.Width / scale,
			Height: w.Height / scale,
		}
		key := fmt.Sprintf("%s_%.0f_%.0f", rw.Text, rw.X/10, rw.Y/10)
		if _, exists := allWordsMap[key]; !exists {
			allWordsMap[key] = rw
		}
	}
	words := make([]OCRWord, 0, len(allWordsMap))
	for _, w := range allWordsMap {
		words = append(words, w)
	}

	// 아이템 매칭
	for _, item := range config.Items {
		if item.Name == "" {
			continue
		}
		targetName := item.Name
		for _, w := range words {
			if fuzzyMatchItem(w.Text, targetName) {
				mx := int(w.X + w.Width/2)
				my := int(w.Y + w.Height/2)
				return &nearbyItem{name: targetName, x: mx, y: my}
			}
		}
		// 연속 단어 조합
		for i := 0; i < len(words); i++ {
			combined := words[i].Text
			combinedX := words[i].X
			combinedY := words[i].Y
			combinedW := words[i].Width
			combinedH := words[i].Height
			for j := i + 1; j < len(words) && j <= i+3; j++ {
				combined += words[j].Text
				endX := words[j].X + words[j].Width
				endY := words[j].Y + words[j].Height
				if endX > combinedX+combinedW {
					combinedW = endX - combinedX
				}
				if endY > combinedY+combinedH {
					combinedH = endY - combinedY
				}
				if fuzzyMatchItem(combined, targetName) {
					mx := int(combinedX + combinedW/2)
					my := int(combinedY + combinedH/2)
					return &nearbyItem{name: targetName, x: mx, y: my}
				}
			}
		}
	}

	return nil
}

// pickupItem 아이템 위치로 이동 → 습득 → OCR 좌표 기반으로 원점 복귀
// 알고리즘:
//  1. 매크로 일시정지
//  2. Ctrl+D (타일모드) → 아이템 위치 더블클릭 → ESC
//  3. ',' 줍기
//  3-1. 추가 아이템 스캔 (최대 5회) → 있으면 연속 습득
//  4. OCR로 현재 좌표 읽기 → 원점까지 diff 계산
//  5. Ctrl+D → 화살표 키로 diff만큼 이동 → 엔터
//  6. OCR 검증
func (is *ItemScanner) pickupItem(itemName string, itemX, itemY, centerX, centerY int) {
	is.mutex.Lock()
	hwnd := is.hwnd
	config := is.config
	is.mutex.Unlock()

	is.log("========== 아이템 습득 시작 ==========")

	// 중지 체크 헬퍼
	checkStop := func() bool {
		if is.isStopped() {
			is.log("[중지] 사용자 중지 요청 — 아이템 습득 중단")
			is.km.Resume()
			return true
		}
		return false
	}

	// 1. 매크로 일시정지
	is.log("[1단계] 매크로 일시정지")
	is.km.Pause()
	time.Sleep(800 * time.Millisecond)

	if checkStop() {
		return
	}

	// 2. 이동 전 현재 좌표 읽기
	preCoords, preErr := is.om.ReadCoordinates(hwnd)
	if preErr == nil {
		is.log(fmt.Sprintf("[2단계] 이동 전 좌표: (%d,%d)", preCoords.X, preCoords.Y))
	}

	// 3. Ctrl+D → 더블클릭 → ESC → 이동 확인 (최대 3회 재시도)
	moved := false
	maxMoveAttempts := 2
	for attempt := 1; attempt <= maxMoveAttempts; attempt++ {
		is.log(fmt.Sprintf("[3단계] Ctrl+D → 더블클릭(%d,%d) (시도 %d/%d)", itemX, itemY, attempt, maxMoveAttempts))

		is.pressCtrlD()
		time.Sleep(500 * time.Millisecond)

		if err := is.ma.DoubleClickRelative(hwnd, itemX, itemY); err != nil {
			is.log(fmt.Sprintf("[3단계] 더블클릭 실패: %v", err))
			robotgo.KeyTap("escape")
			time.Sleep(300 * time.Millisecond)
			continue
		}
		time.Sleep(500 * time.Millisecond)

		// ESC로 타일모드 닫기
		robotgo.KeyTap("escape")
		time.Sleep(500 * time.Millisecond)

		if checkStop() {
			return
		}

		// 이동 성공 여부 확인 (좌표 변화 체크)
		if preErr != nil {
			// 이동 전 좌표를 못 읽었으면 이동 성공으로 간주
			is.log("[3단계] 이동 전 좌표 없음 — 이동 성공 간주")
			moved = true
			break
		}
		postCoords, postErr := is.om.ReadCoordinates(hwnd)
		if postErr != nil {
			is.log("[3단계] 이동 후 좌표 읽기 실패 — 이동 성공 간주")
			moved = true
			break
		}
		if postCoords.X != preCoords.X || postCoords.Y != preCoords.Y {
			is.log(fmt.Sprintf("[3단계] 이동 성공: (%d,%d) → (%d,%d)",
				preCoords.X, preCoords.Y, postCoords.X, postCoords.Y))
			moved = true
			break
		}
		is.log(fmt.Sprintf("[3단계] 이동 실패 (좌표 변화 없음) — 시도 %d/%d", attempt, maxMoveAttempts))
	}

	if !moved {
		is.log(fmt.Sprintf("[3단계] 2회 이동 시도 실패 — '%s' @ (%d,%d) 무시 목록 등록", itemName, itemX, itemY))
		is.ignoredItems = append(is.ignoredItems, ignoredItem{name: itemName, x: itemX, y: itemY})
		is.km.Resume()
		return
	}

	// 4. 아이템 습득
	is.log("[4단계] ',' 아이템 습득")
	robotgo.KeyTap(",")
	time.Sleep(300 * time.Millisecond)

	// 4-1. 연속습득: OCR 스캔 → 아이템 있으면 쿨타임 대기 후 이동+습득 반복
	for cycle := 1; cycle <= 5; cycle++ {
		if checkStop() {
			return
		}

		// 2초 간격 스캔, 3회 연속 없으면 조기 종료
		var lastItem *nearbyItem
		scanCount := 0
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if checkStop() {
				return
			}
			time.Sleep(2 * time.Second)
			scanCount++
			item := is.scanForNearbyItem(hwnd, config)
			if item != nil {
				lastItem = item
				is.log(fmt.Sprintf("[연속습득] 사이클%d 스캔%d — '%s' @ (%d,%d)",
					cycle, scanCount, item.name, item.x, item.y))
			} else if lastItem == nil && scanCount >= 3 {
				// 3회 연속 아이템 없으면 조기 종료
				break
			}
		}

		if lastItem == nil {
			is.log(fmt.Sprintf("[연속습득] 사이클 %d — %d회 스캔, 아이템 없음", cycle, scanCount))
			break
		}

		is.log(fmt.Sprintf("[연속습득] 사이클 %d — '%s' @ (%d,%d) 이동+습득",
			cycle, lastItem.name, lastItem.x, lastItem.y))

		// 화면 중심 근처(20px 이내)면 바로 줍기
		if math.Abs(float64(lastItem.x-centerX)) < 20 && math.Abs(float64(lastItem.y-centerY)) < 20 {
			is.log("[연속습득] 같은 위치 — 바로 습득")
			robotgo.KeyTap(",")
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// Ctrl+D 쿨타임 잔여 대기 → 이동 → 습득
		is.waitCtrlDCooldown()
		is.pressCtrlD()
		time.Sleep(500 * time.Millisecond)
		if err := is.ma.DoubleClickRelative(hwnd, lastItem.x, lastItem.y); err != nil {
			is.log(fmt.Sprintf("[연속습득] 더블클릭 실패: %v", err))
			robotgo.KeyTap("escape")
			time.Sleep(300 * time.Millisecond)
			break
		}
		time.Sleep(500 * time.Millisecond)
		robotgo.KeyTap("escape")
		time.Sleep(500 * time.Millisecond)
		robotgo.KeyTap(",")
		time.Sleep(300 * time.Millisecond)
	}

	// 5. 원점 복귀
	if config.OriginX > 0 || config.OriginY > 0 {
		// 현재 좌표 읽어서 원점과의 거리 확인
		postCoords, postErr := is.om.ReadCoordinates(hwnd)
		if postErr == nil {
			dX := config.OriginX - postCoords.X
			dY := config.OriginY - postCoords.Y
			absDX, absDY := dX, dY
			if absDX < 0 {
				absDX = -absDX
			}
			if absDY < 0 {
				absDY = -absDY
			}

			if absDX <= 3 && absDY <= 3 && (dX != 0 || dY != 0) {
				// ±3칸 이내 → 걸어서 복귀 (최대 10회 재시도)
				for walkAttempt := 1; walkAttempt <= 10; walkAttempt++ {
					is.log(fmt.Sprintf("[원점복귀] 걸어가기 시도 %d/10: (%d,%d) → (%d,%d) diff=(%+d,%+d)",
						walkAttempt, postCoords.X, postCoords.Y, config.OriginX, config.OriginY, dX, dY))
					is.walkByArrowKeys(dX, dY)

					// 좌표 검증
					verifyCoords, verifyErr := is.om.ReadCoordinates(hwnd)
					if verifyErr != nil {
						is.log("[원점복귀] 좌표 검증 실패 — Ctrl+D 폴백")
						if !is.moveToOrigin(hwnd, config.OriginX, config.OriginY) {
							is.mapConfirmed = false
						}
						break
					}
					if verifyCoords.X == config.OriginX && verifyCoords.Y == config.OriginY {
						is.log(fmt.Sprintf("[원점복귀] 도착 확인: (%d,%d)", verifyCoords.X, verifyCoords.Y))
						is.ignoredItems = nil
						break
					}
					// 남은 거리 재계산
					dX = config.OriginX - verifyCoords.X
					dY = config.OriginY - verifyCoords.Y
					postCoords = verifyCoords
					is.log(fmt.Sprintf("[원점복귀] 미도착 — 남은 diff=(%+d,%+d)", dX, dY))
				}
			} else if dX != 0 || dY != 0 {
				// 4칸 이상 → 기존 Ctrl+D 이동
				if !is.moveToOrigin(hwnd, config.OriginX, config.OriginY) {
					is.mapConfirmed = false
				}
			}
			// dX==0 && dY==0 → 이미 원점, 이동 불필요
		} else {
			// 좌표 읽기 실패 → 기존 방식
			if !is.moveToOrigin(hwnd, config.OriginX, config.OriginY) {
				is.mapConfirmed = false
			}
		}
	}

	// 6. 매크로 재개
	is.km.Resume()
	is.log("========== 아이템 습득 완료 ==========")
}

// TestScan 테스트용 1회 스캔 (결과 반환)
func (is *ItemScanner) TestScan(hwnd uint64) ([]OCRWord, string, error) {
	config := is.GetConfig()

	img, _, err := is.om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		return nil, "", fmt.Errorf("화면 캡처 실패: %v", err)
	}

	windowW := float64(img.Bounds().Dx())
	windowH := float64(img.Bounds().Dy())

	words, err := is.om.RecognizeWithPositions(img)
	if err != nil {
		return nil, "", fmt.Errorf("OCR 실패: %v", err)
	}

	centerX := windowW / 2
	centerY := windowH / 2

	// 모든 아이템에 대해 검색
	var matchResults []string
	for _, item := range config.Items {
		if item.Name == "" {
			continue
		}
		targetName := item.Name
		matched := false

		// 단일 단어 매칭
		for _, w := range words {
			if fuzzyMatchItem(w.Text, targetName) {
				matchX := w.X + w.Width/2
				matchY := w.Y + w.Height/2
				matchResults = append(matchResults, fmt.Sprintf("[%s] '%s' 발견: '%s' @ (%.0f,%.0f) 클릭=(%d,%d)",
					item.Color, targetName, w.Text, matchX, matchY, int(matchX), int(matchY)))
				matched = true
				break
			}
		}
		if matched {
			continue
		}

		// 연속 단어 합치기
		for i := 0; i < len(words); i++ {
			combined := words[i].Text
			combinedX := words[i].X
			combinedW := words[i].Width
			combinedY := words[i].Y
			combinedH := words[i].Height
			for j := i + 1; j < len(words) && j <= i+3; j++ {
				combined += words[j].Text
				endX := words[j].X + words[j].Width
				endY := words[j].Y + words[j].Height
				if endX > combinedX+combinedW {
					combinedW = endX - combinedX
				}
				if endY > combinedY+combinedH {
					combinedH = endY - combinedY
				}
				if fuzzyMatchItem(combined, targetName) {
					matchX := combinedX + combinedW/2
					matchY := combinedY + combinedH/2
					matchResults = append(matchResults, fmt.Sprintf("[%s] '%s' 발견(합성): '%s' @ (%.0f,%.0f) 클릭=(%d,%d)",
						item.Color, targetName, combined, matchX, matchY, int(matchX), int(matchY)))
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			matchResults = append(matchResults, fmt.Sprintf("'%s' 미발견", targetName))
		}
	}

	msg := fmt.Sprintf("화면(%dx%d) 중심=(%.0f,%.0f) 단어 %d개\n%s",
		int(windowW), int(windowH), centerX, centerY, len(words),
		strings.Join(matchResults, "\n"))

	return words, msg, nil
}
