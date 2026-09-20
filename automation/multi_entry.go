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

// meModeCfg 모드별 맵 판별 설정 (대야/칸첸)
type meModeCfg struct {
	modeName     string   // 로그용: "대야" | "칸첸"
	entranceName string   // 예: 대야산기슭
	insideName   string   // 예: 대야전투
	entranceKeys []string // 입구 확정 키워드 (우선순위 높음)
	insideKeys   []string // 전투/사냥맵 보조 키워드
}

func meCfgFor(mode string) (meModeCfg, error) {
	switch mode {
	case "daeya":
		return meModeCfg{
			modeName:     "대야",
			entranceName: "대야산기슭",
			insideName:   "대야전투",
			entranceKeys: []string{"기슭", "산기"},
			insideKeys:   []string{"전투"},
		}, nil
	case "kanchen":
		return meModeCfg{
			modeName:     "칸첸",
			entranceName: "칸첸중가설산초입",
			insideName:   "칸첸중가설산",
			entranceKeys: []string{"초입"},
			insideKeys:   []string{"설산", "중가", "칸첸"},
		}, nil
	default:
		return meModeCfg{}, fmt.Errorf("알 수 없는 모드: %s", mode)
	}
}

// EntryWindow 다중 입장 창 1개: 창 핸들 + 모드("daeya"|"kanchen") + 입력 방식.
// 창마다 모드와 입력 방식을 다르게 섞을 수 있다
// (예: 2창 대야 + 1창 칸첸, 메인 캐릭만 포그라운드 + 나머지는 백그라운드).
type EntryWindow struct {
	HWND uint64
	Mode string
	BG   bool // true=백그라운드(창 안 띄움), false=포그라운드(창을 앞으로)
}

// MultiEntry 다중 창 솔로 입장 유지 루프 (대야/칸첸 혼합 가능).
// 게임에서 그룹 입장이 사라져 창마다 각자 입장해야 함 → 최대 4개 창을 순환(와리가리)하며:
//   창 활성화 → 맵 OCR (창별 모드 설정으로 판별)
//     입구맵(대야산기슭/칸첸중가설산초입) → 입장(o→Enter→Enter→ESC) → 전투맵 확인 → 최소화
//     전투맵(대야전투/칸첸중가설산)       → 그대로 최소화
//   다음 창 → … 한 바퀴 후 대기, 약 30초 주기로 반복.
// 스킬/아이템 로직은 안 함(사냥은 게임 내 자동사냥). 창 1개짜리 기존 로직과 별개.
type MultiEntry struct {
	wm *WindowManager
	om *OCRManager

	mu       sync.Mutex
	running   bool
	stopChan  chan struct{}
	entries   []EntryWindow
	minimize  bool // 입장 후 창 최소화 여부 (옵션, 포그라운드 창에만 적용)
	centerX   int  // 칸첸 창 중앙 이동 목표 (0,0이면 이동 안 함, 칸첸 창에만 적용)
	centerY   int
	centerSet bool

	roundInterval time.Duration // 한 바퀴 주기 (기본 30초)
	logFunc       func(string)
}

// NewMultiEntry 생성
func NewMultiEntry(wm *WindowManager, om *OCRManager) *MultiEntry {
	return &MultiEntry{wm: wm, om: om, roundInterval: 30 * time.Second}
}

func (me *MultiEntry) SetLogFunc(f func(string)) { me.logFunc = f }

func (me *MultiEntry) log(format string, a ...interface{}) {
	m := fmt.Sprintf(format, a...)
	log.Printf("[다중입장] %s", m)
	if me.logFunc != nil {
		me.logFunc(m)
	}
}

// IsRunning 동작 여부
func (me *MultiEntry) IsRunning() bool {
	me.mu.Lock()
	defer me.mu.Unlock()
	return me.running
}

// Start 다중 입장 유지 시작 (단일 모드 — 모든 창 동일 모드). 하위 호환용 래퍼.
func (me *MultiEntry) Start(mode string, hwnds []uint64, minimize bool, centerX, centerY int) error {
	entries := make([]EntryWindow, 0, len(hwnds))
	for _, h := range hwnds {
		entries = append(entries, EntryWindow{HWND: h, Mode: mode})
	}
	return me.StartEntries(entries, minimize, centerX, centerY)
}

// StartEntries 다중 입장 유지 시작 (창별 모드 혼합 가능, 최대 4개).
// minimize: 입장 후 창 최소화 여부. centerX/Y: 칸첸 창 입장 후 이동할 중앙 좌표(0,0이면 이동 안 함).
func (me *MultiEntry) StartEntries(entries []EntryWindow, minimize bool, centerX, centerY int) error {
	me.mu.Lock()
	if me.running {
		me.mu.Unlock()
		return fmt.Errorf("다중 입장이 이미 실행 중")
	}
	if len(entries) == 0 {
		me.mu.Unlock()
		return fmt.Errorf("창이 지정되지 않음")
	}
	if len(entries) > 4 {
		entries = entries[:4]
	}

	// 모드 유효성 검증 + 로그용 요약
	counts := map[string]int{}
	for _, e := range entries {
		cfg, err := meCfgFor(e.Mode)
		if err != nil {
			me.mu.Unlock()
			return err
		}
		counts[cfg.modeName]++
	}

	// 최소화는 창을 내려버리므로 백그라운드 캡처(PrintWindow)가 불가능하다.
	// 최소화를 켜면 모든 창을 포그라운드로 강제한다 (UI에서도 막지만 방어적으로).
	if minimize {
		for i := range entries {
			entries[i].BG = false
		}
	}
	me.entries = append([]EntryWindow(nil), entries...)
	me.minimize = minimize
	me.centerX, me.centerY = centerX, centerY
	me.centerSet = centerX > 0 || centerY > 0
	me.running = true
	me.stopChan = make(chan struct{})
	stop := me.stopChan
	me.mu.Unlock()

	go me.run(stop)
	summary := ""
	for name, n := range counts {
		if summary != "" {
			summary += " + "
		}
		summary += fmt.Sprintf("%s %d개", name, n)
	}
	nBG := 0
	for _, e := range entries {
		if e.BG {
			nBG++
		}
	}
	extra := ""
	switch {
	case nBG == len(entries):
		extra += " +전부 백그라운드"
	case nBG > 0:
		extra += fmt.Sprintf(" +백그라운드 %d개/포그라운드 %d개", nBG, len(entries)-nBG)
	}
	if me.minimize {
		extra += " +최소화(포그라운드 창만)"
	}
	if me.centerSet {
		extra += fmt.Sprintf(" +칸첸중앙(%d,%d)", centerX, centerY)
	}
	me.log("다중 입장 유지 시작 — %s, 약 %.0f초 주기%s", summary, me.roundInterval.Seconds(), extra)
	return nil
}

// Stop 중지
func (me *MultiEntry) Stop() {
	me.mu.Lock()
	defer me.mu.Unlock()
	if !me.running {
		return
	}
	me.running = false
	select {
	case <-me.stopChan:
	default:
		close(me.stopChan)
	}
	me.log("다중 입장 유지 중지")
}

func (me *MultiEntry) stopped(stop chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

func (me *MultiEntry) sleepOrStop(stop chan struct{}, d time.Duration) bool {
	select {
	case <-stop:
		return false
	case <-time.After(d):
		return true
	}
}

func (me *MultiEntry) run(stop chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			me.log("패닉 복구: %v", r)
		}
		me.mu.Lock()
		me.running = false
		me.mu.Unlock()
	}()

	for !me.stopped(stop) {
		roundStart := time.Now()

		me.mu.Lock()
		entries := append([]EntryWindow(nil), me.entries...)
		me.mu.Unlock()

		for i, e := range entries {
			if me.stopped(stop) {
				return
			}
			me.handleWindow(stop, i+1, e)
		}

		// 한 바퀴 소요 시간을 빼고 남은 만큼 대기 (최소 3초)
		remain := me.roundInterval - time.Since(roundStart)
		if remain < 3*time.Second {
			remain = 3 * time.Second
		}
		me.log("한 바퀴 완료 — %.0f초 후 재점검", remain.Seconds())
		if !me.sleepOrStop(stop, remain) {
			return
		}
	}
}

// ===== 입력/캡처 추상화 =====
// 비활성 모드(bgMode)면 창을 앞으로 가져오지 않고 PostMessage/PrintWindow로 처리하고,
// 아니면 기존대로 포그라운드(robotgo/BitBlt)를 쓴다.

// tap 키 1회 입력 (mods는 조합키). bg=true면 백그라운드 전송.
func (me *MultiEntry) tap(hwnd uint64, bg bool, key string, mods ...string) {
	if bg {
		if err := BgKeyTap(hwnd, key, mods...); err != nil {
			me.log("비활성 키 입력 실패(%s): %v", key, err)
		}
		return
	}
	for _, m := range mods {
		robotgo.KeyToggle(m, "down")
		time.Sleep(50 * time.Millisecond)
	}
	robotgo.KeyTap(key)
	for i := len(mods) - 1; i >= 0; i-- {
		time.Sleep(50 * time.Millisecond)
		robotgo.KeyToggle(mods[i], "up")
	}
}

// captureWindow 화면 캡처. 비활성 모드는 창을 앞으로 끌어오지 않는다.
// (기존 CaptureWindowRaw는 내부에서 ActivateWindow를 호출한다)
func (me *MultiEntry) captureWindow(hwnd uint64, bg bool) (*image.RGBA, error) {
	if bg {
		img, _, err := me.wm.CaptureWindowBG(hwnd)
		return img, err
	}
	img, _, err := me.wm.CaptureWindowRaw(hwnd)
	return img, err
}

// readCoords Ctrl+D 좌표창의 좌표 읽기. 비활성 모드는 BG 캡처 + 이미지 기반 인식.
func (me *MultiEntry) readCoords(hwnd uint64, bg bool) (GameCoords, error) {
	if !bg {
		return me.om.ReadCoordinates(hwnd)
	}
	img, _, err := me.wm.CaptureWindowBG(hwnd)
	if err != nil {
		return GameCoords{}, err
	}
	c, _, err := me.om.ReadCoordinatesFromImage(img)
	return c, err
}

// handleWindow 창 1개 점검: 활성화 → 맵 판별(창별 모드) → 입구면 입장 → 최소화.
func (me *MultiEntry) handleWindow(stop chan struct{}, idx int, entry EntryWindow) {
	hwnd := entry.HWND
	cfg, err := meCfgFor(entry.Mode)
	if err != nil {
		me.log("창%d 모드 오류: %v — 건너뜀", idx, err)
		return
	}

	if !me.wm.IsWindowValid(hwnd) {
		me.log("창%d(hwnd=%d) 유효하지 않음 — 건너뜀", idx, hwnd)
		return
	}

	bg := entry.BG
	if !bg {
		me.wm.ActivateWindow(hwnd)
		if !me.sleepOrStop(stop, 900*time.Millisecond) { // 복원/렌더 대기
			return
		}
	}

	state, raw := me.detectState(hwnd, bg, cfg)
	inputName := "포그라운드"
	if bg {
		inputName = "백그라운드"
	}
	me.log("창%d[%s/%s] 맵='%s' → %s", idx, cfg.modeName, inputName, raw, state)

	switch state {
	case "entrance":
		me.enterOnce(stop, hwnd, bg)
		// 입장/로딩 대기 후 확인
		if !me.sleepOrStop(stop, 4*time.Second) {
			return
		}
		state, raw = me.detectState(hwnd, bg, cfg)
		if state == "inside" {
			me.log("창%d[%s] 입장 성공 (%s)", idx, cfg.modeName, raw)
		} else {
			me.log("창%d[%s] 입장 확인 안 됨 (%s) — 다음 바퀴 재시도", idx, cfg.modeName, raw)
		}
	case "inside":
		// 이미 사냥터
	default:
		// 맵 미상 — 칸첸은 단일창 모드(km.KanchenEnter)가 맵 확인 없이 입장 키를
		// 반복해도 안전하다고 검증됐으므로(사냥맵 안에서 눌러도 무해) 입장을 시도한다.
		// 대야는 OCR 분류가 안정적이므로 기존대로 다음 바퀴 재확인.
		if entry.Mode == "kanchen" {
			me.log("창%d[칸첸] 맵 미상 (OCR='%s') — 단일창 방식으로 입장 시도", idx, raw)
			me.enterOnce(stop, hwnd, bg)
			if !me.sleepOrStop(stop, 4*time.Second) {
				return
			}
			state, raw = me.detectState(hwnd, bg, cfg)
			me.log("창%d[칸첸] 입장 시도 후 맵='%s' → %s", idx, raw, state)
		} else {
			me.log("창%d[%s] 맵 미상 — 다음 바퀴 재확인", idx, cfg.modeName)
		}
	}

	// 칸첸 창: 사냥맵이면 중앙 좌표로 이동 (아이템은 안 먹고 자리만). 대야 창은 이동 안 함.
	if me.centerSet && entry.Mode == "kanchen" && state == "inside" {
		me.moveToCenter(stop, idx, hwnd, bg)
	}

	// 최소화는 옵션 (기본 꺼짐). 최소화를 켜면 위에서 모든 창을 포그라운드로
	// 강제하므로 여기서 bg는 항상 false다.
	if me.minimize && !bg {
		me.wm.MinimizeWindow(hwnd)
	}
	me.sleepOrStop(stop, 300*time.Millisecond)
}

// moveToCenter Ctrl+D 좌표창을 열어 현재 좌표를 읽고 중앙 좌표로 방향키 이동(1회).
// 오버슈팅해도 다음 바퀴에 보정. 아이템 습득은 하지 않는다.
func (me *MultiEntry) moveToCenter(stop chan struct{}, idx int, hwnd uint64, bg bool) {
	// Ctrl+D 좌표창 열기
	me.tap(hwnd, bg, "d", "ctrl")
	if !me.sleepOrStop(stop, 500*time.Millisecond) {
		return
	}
	c, err := me.readCoords(hwnd, bg)
	if err != nil {
		me.log("창%d 좌표 읽기 실패 — 중앙 이동 생략", idx)
		me.tap(hwnd, bg, "escape")
		return
	}
	dx, dy := me.centerX-c.X, me.centerY-c.Y
	if meAbs(dx) <= 1 && meAbs(dy) <= 1 {
		me.log("창%d (%d,%d) 이미 중앙 근처", idx, c.X, c.Y)
		me.tap(hwnd, bg, "escape")
		return
	}
	dx, dy = meClamp(dx, dy, 7) // 과도한 점프 방지
	me.log("창%d (%d,%d) → 중앙(%d,%d) 이동: dx=%+d dy=%+d", idx, c.X, c.Y, me.centerX, me.centerY, dx, dy)
	me.arrows(hwnd, bg, dx, dy)
	me.sleepOrStop(stop, 400*time.Millisecond)
	me.tap(hwnd, bg, "escape") // 좌표창 닫기
}

// arrows 좌표창(Ctrl+D) 타일모드 방향키 이동 후 엔터.
func (me *MultiEntry) arrows(hwnd uint64, bg bool, dx, dy int) {
	if dx != 0 {
		dir, n := "right", dx
		if dx < 0 {
			dir, n = "left", -dx
		}
		for i := 0; i < n; i++ {
			me.tap(hwnd, bg, dir)
			time.Sleep(50 * time.Millisecond)
		}
	}
	if dy != 0 {
		dir, n := "down", dy
		if dy < 0 {
			dir, n = "up", -dy
		}
		for i := 0; i < n; i++ {
			me.tap(hwnd, bg, dir)
			time.Sleep(50 * time.Millisecond)
		}
	}
	time.Sleep(200 * time.Millisecond)
	me.tap(hwnd, bg, "enter")
}

func meAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func meClamp(dx, dy, max int) (int, int) {
	t := meAbs(dx) + meAbs(dy)
	if t <= max || t == 0 {
		return dx, dy
	}
	nx, ny := dx*max/t, dy*max/t
	if nx == 0 && dx != 0 {
		nx = dx / meAbs(dx)
	}
	if ny == 0 && dy != 0 {
		ny = dy / meAbs(dy)
	}
	return nx, ny
}

// DebugDetect 디버깅용: 창의 맵 OCR 원문과 해당 모드 분류 결과를 반환.
// 실제 루프와 동일한 크롭/OCR/분류 경로를 탄다.
func (me *MultiEntry) DebugDetect(hwnd uint64, mode string) (mapName, state string, err error) {
	cfg, err := meCfgFor(mode)
	if err != nil {
		return "", "", err
	}
	st, raw := me.detectState(hwnd, false, cfg)
	return raw, st, nil
}

// detectState 상단 중앙 맵 이름 OCR → "entrance" | "inside" | "unknown" (+원본 텍스트).
func (me *MultiEntry) detectState(hwnd uint64, bg bool, cfg meModeCfg) (string, string) {
	rawImg, err := me.captureWindow(hwnd, bg)
	if err != nil {
		return "unknown", ""
	}
	name := me.detectMapName(hwnd, rawImg)
	if name == "" {
		return "unknown", ""
	}
	return me.classify(name, cfg), name
}

// cropMapRegion 게임 화면 상단 중앙 맵 이름 영역 크롭 (daeya_battle과 동일).
func (me *MultiEntry) cropMapRegion(hwnd uint64, img *image.RGBA) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()

	clientOffX, clientOffY, err := me.wm.GetClientOffset(hwnd)
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
		return nil
	}

	return img.SubImage(image.Rect(
		bounds.Min.X+cropX, bounds.Min.Y+cropY,
		bounds.Min.X+cropX+cropW, bounds.Min.Y+cropY+cropH,
	))
}

// detectMapName 상단 중앙 맵 이름 OCR — 아이템 스캐너와 동일한 8x 방식
// (RecognizeText의 흰색 이진화는 칸첸 맵 이름 색상을 지워 빈 값이 나옴).
func (me *MultiEntry) detectMapName(hwnd uint64, img *image.RGBA) string {
	cropped := me.cropMapRegion(hwnd, img)
	if cropped == nil {
		return ""
	}
	text, err := me.om.RecognizeMapName(cropped)
	if err != nil {
		return ""
	}
	return text
}

// DebugMapInfo 진단용: 맵 이름 OCR 텍스트 + 크롭 이미지 반환 (창 감지 UI 표시용).
func (me *MultiEntry) DebugMapInfo(hwnd uint64) (string, image.Image, error) {
	rawImg, _, err := me.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		return "", nil, err
	}
	cropped := me.cropMapRegion(hwnd, rawImg)
	if cropped == nil {
		return "", nil, fmt.Errorf("크롭 영역 계산 실패")
	}
	text, err := me.om.RecognizeMapName(cropped)
	if err != nil {
		return "", cropped, nil // OCR 실패여도 이미지는 반환 (눈으로 확인용)
	}
	return text, cropped, nil
}

// classify 맵 이름 → entrance/inside/unknown.
// 칸첸은 입구맵 이름(칸첸중가설산초입)이 사냥맵 이름(칸첸중가설산)을 포함하므로
// "입구 우선" 같은 단순 우선순위는 사냥맵을 입구로 오판한다 (사냥맵 안에서
// 입장 시퀀스를 눌러 오작동). → 입구 확정 키워드 우선, 그 외엔 거리가
// 더 가까운 쪽을 선택한다.
func (me *MultiEntry) classify(mapName string, cfg meModeCfg) string {
	nameRunes := []rune(mapName)

	// 1) 입구 확정 키워드 (초입/기슭 등) — 있으면 무조건 입구
	for _, k := range cfg.entranceKeys {
		if strings.Contains(mapName, k) {
			return "entrance"
		}
	}

	// 2) Levenshtein 50% 매칭
	entRunes := []rune(cfg.entranceName)
	inRunes := []rune(cfg.insideName)
	entDist := levenshteinRunes(nameRunes, entRunes)
	inDist := levenshteinRunes(nameRunes, inRunes)
	entMax := len(entRunes) * 50 / 100
	if entMax < 1 {
		entMax = 1
	}
	inMax := len(inRunes) * 50 / 100
	if inMax < 1 {
		inMax = 1
	}
	entMatch := entDist <= entMax
	inMatch := inDist <= inMax

	// 3) 사냥맵 보조 키워드 (거리 매칭 실패 시)
	if !inMatch {
		for _, k := range cfg.insideKeys {
			if strings.Contains(mapName, k) {
				inMatch = true
				inDist = inMax // 키워드 매칭은 최대 허용 거리로 취급
				break
			}
		}
	}

	// 4) 둘 다 매칭이면 거리가 가까운 쪽 (같으면 입구 — 입구 오판이 덜 위험).
	//    입구 확정 키워드(초입 등)가 없는데 사냥맵 이름과 정확히 일치하면 inside가 된다.
	if entMatch && inMatch {
		if inDist < entDist {
			return "inside"
		}
		return "entrance"
	}
	if entMatch {
		return "entrance"
	}
	if inMatch {
		return "inside"
	}
	return "unknown"
}

// enterOnce 입구맵 입장 시퀀스 1회: o → Enter → Enter → ESC (1~3초 랜덤 딜레이).
func (me *MultiEntry) enterOnce(stop chan struct{}, hwnd uint64, bg bool) {
	keys := []string{"o", "enter", "enter", "esc"}
	for _, key := range keys {
		if me.stopped(stop) {
			return
		}
		me.tap(hwnd, bg, key)
		delay := 1*time.Second + time.Duration(rand.Intn(2001))*time.Millisecond
		me.log("[입장] '%s' 키 (대기 %.1f초)", key, delay.Seconds())
		if !me.sleepOrStop(stop, delay) {
			return
		}
	}
}
