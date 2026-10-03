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

// 시련 던전 / 위치 — API·저장 파일과 같은 값.
// 게임의 시련 던전(대야/칸첸)은 사용자가 시련장 NPC에서 직접 바꾸고, 도우미는 시련 탭에서 고른 값을 따른다.
// 던전만 다를 뿐 흐름(시련장 → 입장 → 던전 → 시련장 복귀)은 같다.
const (
	TrialDungeonDaeya   = "daeya"
	TrialDungeonKanchen = "kanchen"
	TrialPlaceLobby     = "lobby" // 환상의시련장
)

// 시련 진행 단계 — GET /api/trial/status 의 phase
const (
	TrialPhaseIdle    = "대기"
	TrialPhaseLobby   = "시련장 대기"
	TrialPhaseEnter   = "입장"
	TrialPhaseMapWait = "맵 이동 대기"
	TrialPhaseBattle  = "전투"
	TrialPhaseAbility = "능력 선택"
	TrialPhaseDone    = "완료"
)

// trialSkillGap 칸첸 던전 스킬 키 사이 대기 (다중 입장 스킬과 같은 간격)
const trialSkillGap = 300 * time.Millisecond

// TrialConfig 시련 자동화 설정
type TrialConfig struct {
	MaxRuns int    // 최대 반복 횟수 (기본 10)
	Dungeon string // 시련 던전: "daeya" | "kanchen"
	TargetX int    // 목표 좌표 X — 시작할 때마다 던전에 맞춰 정한다
	TargetY int    // 목표 좌표 Y
}

// DefaultTrialConfig 기본 시련 설정
func DefaultTrialConfig() TrialConfig {
	return TrialConfig{
		MaxRuns: 10,
		Dungeon: TrialDungeonDaeya,
		TargetX: 29,
		TargetY: 32,
	}
}

// TrialWindow 시련 창 1개: 창 핸들 + 입력 방식 + 캐릭터 이름 + 던전에서 누를 스킬 키.
type TrialWindow struct {
	HWND uint64
	BG   bool   // true=백그라운드(창 안 띄움 — PostMessage/PrintWindow), false=포그라운드(기존: 창 활성화 + robotgo)
	Nick string // 창 감지로 읽은 캐릭터 이름 (못 읽었으면 "") — 로그·스킬 키 선택용
	// Skills 칸첸 던전에 있는 동안 2초마다 누를 스킬 키 (시련 전용 표에서 고른 것). 비면 안 누른다.
	Skills []string
}

// TrialStatus 시련 진행 상태 (UI 상단 표시용)
type TrialStatus struct {
	Running  bool   `json:"running"`
	RunCount int    `json:"runCount"`
	MaxRuns  int    `json:"maxRuns"`
	Dungeon  string `json:"dungeon"`
	Group    bool   `json:"group"`
	Phase    string `json:"phase"`
}

// Trial 시련 자동화 관리자
type Trial struct {
	om     *OCRManager
	km     *KeyboardManager
	wm     *WindowManager
	config TrialConfig // 다음 시작에 쓸 설정 (SetConfig)
	cfg    TrialConfig // 이번 실행 설정 — Start 가 config 를 복사해 두고 루프는 이것만 읽는다

	solo    TrialWindow // 솔로
	leader  TrialWindow // 그룹
	member  TrialWindow
	isGroup bool

	stopChan chan struct{}
	done     chan struct{} // 루프 고루틴이 끝나면 닫힌다
	running  bool
	runCount int
	phase    string
	mutex    sync.Mutex
	logFunc  func(string)

	// 루프 고루틴 전용 (Start 에서 초기화)
	dungeonSeen map[uint64]bool // 이번 회차에 "던전 확인" 로그를 남긴 창
	wrongWarned map[uint64]bool // 이번 회차에 "다른 던전" 경고를 남긴 창
	logged      map[string]bool // 실행마다 한 번만 남길 로그 (입력·캡처 실패 등)
}

// NewTrial 새로운 시련 관리자 생성
func NewTrial(om *OCRManager, km *KeyboardManager, wm *WindowManager) *Trial {
	return &Trial{
		om:          om,
		km:          km,
		wm:          wm,
		config:      DefaultTrialConfig(),
		phase:       TrialPhaseIdle,
		dungeonSeen: map[uint64]bool{},
		wrongWarned: map[uint64]bool{},
		logged:      map[string]bool{},
	}
}

// GetConfig 현재 설정 반환
func (t *Trial) GetConfig() TrialConfig {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.config
}

// SetConfig 설정 업데이트 — 다음 시작부터 쓴다 (실행 중인 시련은 시작할 때 설정 그대로)
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

// logOnce 실행마다 한 번만 남길 로그 (2초마다 반복되는 실패가 로그를 덮지 않게). 루프 고루틴 전용.
func (t *Trial) logOnce(id, msg string) {
	if t.logged[id] {
		return
	}
	t.logged[id] = true
	t.log(msg)
}

// IsRunning 실행 중 여부
func (t *Trial) IsRunning() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.running
}

// Status 진행 상태. 실행 중이면 이번 실행 설정을, 아니면 다음 시작에 쓸 설정을 보여준다.
func (t *Trial) Status() TrialStatus {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	cfg := t.config
	if t.running {
		cfg = t.cfg
	}
	phase := t.phase
	if phase == "" {
		phase = TrialPhaseIdle
	}
	return TrialStatus{
		Running:  t.running,
		RunCount: t.runCount,
		MaxRuns:  cfg.MaxRuns,
		Dungeon:  NormalizeTrialDungeon(cfg.Dungeon),
		Group:    t.isGroup,
		Phase:    phase,
	}
}

// setPhase 진행 단계 갱신 (상태 표시용). 중지된 뒤엔 바꾸지 않는다.
func (t *Trial) setPhase(p string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.running {
		t.phase = p
	}
}

// incRun 완료 횟수 +1 (상태 API 가 같이 읽으므로 잠금)
func (t *Trial) incRun() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.runCount++
}

// newRound 회차마다 한 번씩 남기는 로그 기록 초기화
func (t *Trial) newRound() {
	t.dungeonSeen = map[uint64]bool{}
	t.wrongWarned = map[uint64]bool{}
}

func (t *Trial) isStopped() bool {
	select {
	case <-t.stopChan:
		return true
	default:
		return false
	}
}

// begin 실행 상태를 새로 잡는다. 이미 실행 중이면 false.
// 중지 직후 바로 다시 시작하면 직전 루프가 아직 정리 중일 수 있다 — 그 루프가 새 stopChan 을 보고
// 계속 돌면 키가 두 번씩 들어가므로 끝날 때까지 기다린다(최대 10초).
func (t *Trial) begin(group bool, solo, leader, member TrialWindow) bool {
	t.mutex.Lock()
	running, prev := t.running, t.done
	t.mutex.Unlock()
	if running {
		t.log("이미 실행 중 — 시작 안 함")
		return false
	}
	if prev != nil {
		select {
		case <-prev:
		case <-time.After(10 * time.Second):
			t.log("이전 시련이 아직 정리 중이라 시작하지 않았습니다 — 잠시 후 다시 시작해주세요")
			return false
		}
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.running {
		return false
	}
	cfg := t.config
	cfg.Dungeon = NormalizeTrialDungeon(cfg.Dungeon)
	if cfg.MaxRuns < 1 {
		cfg.MaxRuns = 10
	}
	t.cfg = cfg
	t.solo, t.leader, t.member, t.isGroup = solo, leader, member, group
	t.running = true
	t.runCount = 0
	t.phase = TrialPhaseIdle
	t.stopChan = make(chan struct{})
	t.done = make(chan struct{})
	t.dungeonSeen = map[uint64]bool{}
	t.wrongWarned = map[uint64]bool{}
	t.logged = map[string]bool{}
	return true
}

// finish 루프 고루틴 마무리 (defer): 패닉이 나도 앱 전체가 죽지 않게 하고, 실행 상태를 내린 뒤
// (최대 횟수로 끝났으면 "완료" 유지) done 을 닫는다.
func (t *Trial) finish(msg string) {
	if r := recover(); r != nil {
		t.log(fmt.Sprintf("패닉 복구: %v", r))
	}
	t.mutex.Lock()
	t.running = false
	if t.phase != TrialPhaseDone {
		t.phase = TrialPhaseIdle
	}
	done := t.done
	t.mutex.Unlock()
	t.log(msg)
	close(done)
}

// Start 솔로 시련 시작 — 던전·횟수·목표 좌표는 먼저 SetConfig 로 정한다.
func (t *Trial) Start(w TrialWindow) {
	if !t.begin(false, w, TrialWindow{}, TrialWindow{}) {
		return
	}
	t.log(fmt.Sprintf("솔로 시련 시작 — 던전 %s, 최대 %d회, 목표 (%d,%d)",
		trialDungeonName(t.cfg.Dungeon), t.cfg.MaxRuns, t.cfg.TargetX, t.cfg.TargetY))
	t.log(" · " + trialWindowSummary("솔로", w, t.cfg.Dungeon))

	go t.runLoopSolo()
}

// StartGroup 그룹 시련 시작 — 던전·횟수·목표 좌표는 먼저 SetConfig 로 정한다.
func (t *Trial) StartGroup(leader, member TrialWindow) {
	if !t.begin(true, TrialWindow{}, leader, member) {
		return
	}
	t.log(fmt.Sprintf("그룹 시련 시작 — 던전 %s, 최대 %d회, 목표 (%d,%d)",
		trialDungeonName(t.cfg.Dungeon), t.cfg.MaxRuns, t.cfg.TargetX, t.cfg.TargetY))
	t.log(" · " + trialWindowSummary("그룹장", leader, t.cfg.Dungeon))
	t.log(" · " + trialWindowSummary("그룹원", member, t.cfg.Dungeon))

	go t.runLoopGroup()
}

// Stop 시련 중지
func (t *Trial) Stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.running {
		close(t.stopChan)
		t.running = false
		t.phase = TrialPhaseIdle
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

// ========== 입력/읽기 ==========
// 백그라운드 창은 절대 앞으로 가져오지 않는다(키 = PostMessage, 화면 = PrintWindow).
// 포그라운드 창은 키를 누를 때만 기존처럼 창을 띄우고 robotgo 로 누른다. 읽기는 둘 다 조용한 캡처 +
// 글리프(정확히 읽거나 못 읽음)가 먼저이고, 못 읽을 때만 OCR 로 보완한다.

// tap 입장/능력 선택 키 1회. 백그라운드는 PostMessage(창을 안 띄움), 포그라운드는 기존 pressKey
// (창 활성화 + 300ms + robotgo).
func (t *Trial) tap(w TrialWindow, key, label string) {
	if !w.BG {
		t.pressKey(w.HWND, key, label)
		return
	}
	if err := BgKeyTap(w.HWND, key); err != nil {
		t.log(fmt.Sprintf("[키입력] %s: '%s' 백그라운드 입력 실패: %v", label, key, err))
		return
	}
	t.log(fmt.Sprintf("[키입력] %s: '%s'", label, key))
}

// key 걷기·스킬 키 1회 (키마다 로그 없음). 백그라운드는 PostMessage — 실패는 키마다 한 번만 로그.
// 포그라운드는 robotgo (창은 호출한 쪽이 activate 로 먼저 띄운다).
func (t *Trial) key(w TrialWindow, k, role string) {
	if !w.BG {
		robotgo.KeyTap(k)
		return
	}
	if err := BgKeyTap(w.HWND, k); err != nil {
		t.logOnce(fmt.Sprintf("key-%d-%s", w.HWND, k),
			fmt.Sprintf("[입력] %s 백그라운드 키 '%s' 실패: %v", role, k, err))
	}
}

// activate 포그라운드 창을 앞으로 가져온다(이미 앞이면 그대로). 백그라운드 창은 절대 건드리지 않는다.
func (t *Trial) activate(w TrialWindow) {
	if w.BG || t.wm.GetForegroundWindow() == w.HWND {
		return
	}
	t.wm.ActivateWindow(w.HWND)
	time.Sleep(300 * time.Millisecond)
}

// readMap 창의 맵 이름. 조용한 캡처(PrintWindow) + 글리프 매칭이 먼저 — 정확한 이름이거나 못 읽음.
// 글리프로 못 읽으면(사전에 없는 글자 등) OCR 로 보완한다: 백그라운드는 같은 캡처의 상단 중앙을,
// 포그라운드는 기존 detectMap(창 활성화 + 화면 캡처)을 쓴다. 백그라운드 창은 캡처가 안 되면
// (맵 이동 암전·최소화 등) 창을 띄워서 읽지 않고 ""(못 읽음).
func (t *Trial) readMap(w TrialWindow) string {
	img, err := t.wm.CaptureWindowQuiet(w.HWND)
	if err == nil {
		if name, ok := t.om.ReadMapGlyph(img, w.HWND); ok {
			return name
		}
	} else {
		t.logOnce(fmt.Sprintf("cap-%d", w.HWND),
			fmt.Sprintf("[읽기] 창(0x%X) 화면을 조용히 못 읽음: %v (맵 이동 암전·최소화 등)", w.HWND, err))
	}
	if w.BG {
		if err != nil {
			return ""
		}
		return t.ocrMapName(w.HWND, img)
	}
	t.wm.ActivateWindow(w.HWND)
	time.Sleep(300 * time.Millisecond)
	return t.detectMap(w.HWND)
}

// readCoords 화면 우하단 HUD 좌표("X 034 Y 037")를 조용한 캡처로 읽는다 — 키도 창 활성화도 필요 없다.
// 포그라운드 창은 그게 안 되면 기존처럼 창을 앞으로 가져와 화면 캡처로 읽는다.
func (t *Trial) readCoords(w TrialWindow) (GameCoords, error) {
	img, err := t.wm.CaptureWindowQuiet(w.HWND)
	if err == nil {
		var c GameCoords
		if c, _, err = t.om.ReadCoordinatesFromImage(img); err == nil {
			return c, nil
		}
	}
	if w.BG {
		return GameCoords{}, err
	}
	t.wm.ActivateWindow(w.HWND)
	time.Sleep(500 * time.Millisecond)
	return t.om.ReadCoordinates(w.HWND)
}

// ========== 솔로 루프 ==========

func (t *Trial) runLoopSolo() {
	defer t.finish("솔로 시련 종료")

	t.log("초기 안정화 3초 대기...")
	if !t.sleep(3 * time.Second) {
		return
	}

	w := t.solo
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}
		if t.runCount >= t.cfg.MaxRuns {
			t.setPhase(TrialPhaseDone)
			t.log(fmt.Sprintf("최대 횟수 %d회 도달 — 종료", t.cfg.MaxRuns))
			return
		}
		t.newRound()

		t.log(fmt.Sprintf("===== 솔로 시련 %d/%d회 시작 =====", t.runCount+1, t.cfg.MaxRuns))

		// 1. 시련장 대기
		t.setPhase(TrialPhaseLobby)
		t.log("[1] 환상의시련장 대기 중...")
		if !t.waitForTrialLobby(w) {
			return
		}

		// 2. 입장: o → enter → enter (+ 첫 사이클만 enter 한 번 더)
		t.setPhase(TrialPhaseEnter)
		firstCycle := t.runCount == 0
		if firstCycle {
			t.log("[2] 입장 시퀀스 (첫 사이클): o → enter → enter → enter")
		} else {
			t.log("[2] 입장 시퀀스: o → enter → enter")
		}
		t.tap(w, "o", "솔로")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.tap(w, "enter", "솔로")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.tap(w, "enter", "솔로")
		if firstCycle {
			if !t.sleep(3 * time.Second) {
				return
			}
			t.tap(w, "enter", "솔로-첫사이클")
		}
		// 마무리 ESC (남은 NPC 대화창 닫기)
		if !t.sleep(800 * time.Millisecond) {
			return
		}
		t.tap(w, "escape", "솔로-마무리")

		// 3. 맵 전환 대기 (7초)
		t.setPhase(TrialPhaseMapWait)
		t.log("[3] 맵 전환 대기 7초...")
		if !t.sleep(7 * time.Second) {
			return
		}

		// 4+5. 던전에서 주기적 이동(+칸첸 스킬) + 시련장 복귀 감지
		t.setPhase(TrialPhaseBattle)
		t.log("[4] 전투 중 주기적 이동 + 복귀 감지 시작")
		t.battleLoopSolo()
		if t.isStopped() || !t.km.IsRunning() {
			return // 중지로 빠져나온 회차는 완료로 세지 않는다
		}

		t.incRun()
		t.log(fmt.Sprintf("===== 솔로 시련 %d회 완료 =====", t.runCount))

		if !t.sleep(3 * time.Second) {
			return
		}
	}
}

// ========== 그룹 루프 ==========

func (t *Trial) runLoopGroup() {
	defer t.finish("그룹 시련 종료")

	t.log("초기 안정화 3초 대기...")
	if !t.sleep(3 * time.Second) {
		return
	}

	leader, member := t.leader, t.member
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}
		if t.runCount >= t.cfg.MaxRuns {
			t.setPhase(TrialPhaseDone)
			t.log(fmt.Sprintf("최대 횟수 %d회 도달 — 종료", t.cfg.MaxRuns))
			return
		}
		t.newRound()

		t.log(fmt.Sprintf("===== 그룹 시련 %d/%d회 시작 =====", t.runCount+1, t.cfg.MaxRuns))

		// 1. 시련장 대기 (그룹장 기준)
		t.setPhase(TrialPhaseLobby)
		t.log("[1] 환상의시련장 대기 중...")
		if !t.waitForTrialLobby(leader) {
			return
		}

		// 2. 그룹장 입장: o → enter → enter (+ 첫 사이클만 enter 한 번 더)
		t.setPhase(TrialPhaseEnter)
		firstCycle := t.runCount == 0
		if firstCycle {
			t.log("[2] 그룹장 입장 (첫 사이클): o → enter → enter → enter")
		} else {
			t.log("[2] 그룹장 입장: o → enter → enter")
		}
		t.tap(leader, "o", "그룹장")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.tap(leader, "enter", "그룹장")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.tap(leader, "enter", "그룹장")
		if firstCycle {
			if !t.sleep(3 * time.Second) {
				return
			}
			t.tap(leader, "enter", "그룹장-첫사이클")
		}
		// 그룹장 마무리 ESC (남은 NPC 대화창 닫기)
		if !t.sleep(800 * time.Millisecond) {
			return
		}
		t.tap(leader, "escape", "그룹장-마무리")

		// 3. 그룹원 5초 대기 → enter (+ 첫 사이클만 enter 한 번 더)
		if firstCycle {
			t.log("[3] 그룹원 (첫 사이클) 5초 대기 → enter → 3초 → enter")
		} else {
			t.log("[3] 그룹원 5초 대기 → enter")
		}
		if !t.sleep(5 * time.Second) {
			return
		}
		t.tap(member, "enter", "그룹원")
		if firstCycle {
			if !t.sleep(3 * time.Second) {
				return
			}
			t.tap(member, "enter", "그룹원-첫사이클")
		}
		// 그룹원 마무리 ESC (남은 NPC 대화창 닫기)
		if !t.sleep(800 * time.Millisecond) {
			return
		}
		t.tap(member, "escape", "그룹원-마무리")

		// 4. 맵 전환 대기 (7초)
		t.setPhase(TrialPhaseMapWait)
		t.log("[4] 맵 전환 대기 7초...")
		if !t.sleep(7 * time.Second) {
			return
		}

		// 5+6. 던전에서 주기적 이동(+칸첸 스킬) + 시련장 복귀 감지
		t.setPhase(TrialPhaseBattle)
		t.log("[5] 전투 중 주기적 이동 + 복귀 감지 시작")
		t.battleLoopGroup()
		if t.isStopped() || !t.km.IsRunning() {
			return // 중지로 빠져나온 회차는 완료로 세지 않는다
		}

		// 7. 능력 선택 — 그룹장
		t.setPhase(TrialPhaseAbility)
		t.log("[7] 그룹장 능력 선택: o → enter")
		if !t.sleep(2 * time.Second) {
			return
		}
		t.tap(leader, "o", "그룹장-능력")
		if !t.sleep(t.randomDelay()) {
			return
		}
		t.tap(leader, "enter", "그룹장-능력")

		// 8. 그룹원 능력 선택: 3초 대기 → enter
		t.log("[8] 그룹원 능력 선택: 3초 대기 → enter")
		if !t.sleep(3 * time.Second) {
			return
		}
		t.tap(member, "enter", "그룹원-능력")

		t.incRun()
		t.log(fmt.Sprintf("===== 그룹 시련 %d회 완료 =====", t.runCount))

		if !t.sleep(3 * time.Second) {
			return
		}
	}
}

// ========== 공통 유틸 ==========

// detectMap 맵 이름 OCR 감지 (포그라운드 보완용 — 창을 앞으로 가져와 화면 캡처)
func (t *Trial) detectMap(hwnd uint64) string {
	rawImg, _, err := t.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		t.log(fmt.Sprintf("[맵OCR] 캡처 실패: %v", err))
		return ""
	}
	return t.ocrMapName(hwnd, rawImg)
}

// ocrMapName 창 이미지 상단 중앙(맵 이름 자리)을 OCR — 글리프로 못 읽었을 때의 보완
func (t *Trial) ocrMapName(hwnd uint64, rawImg *image.RGBA) string {
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

// walkTo 목표 좌표로 걸어서 이동 (방향키 1칸씩, 칸마다 300~500ms).
// 좌표는 화면 우하단 HUD 에 늘 떠 있어서 조용한 캡처로 먼저 읽는다 — 목표 근처면 창을 건드리지 않는다.
func (t *Trial) walkTo(w TrialWindow, role string) {
	targetX := t.cfg.TargetX
	targetY := t.cfg.TargetY
	if targetX == 0 && targetY == 0 {
		t.log("[걷기] 목표 좌표 미설정 — 스킵")
		return
	}

	// 좌표 읽기 (최대 3회 시도)
	var coords GameCoords
	var readErr error
	for retry := 0; retry < 3; retry++ {
		coords, readErr = t.readCoords(w)
		if readErr == nil && (coords.X > 0 || coords.Y > 0) {
			break
		}
		t.log(fmt.Sprintf("[걷기] %s 좌표 읽기 시도 %d/3 실패: %v (좌표=%d,%d)", role, retry+1, readErr, coords.X, coords.Y))
		if !t.sleep(2 * time.Second) {
			return
		}
	}
	if readErr != nil || (coords.X == 0 && coords.Y == 0) {
		t.log(fmt.Sprintf("[걷기] %s 좌표 읽기 최종 실패 — 이동 포기", role))
		return
	}

	diffX := targetX - coords.X
	diffY := targetY - coords.Y
	if meAbs(diffX) <= 1 && meAbs(diffY) <= 1 {
		// 목표 근처 — 로그 생략 (2초마다 호출되므로)
		return
	}

	t.log(fmt.Sprintf("[걷기] %s (%d,%d) → (%d,%d) diff=(%+d,%+d)", role, coords.X, coords.Y, targetX, targetY, diffX, diffY))

	t.activate(w) // 포그라운드만 한 번 앞으로 (백그라운드는 그대로)
	if !t.walkAxis(w, role, diffX, "right", "left") || !t.walkAxis(w, role, diffY, "down", "up") {
		return
	}
	t.log("[걷기] 이동 완료")
}

// walkAxis 한 축으로 |diff| 칸 걷기 (diff>0 → pos 방향, diff<0 → neg 방향). 중지되면 false.
func (t *Trial) walkAxis(w TrialWindow, role string, diff int, pos, neg string) bool {
	if diff == 0 {
		return true
	}
	dir, count := pos, diff
	if diff < 0 {
		dir, count = neg, -diff
	}
	for i := 0; i < count; i++ {
		if t.isStopped() {
			return false
		}
		t.key(w, dir, role)
		time.Sleep(time.Duration(300+rand.Intn(200)) * time.Millisecond)
	}
	t.log(fmt.Sprintf("[걷기] %s %d칸", dir, count))
	return true
}

// pressSkills 칸첸 던전에 있는 창에 그 캐릭터의 스킬 키를 한 번씩 누른다 (키 사이 300ms).
// 2초마다 누르므로 키마다 로그는 남기지 않는다. 포그라운드는 창을 띄운 뒤 robotgo, 백그라운드는 PostMessage.
func (t *Trial) pressSkills(w TrialWindow, role string) {
	if len(w.Skills) == 0 {
		return
	}
	t.activate(w)
	for i, k := range w.Skills {
		if i > 0 && !t.sleep(trialSkillGap) {
			return
		}
		t.key(w, k, role)
	}
}

// battleTick 창 1개 점검: 맵을 읽어 고른 던전이면 목표 좌표로 걷고(칸첸은 스킬도 누름) 위치를 돌려준다.
// 시련장이면 아무것도 안 한다(회차 끝 판단은 호출한 쪽). 다른 던전이면 회차마다 한 번 경고만 하고
// 그 창은 건드리지 않는다. 맵을 못 읽으면 이번엔 건너뛴다.
func (t *Trial) battleTick(w TrialWindow, role string) (place, mapName string) {
	mapName = t.readMap(w)
	place = TrialPlaceOfMap(mapName)
	switch {
	case place == TrialPlaceLobby:
	case place == t.cfg.Dungeon:
		if !t.dungeonSeen[w.HWND] {
			t.dungeonSeen[w.HWND] = true
			t.log(fmt.Sprintf("[전투] %s: %s 던전 확인 ('%s')", role, trialDungeonName(place), mapName))
		}
		t.walkTo(w, role)
		if place == TrialDungeonKanchen {
			t.pressSkills(w, role)
		}
	case place != "":
		if !t.wrongWarned[w.HWND] {
			t.wrongWarned[w.HWND] = true
			t.log(fmt.Sprintf("⚠ [전투] %s: 선택한 던전(%s)과 다른 맵 '%s' — 게임 NPC에서 '시련 던전 변경'으로 바꿔주세요 (이 창은 이동·스킬 안 함)",
				role, trialDungeonName(t.cfg.Dungeon), mapName))
		}
	default:
		t.log(fmt.Sprintf("[전투] %s 맵='%s' — 던전 미확인, 이동 스킵", role, mapName))
	}
	return place, mapName
}

// battleLoopSolo 솔로 전투 루프: 2초마다 맵 확인 → 던전이면 목표 좌표로 이동(+칸첸 스킬), 시련장 복귀 감지
func (t *Trial) battleLoopSolo() {
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}

		if place, _ := t.battleTick(t.solo, "솔로"); place == TrialPlaceLobby {
			t.log("[전투] 환상의시련장 복귀 감지!")
			return
		}

		if !t.sleep(2 * time.Second) {
			return
		}
	}
}

// battleLoopGroup 그룹 전투 루프: 2초마다 그룹장/그룹원 맵을 각각 읽어 던전에 있는 창만 이동(+칸첸 스킬).
// 시련장에 있는 창은 절대 이동하지 않는다 (이동은 던전에서만). 그룹장이 시련장으로 돌아오면 회차 끝.
func (t *Trial) battleLoopGroup() {
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}

		// === 그룹장 ===
		if place, _ := t.battleTick(t.leader, "그룹장"); place == TrialPlaceLobby {
			t.log("[전투] 그룹장 환상의시련장 복귀 감지!")
			return
		}

		// === 그룹원 (맵은 따로 읽는다) ===
		if place, memberMap := t.battleTick(t.member, "그룹원"); place == TrialPlaceLobby {
			t.log(fmt.Sprintf("[전투] 그룹원 시련장(%s) — 이동 안 함", memberMap))
		}

		if !t.sleep(2 * time.Second) {
			return
		}
	}
}

// waitForTrialLobby 시련장에 있을 때까지 대기 (맵 인식 실패에도 견고하도록)
//   - 30회 시도(약 90초) 이상 시련장이 아니면 경고 로그
//   - 맵을 연속 5회 못 읽으면(글리프·OCR 모두 실패) 캐릭터 위치가 시련장이라고 가정하고 진입
//     (사용자가 의도적으로 시련장에 두고 시작했을 가능성 + 이름을 못 잡는 경우 대응)
func (t *Trial) waitForTrialLobby(w TrialWindow) bool {
	attempts := 0
	emptyStreak := 0
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return false
		}
		mapName := t.readMap(w)
		attempts++

		if TrialPlaceOfMap(mapName) == TrialPlaceLobby {
			t.log(fmt.Sprintf("[대기] 환상의시련장 확인! (시도 %d회)", attempts))
			return true
		}

		if mapName == "" {
			emptyStreak++
			t.log(fmt.Sprintf("[대기] 맵 인식 실패 (%d회 연속)", emptyStreak))
			// 5회 연속 못 읽으면 사용자가 시련장에 두고 시작했다고 가정하고 진입
			if emptyStreak >= 5 {
				t.log("[대기] 맵 인식 5회 연속 실패 — 시련장으로 가정하고 진입")
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
func (t *Trial) waitForLobby(w TrialWindow) {
	for {
		if t.isStopped() || !t.km.IsRunning() {
			return
		}
		mapName := t.readMap(w)
		if TrialPlaceOfMap(mapName) == TrialPlaceLobby {
			t.log("[대기] 환상의시련장 복귀 감지!")
			return
		}
		t.log(fmt.Sprintf("[대기] 아직 전투 중... 맵='%s' (2초 후 재확인)", mapName))
		if !t.sleep(2 * time.Second) {
			return
		}
	}
}

// trialDungeonName 로그용 던전 이름
func trialDungeonName(d string) string {
	if d == TrialDungeonKanchen {
		return "칸첸"
	}
	return "대야"
}

// trialWindowSummary 시작 로그용 창 요약: 역할, 입력 방식, 캐릭터 이름, 스킬 키
func trialWindowSummary(role string, w TrialWindow, dungeon string) string {
	input := "포그라운드"
	if w.BG {
		input = "백그라운드"
	}
	nick := "캐릭터 이름 못 읽음"
	if w.Nick != "" {
		nick = fmt.Sprintf("캐릭터 '%s'", w.Nick)
	}
	var skills string
	switch {
	case dungeon != TrialDungeonKanchen:
		skills = "스킬 안 누름(대야)"
	case len(w.Skills) == 0:
		skills = "스킬 키 없음 — 스킬은 안 누름"
	default:
		skills = fmt.Sprintf("스킬 %s — 칸첸 던전에 있는 동안 약 2초마다", strings.Join(w.Skills, ","))
	}
	return fmt.Sprintf("%s(hwnd=0x%X): %s, %s, %s", role, w.HWND, input, nick, skills)
}

// ========== 위치 판별 / 설정 정리 (순수 함수) ==========

// TrialPlaceOfMap 맵 이름 → 시련 위치: "lobby"(환상의시련장) | "kanchen" | "daeya" | ""(그 외·못 읽음).
// 글리프로 읽은 정확한 이름("환상의시련장", "[환상]칸첸중가설산", "[환상]대야전투")은 물론 OCR 보완 값
// (한글만 남거나 글자가 틀림)도 받는다 — 시련장은 예전 isTrialLobby 의 너그러운 매칭을 그대로 쓴다.
// 던전은 "환상"이 붙어야 한다: 메인화면 사냥맵 "칸첸중가설산"/"대야전투"는 시련 던전이 아니다("").
func TrialPlaceOfMap(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.Contains(name, "환상") {
		switch {
		case strings.Contains(name, "칸첸") || strings.Contains(name, "설산"):
			return TrialDungeonKanchen
		case strings.Contains(name, "대야") || strings.Contains(name, "전투"):
			return TrialDungeonDaeya
		}
	}
	if isTrialLobbyName(name) {
		return TrialPlaceLobby
	}
	return ""
}

// isTrialLobbyName "환상의시련장" 매칭 (OCR 부정확 대응 강화 — 글리프로 읽은 정확한 이름은 그대로 통과)
// 1) 키워드: "시련", "련장", "환상의", "환상시" 등 하나만 들어가도 true
// 2) Levenshtein 60% 허용 (6글자 → 최대 3글자 차이까지 허용)
// 3) 시련장은 "전투" 단어가 절대 안 들어가므로 "전투" 포함이면 false
func isTrialLobbyName(mapName string) bool {
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

// NormalizeTrialDungeon 시련 던전 값 정리 — "kanchen" 이 아니면 전부 "daeya"(예전 시련).
func NormalizeTrialDungeon(d string) string {
	if strings.EqualFold(strings.TrimSpace(d), TrialDungeonKanchen) {
		return TrialDungeonKanchen
	}
	return TrialDungeonDaeya
}

// NormalizeTrialMaxRuns 반복 횟수 정리 — 1~99 밖이면 10.
func NormalizeTrialMaxRuns(n int) int {
	if n < 1 || n > 99 {
		return 10
	}
	return n
}

// TrialDaeyaTarget 대야 시련 목표 좌표 — 0(미설정)이면 기본 29,32.
func TrialDaeyaTarget(x, y int) (int, int) {
	return trialCoordOr(x, 29), trialCoordOr(y, 32)
}

// TrialKanchenTarget 칸첸 시련 목표 좌표 = 메인화면 칸첸 "사냥 자리"(아이템 습득 원점). 0이면 34,37.
func TrialKanchenTarget(originX, originY int) (int, int) {
	return trialCoordOr(originX, 34), trialCoordOr(originY, 37)
}

func trialCoordOr(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// TrialSkillKeysFor 칸첸 시련에서 이 캐릭터가 누를 스킬 키 — 시련 전용 표에서 캐릭터별 키가 있으면 그것,
// 없거나 이름을 못 읽었으면("") 기본 키. 메인화면 칸첸 표(ItemScannerConfig.SkillKeysFor)와 같은 규칙.
func TrialSkillKeysFor(keys []string, byChar map[string][]string, name string) []string {
	if name = strings.TrimSpace(name); name != "" {
		if k := byChar[name]; len(k) > 0 {
			return k
		}
	}
	return keys
}
