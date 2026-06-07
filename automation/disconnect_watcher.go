package automation

import (
	"fmt"
	"image"
	"log"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
)

// DisconnectWatcher 게임 연결 끊김 감지 및 자동 재접속 처리
//
// 1분마다 게임창을 캡처해 다음을 검사:
//  1. config/disconnect_reconnect.png (Reconnect 버튼) 매칭 → 매칭 위치 클릭
//  2. config/disconnect_charsel.png (캐릭터 선택 화면) 매칭 → Enter 키 입력
//  3. 두 단계 모두 처리되면 30초 대기 후 onReconnected 콜백 호출 (자동사냥 재시작용)
//
// 해상도 차이에도 대응하기 위해 FindImageMultiScale 사용.
type DisconnectWatcher struct {
	wm *WindowManager
	ma *MouseAutomation

	mu              sync.Mutex
	running         bool
	stopChan        chan struct{}
	hwnds           []uint64
	onReconnected   func(hwnd uint64) // 재접속 후 호출 (예: StartHunting 재실행)
	logFn           func(string)
	checkInterval   time.Duration

	// needle 캐시
	reconnectNeedle image.Image
	charSelNeedle   image.Image
	needlesLoaded   bool
}

// NewDisconnectWatcher 새 워처 생성
func NewDisconnectWatcher(wm *WindowManager, ma *MouseAutomation) *DisconnectWatcher {
	return &DisconnectWatcher{
		wm:            wm,
		ma:            ma,
		checkInterval: 60 * time.Second,
	}
}

// SetLogFunc 로그 콜백 설정
func (dw *DisconnectWatcher) SetLogFunc(f func(string)) {
	dw.logFn = f
}

// SetOnReconnected 재접속 완료 후 호출되는 콜백 (예: 자동사냥 재시작)
func (dw *DisconnectWatcher) SetOnReconnected(f func(hwnd uint64)) {
	dw.onReconnected = f
}

func (dw *DisconnectWatcher) log(msg string) {
	log.Printf("[팅김감지] %s", msg)
	if dw.logFn != nil {
		dw.logFn(msg)
	}
}

// loadNeedles config/disconnect_*.png 로드 (한 번만)
func (dw *DisconnectWatcher) loadNeedles() error {
	if dw.needlesLoaded {
		return nil
	}
	rc, err := LoadPNG("config/disconnect_reconnect.png")
	if err != nil {
		return fmt.Errorf("disconnect_reconnect.png 로드 실패: %v", err)
	}
	cs, err := LoadPNG("config/disconnect_charsel.png")
	if err != nil {
		return fmt.Errorf("disconnect_charsel.png 로드 실패: %v", err)
	}
	dw.reconnectNeedle = rc
	dw.charSelNeedle = cs
	dw.needlesLoaded = true
	dw.log(fmt.Sprintf("needle 로드 완료: reconnect=%dx%d, charsel=%dx%d",
		rc.Bounds().Dx(), rc.Bounds().Dy(), cs.Bounds().Dx(), cs.Bounds().Dy()))
	return nil
}

// Start 워처 시작 (감시 대상 hwnd 목록)
func (dw *DisconnectWatcher) Start(hwnds []uint64) {
	dw.mu.Lock()
	if dw.running {
		dw.mu.Unlock()
		return
	}
	if err := dw.loadNeedles(); err != nil {
		dw.mu.Unlock()
		dw.log(fmt.Sprintf("워처 시작 실패: %v", err))
		return
	}
	dw.running = true
	dw.hwnds = append([]uint64{}, hwnds...)
	dw.stopChan = make(chan struct{})
	stopChan := dw.stopChan
	dw.mu.Unlock()

	dw.log(fmt.Sprintf("팅김 감지 시작 (대상 %d개 창, %s 주기)",
		len(hwnds), dw.checkInterval))
	go dw.run(stopChan)
}

// Stop 워처 중지
func (dw *DisconnectWatcher) Stop() {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	if !dw.running {
		return
	}
	close(dw.stopChan)
	dw.running = false
	dw.log("팅김 감지 중지")
}

// IsRunning 실행 중 여부
func (dw *DisconnectWatcher) IsRunning() bool {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	return dw.running
}

func (dw *DisconnectWatcher) run(stopChan chan struct{}) {
	// 첫 검사는 30초 후 (시작 직후 게임 안정화 시간)
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-timer.C:
			dw.checkAll()
			timer.Reset(dw.checkInterval)
		}
	}
}

// checkAll 등록된 모든 hwnd에 대해 팅김 검사
func (dw *DisconnectWatcher) checkAll() {
	dw.mu.Lock()
	hwnds := append([]uint64{}, dw.hwnds...)
	dw.mu.Unlock()

	for _, hwnd := range hwnds {
		if !dw.IsRunning() {
			return
		}
		dw.checkOne(hwnd)
	}
}

// checkOne 단일 hwnd 검사 + 재접속 처리
func (dw *DisconnectWatcher) checkOne(hwnd uint64) {
	raw, _, err := dw.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		dw.log(fmt.Sprintf("hwnd=0x%X 캡처 실패: %v", hwnd, err))
		return
	}

	// 1. Reconnect 다이얼로그 매칭
	if x, y, scale, found := FindImageMultiScale(raw, dw.reconnectNeedle, nil, 60, 0.85); found {
		nb := dw.reconnectNeedle.Bounds()
		cx := x + int(float64(nb.Dx())*scale)/2
		cy := y + int(float64(nb.Dy())*scale)/2
		dw.log(fmt.Sprintf("hwnd=0x%X Reconnect 발견 (scale=%.2f) → 클릭 (%d,%d)", hwnd, scale, cx, cy))
		dw.wm.ActivateWindow(hwnd)
		time.Sleep(500 * time.Millisecond)
		if err := dw.ma.ClickRelative(hwnd, cx, cy); err != nil {
			dw.log(fmt.Sprintf("Reconnect 클릭 실패: %v", err))
			return
		}
		// 캐릭터 선택 화면 진입까지 대기
		dw.waitForCharSelect(hwnd, 30*time.Second)
		return
	}

	// 2. 캐릭터 선택 화면 매칭 (다이얼로그 없이 캐릭터 선택만 떠있는 경우)
	if _, _, scale, found := FindImageMultiScale(raw, dw.charSelNeedle, nil, 60, 0.85); found {
		dw.log(fmt.Sprintf("hwnd=0x%X 캐릭터 선택 화면 (scale=%.2f) → Enter", hwnd, scale))
		dw.handleCharSelect(hwnd)
		return
	}
}

// waitForCharSelect Reconnect 클릭 후 캐릭터 선택 화면 진입 대기
func (dw *DisconnectWatcher) waitForCharSelect(hwnd uint64, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !dw.IsRunning() {
			return
		}
		raw, _, err := dw.wm.CaptureWindowRaw(hwnd)
		if err == nil {
			if _, _, _, found := FindImageMultiScale(raw, dw.charSelNeedle, nil, 60, 0.85); found {
				dw.handleCharSelect(hwnd)
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	dw.log("캐릭터 선택 화면 진입 시간 초과")
}

// handleCharSelect 캐릭터 선택 화면 → Enter → 30초 대기 → 자동사냥 재시작
func (dw *DisconnectWatcher) handleCharSelect(hwnd uint64) {
	dw.wm.ActivateWindow(hwnd)
	time.Sleep(500 * time.Millisecond)
	robotgo.KeyTap("enter")
	dw.log("Enter 입력 — 게임 시작 진행")

	// 게임 진입 대기 30초
	dw.log("게임 진입 대기 30초...")
	for i := 0; i < 30; i++ {
		if !dw.IsRunning() {
			return
		}
		time.Sleep(1 * time.Second)
	}

	// 재접속 콜백 (자동사냥 재시작)
	if dw.onReconnected != nil {
		dw.log(fmt.Sprintf("재접속 완료 → onReconnected 호출 (hwnd=0x%X)", hwnd))
		dw.onReconnected(hwnd)
	}
}
