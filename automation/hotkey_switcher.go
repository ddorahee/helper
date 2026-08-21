package automation

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/go-vgo/robotgo"
)

// WindowSwitcher 전역 핫키로 창을 슬롯에 기억하고 그 창으로 전환하는 기능.
//   이동: Shift+F1, Shift+F2, Shift+F3, Shift+F4, Alt+5  → 슬롯의 창으로 활성화
//   등록: Ctrl+Shift+1~5                                  → 현재 맨 앞 창을 슬롯에 기억
// 바람의나라뿐 아니라 카카오톡 등 임의의 창을 대상으로 한다.
// 추가: 슬롯마다 "복사 텍스트"를 둘 수 있어, 그 창으로 전환할 때 클립보드에 복사한다
// (붙여넣기는 사용자가 직접). HWND/텍스트 모두 앱 실행 중에만 유지(영속화 X).
//
// gohook(키매핑)과 충돌하지 않도록 Win32 RegisterHotKey를 사용한다. OS가 modifier
// 조합을 정확히 구분해주고, 전용 OS 스레드의 메시지 루프(WM_HOTKEY)로 받는다.
const SwitcherSlots = 5

// 이동 핫키 조합 (슬롯1~5). 등록은 Ctrl+Shift+숫자로 별도.
// F키(0x70~0x73)는 글자 입력에 안 쓰여 Shift+숫자처럼 특수문자 입력을 막지 않는다.
var activateHK = [SwitcherSlots]struct {
	mod, vk uintptr
	label   string
}{
	{modShift, 0x70, "Shift+F1"}, // VK_F1
	{modShift, 0x71, "Shift+F2"},
	{modShift, 0x72, "Shift+F3"},
	{modShift, 0x73, "Shift+F4"},
	{modAlt, 0x35, "Alt+5"}, // vk '5'
}

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modNoRepeat = 0x4000 // 누르고 있어도 1회만

	wmHotkey = 0x0312
	wmQuit   = 0x0012

	// 핫키 ID: 이동 = 1..5, 등록 = 11..15
	hkActivateBase = 1
	hkRegisterBase = 11
)

var (
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")

	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentThread = kernel32.NewProc("GetCurrentThreadId")
)

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

// SwitcherSlot UI 표시용 슬롯 상태
type SwitcherSlot struct {
	Slot     int    `json:"slot"`     // 1~5
	HWND     uint64 `json:"hwnd"`     // 0 = 비어있음
	Title    string `json:"title"`    // 등록 당시 창 제목
	Valid    bool   `json:"valid"`    // 창이 아직 살아있는지
	Hotkey   string `json:"hotkey"`   // 이동 핫키 표기(Shift+1 등)
	ClipText string `json:"clipText"` // 전환 시 클립보드에 복사할 텍스트
}

// WindowSwitcher 창 전환 매니저
type WindowSwitcher struct {
	wm       *WindowManager
	mu       sync.Mutex
	slots    [SwitcherSlots]uint64 // HWND, 0 = 빈 슬롯
	titles   [SwitcherSlots]string
	clipText [SwitcherSlots]string // 전환 시 클립보드에 복사할 텍스트(비면 복사 안 함)
	running  bool
	threadID uint32 // 메시지 루프 스레드 ID(종료 신호용)
	selfHWND uint64 // helper 자기 창 — 등록에서 제외

	logFunc  func(string)
	onChange func() // 슬롯 변경 시 UI 갱신용 콜백
}

// NewWindowSwitcher 생성
func NewWindowSwitcher(wm *WindowManager) *WindowSwitcher {
	return &WindowSwitcher{wm: wm}
}

func (ws *WindowSwitcher) SetLogFunc(f func(string)) { ws.logFunc = f }
func (ws *WindowSwitcher) SetOnChange(f func())       { ws.onChange = f }

// SetSelfHWND helper 자기 창 HWND 설정 — 등록 시 자기 자신은 제외(실수 방지).
func (ws *WindowSwitcher) SetSelfHWND(hwnd uint64) {
	ws.mu.Lock()
	ws.selfHWND = hwnd
	ws.mu.Unlock()
}

func (ws *WindowSwitcher) emit(format string, a ...interface{}) {
	m := fmt.Sprintf(format, a...)
	log.Printf("[창전환] %s", m)
	if ws.logFunc != nil {
		ws.logFunc(m)
	}
}

// Start 전역 핫키 등록 + 메시지 루프 시작(전용 OS 스레드). 중복 호출 안전.
func (ws *WindowSwitcher) Start() {
	ws.mu.Lock()
	if ws.running {
		ws.mu.Unlock()
		return
	}
	ws.running = true
	ws.mu.Unlock()

	go ws.messageLoop()
}

// Stop 메시지 루프 종료 + 핫키 해제.
func (ws *WindowSwitcher) Stop() {
	ws.mu.Lock()
	if !ws.running {
		ws.mu.Unlock()
		return
	}
	ws.running = false
	tid := ws.threadID
	ws.mu.Unlock()

	if tid != 0 {
		// 메시지 루프 스레드에 WM_QUIT을 보내 GetMessage를 깨운다
		procPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0)
	}
}

// messageLoop RegisterHotKey는 등록한 스레드의 큐로 WM_HOTKEY를 보내므로,
// 등록과 GetMessage 루프가 반드시 같은 OS 스레드여야 한다(LockOSThread).
func (ws *WindowSwitcher) messageLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		if r := recover(); r != nil {
			ws.emit("패닉 복구: %v", r)
		}
		ws.mu.Lock()
		ws.running = false
		ws.mu.Unlock()
	}()

	tid, _, _ := procGetCurrentThread.Call()
	ws.mu.Lock()
	ws.threadID = uint32(tid)
	ws.mu.Unlock()

	// 이동: activateHK 테이블(Shift+1~4, Alt+1) / 등록: Ctrl+Shift+1~5. vk '1'..'5'=0x31..0x35.
	registered := 0
	for i := 0; i < SwitcherSlots; i++ {
		if r, _, _ := procRegisterHotKey.Call(0, uintptr(hkActivateBase+i), activateHK[i].mod|modNoRepeat, activateHK[i].vk); r != 0 {
			registered++
		} else {
			ws.emit("%s 등록 실패(다른 앱이 선점 중일 수 있음)", activateHK[i].label)
		}
		if r, _, _ := procRegisterHotKey.Call(0, uintptr(hkRegisterBase+i), modControl|modShift|modNoRepeat, uintptr(0x31+i)); r == 0 {
			ws.emit("Ctrl+Shift+%d 등록 실패", i+1)
		}
	}
	defer ws.unregisterAll()
	ws.emit("창 전환 활성화 — 등록: Ctrl+Shift+1~5 / 이동: Shift+F1~F4·Alt+5 (핫키 %d/5)", registered)

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		// GetMessage: 0 = WM_QUIT, ^uintptr(0)(=-1) = 에러
		if ret == 0 || ret == ^uintptr(0) {
			return
		}
		if m.Message == wmHotkey {
			ws.handleHotkey(int(m.WParam))
		}
	}
}

func (ws *WindowSwitcher) unregisterAll() {
	for i := 0; i < SwitcherSlots; i++ {
		procUnregisterHotKey.Call(0, uintptr(hkActivateBase+i))
		procUnregisterHotKey.Call(0, uintptr(hkRegisterBase+i))
	}
}

// handleHotkey WM_HOTKEY id → 등록/이동 분기
func (ws *WindowSwitcher) handleHotkey(id int) {
	switch {
	case id >= hkRegisterBase && id < hkRegisterBase+SwitcherSlots:
		ws.register(id - hkRegisterBase) // 0-based slot
	case id >= hkActivateBase && id < hkActivateBase+SwitcherSlots:
		ws.activate(id - hkActivateBase)
	}
}

// register 현재 맨 앞 창을 슬롯(0-based)에 기억. helper 자기 창은 제외.
func (ws *WindowSwitcher) register(slot int) {
	hwnd := ws.wm.GetForegroundWindow()
	if hwnd == 0 {
		ws.emit("등록 실패 — 활성 창 없음")
		return
	}
	ws.mu.Lock()
	self := ws.selfHWND
	ws.mu.Unlock()
	if hwnd == self {
		ws.emit("등록 무시 — helper 자기 창(다른 창을 앞에 두고 누르세요)")
		return
	}
	title := ws.wm.GetWindowTitle(hwnd)

	ws.mu.Lock()
	ws.slots[slot] = hwnd
	ws.titles[slot] = title
	ws.mu.Unlock()

	ws.emit("슬롯 %d 기억: %q (hwnd=%d)", slot+1, title, hwnd)
	if ws.onChange != nil {
		ws.onChange()
	}
}

// activate 슬롯(0-based) 창으로 전환. 비었거나 죽은 창이면 안내만.
func (ws *WindowSwitcher) activate(slot int) {
	ws.mu.Lock()
	hwnd := ws.slots[slot]
	title := ws.titles[slot]
	clip := ws.clipText[slot]
	ws.mu.Unlock()

	if hwnd == 0 {
		ws.emit("슬롯 %d 비어있음 — Ctrl+Shift+%d로 먼저 등록하세요", slot+1, slot+1)
		return
	}
	if !ws.wm.IsWindowValid(hwnd) {
		ws.emit("슬롯 %d 창이 닫혔습니다(%q) — 다시 등록 필요", slot+1, title)
		ws.mu.Lock()
		ws.slots[slot] = 0
		ws.titles[slot] = ""
		ws.mu.Unlock()
		if ws.onChange != nil {
			ws.onChange()
		}
		return
	}
	if err := ws.wm.ActivateWindow(hwnd); err != nil {
		ws.emit("슬롯 %d 전환 실패: %v", slot+1, err)
		return
	}
	// 전환 시 슬롯 텍스트를 클립보드에 복사(붙여넣기는 사용자가 직접 Ctrl+V).
	if clip != "" {
		if err := robotgo.WriteAll(clip); err != nil {
			ws.emit("슬롯 %d 전환: %q (클립보드 복사 실패: %v)", slot+1, title, err)
			return
		}
		ws.emit("슬롯 %d 전환: %q (클립보드: %q)", slot+1, title, clipPreview(clip))
		return
	}
	ws.emit("슬롯 %d 전환: %q", slot+1, title)
}

// clipPreview 로그용 짧은 미리보기.
func clipPreview(s string) string {
	r := []rune(s)
	if len(r) > 20 {
		return string(r[:20]) + "…"
	}
	return s
}

// SetText 슬롯(1-based)의 전환 시 복사 텍스트 설정.
func (ws *WindowSwitcher) SetText(slot1Based int, text string) error {
	if slot1Based < 1 || slot1Based > SwitcherSlots {
		return fmt.Errorf("슬롯 범위 오류: %d", slot1Based)
	}
	ws.mu.Lock()
	ws.clipText[slot1Based-1] = text
	ws.mu.Unlock()
	if ws.onChange != nil {
		ws.onChange()
	}
	return nil
}

// RegisterCurrent UI '등록' 버튼용 — 현재 맨 앞 창을 슬롯(1-based)에 기억.
func (ws *WindowSwitcher) RegisterCurrent(slot1Based int) error {
	if slot1Based < 1 || slot1Based > SwitcherSlots {
		return fmt.Errorf("슬롯 범위 오류: %d", slot1Based)
	}
	ws.register(slot1Based - 1)
	return nil
}

// Clear UI '비우기' 버튼용 — 슬롯(1-based) 비움.
func (ws *WindowSwitcher) Clear(slot1Based int) error {
	if slot1Based < 1 || slot1Based > SwitcherSlots {
		return fmt.Errorf("슬롯 범위 오류: %d", slot1Based)
	}
	ws.mu.Lock()
	ws.slots[slot1Based-1] = 0
	ws.titles[slot1Based-1] = ""
	ws.mu.Unlock()
	if ws.onChange != nil {
		ws.onChange()
	}
	return nil
}

// GetSlots UI 표시용 슬롯 상태 목록(1-based, 유효성 포함).
func (ws *WindowSwitcher) GetSlots() []SwitcherSlot {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	out := make([]SwitcherSlot, SwitcherSlots)
	for i := 0; i < SwitcherSlots; i++ {
		out[i] = SwitcherSlot{
			Slot:     i + 1,
			HWND:     ws.slots[i],
			Title:    ws.titles[i],
			Valid:    ws.slots[i] != 0 && ws.wm.IsWindowValid(ws.slots[i]),
			Hotkey:   activateHK[i].label,
			ClipText: ws.clipText[i],
		}
	}
	return out
}
