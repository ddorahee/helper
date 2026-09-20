package automation

import (
	"fmt"
	"image"
	"strings"
	"time"
	"unsafe"
)

// 백그라운드(비활성 창) 입력/캡처. baram-yolo 프로젝트에서 이식 — 거기서 2창 동시 운영으로
// 검증된 구현이다. 실측(2026-09-05, 테스트 캐릭터):
//
//   - 키: PostMessage(WM_KEYDOWN/UP)는 게임이 "비활성"이라 판단하면 폐기한다. 직전에
//     WM_ACTIVATE(WA_ACTIVE)+WM_SETFOCUS를 위장 전송하면 방향키·Ctrl+F·Shift+G 전부 정상.
//   - 캡처: PrintWindow(PW_RENDERFULLCONTENT)는 창이 다른 창 뒤에 있어도 전체 화면을 준다
//     (flag 0은 검정 — 반드시 2). 최소화 창은 불가.
//
// 게임이 관리자 권한이면 호출 프로세스도 관리자여야 한다(UIPI, err=5).

var (
	procPostMessageW   = user32.NewProc("PostMessageW")
	procMapVirtualKeyW = user32.NewProc("MapVirtualKeyW")
)

const (
	wmActivate   = 0x0006
	wmSetFocus   = 0x0007
	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	waActive     = 1

	vkShift, vkControl  = 0x10, 0x11 // vkMenu(Alt)=0x12는 window.go에 정의
	pwRenderFullContent = 0x2
)

// bgVK 봇이 쓰는 키 이름 → 가상키 코드(robotgo 키 이름과 호환).
var bgVK = map[string]uint16{
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28,
	"enter": 0x0D, "esc": 0x1B, "escape": 0x1B, "space": 0x20, "tab": 0x09, "backspace": 0x08, "delete": 0x2E,
	"ctrl": vkControl, "control": vkControl, "alt": vkMenu, "shift": vkShift,
}

func keyToVK(key string) (uint16, error) {
	k := strings.ToLower(strings.TrimSpace(key))
	if vk, ok := bgVK[k]; ok {
		return vk, nil
	}
	if len(k) == 1 {
		c := k[0]
		switch {
		case c >= 'a' && c <= 'z':
			return uint16(c - 'a' + 'A'), nil // VK_A~Z = 'A'~'Z'
		case c >= '0' && c <= '9':
			return uint16(c), nil // VK_0~9 = '0'~'9'
		}
	}
	return 0, fmt.Errorf("알 수 없는 키: %q", key)
}

func isExtendedVK(vk uint16) bool {
	switch vk {
	case 0x25, 0x26, 0x27, 0x28, 0x2D, 0x2E, 0x24, 0x23, 0x21, 0x22: // 방향키/Ins/Del/Home/End/PgUp/PgDn
		return true
	}
	return false
}

func keyLParam(vk uint16, up, altHeld bool) uintptr {
	scan, _, _ := procMapVirtualKeyW.Call(uintptr(vk), 0) // MAPVK_VK_TO_VSC
	lp := uintptr(1) | (scan&0xFF)<<16
	if isExtendedVK(vk) {
		lp |= 1 << 24
	}
	if altHeld {
		lp |= 1 << 29 // 컨텍스트 코드(Alt 눌림) — WM_SYSKEY* 규약
	}
	if up {
		lp |= 1<<30 | 1<<31
	}
	return lp
}

func postMsg(hwnd uint64, msg uint32, wp, lp uintptr) error {
	ret, _, err := procPostMessageW.Call(uintptr(hwnd), uintptr(msg), wp, lp)
	if ret == 0 {
		return fmt.Errorf("PostMessage(0x%x) 실패: %v", msg, err)
	}
	return nil
}

// BgSpoofActive 게임에 "너 활성 창이야"를 위장 전송 — 키 메시지 직전마다 호출(싸다).
func BgSpoofActive(hwnd uint64) error {
	if err := postMsg(hwnd, wmActivate, waActive, 0); err != nil {
		return err
	}
	return postMsg(hwnd, wmSetFocus, 0, 0)
}

// 백그라운드 탭 타이밍. 기존 모드(robotgo SendInput, KeySleep=0)는 다운→업 사이가 ~0ms라 탭당 비용이
// 거의 없는데, 백그라운드는 키 유지 45ms + 조합키 간격 15ms×2를 두어 이동(칸당 탭)·스킬이 체감상 느렸다
// (사용자 2026-09-12: "백그라운드가 포그라운드보다 조금 느림 — 움직임도 스킬도"). PostMessage 키는 게임이
// 메시지 자체로 처리하고 키 상태를 폴링하지 않으므로 유지 시간은 짧아도 무방 → 45→22ms, 15→8ms.
// (실측 2창 테스트에서 Ctrl+F·Shift+G·Ctrl+Alt+1 조합은 순서 보존만 필요했음.)
const (
	bgKeyHold = 22 * time.Millisecond
	bgModGap  = 8 * time.Millisecond
)

// BgKeyTap 비활성 창으로 키 탭(조합키 포함) — keyTap(key, mods...)와 같은 인자 규약.
// Alt 조합은 WM_SYSKEYDOWN/UP + 컨텍스트 비트로 보낸다(Alt 눌린 채의 키 메시지 규약).
func BgKeyTap(hwnd uint64, key string, mods ...string) error {
	vk, err := keyToVK(key)
	if err != nil {
		return err
	}
	modVKs := make([]uint16, 0, len(mods))
	altHeld := false
	for _, m := range mods {
		mv, err := keyToVK(m)
		if err != nil {
			return err
		}
		if mv == vkMenu {
			altHeld = true
		}
		modVKs = append(modVKs, mv)
	}
	if err := BgSpoofActive(hwnd); err != nil {
		return err
	}
	// 위장 → 키 사이 대기 불필요: 같은 스레드 큐에 순서대로 들어가므로(PostMessage 순서 보존)
	// 게임은 활성 메시지를 먼저 처리한 뒤 키를 본다. (예전 30ms 대기 제거 — 탭당 비용 절감)
	down, up := uint32(wmKeyDown), uint32(wmKeyUp)
	// 조합키 누르기 — Alt 자체는 SYSKEYDOWN, Alt가 눌린 뒤의 키들도 SYSKEY*
	for _, mv := range modVKs {
		msg := uint32(wmKeyDown)
		if mv == vkMenu {
			msg = wmSysKeyDown
		}
		if err := postMsg(hwnd, msg, uintptr(mv), keyLParam(mv, false, mv == vkMenu)); err != nil {
			return err
		}
		time.Sleep(bgModGap)
	}
	if altHeld {
		down, up = wmSysKeyDown, wmSysKeyUp
	}
	if err := postMsg(hwnd, down, uintptr(vk), keyLParam(vk, false, altHeld)); err != nil {
		return err
	}
	time.Sleep(bgKeyHold)
	if err := postMsg(hwnd, up, uintptr(vk), keyLParam(vk, true, altHeld)); err != nil {
		return err
	}
	// 조합키 떼기(역순)
	for i := len(modVKs) - 1; i >= 0; i-- {
		mv := modVKs[i]
		time.Sleep(bgModGap)
		msg := uint32(wmKeyUp)
		if mv == vkMenu {
			msg = wmSysKeyUp
		}
		if err := postMsg(hwnd, msg, uintptr(mv), keyLParam(mv, true, false)); err != nil {
			return err
		}
	}
	return nil
}

// CaptureWindowBG 비활성(가려진) 창 캡처 — PrintWindow(PW_RENDERFULLCONTENT). 창을 앞으로
// 끌어오지 않는다. 결과 크기 = GetWindowRect 크기(DPI 인식 프로세스면 물리 픽셀).
func (wm *WindowManager) CaptureWindowBG(hwnd uint64) (*image.RGBA, WindowRect, error) {
	rect, err := wm.GetWindowRect(hwnd)
	if err != nil {
		return nil, rect, err
	}
	width, height := int(rect.Right-rect.Left), int(rect.Bottom-rect.Top)
	if width <= 0 || height <= 0 {
		return nil, rect, fmt.Errorf("유효하지 않은 창 크기: %dx%d", width, height)
	}
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, rect, fmt.Errorf("GetDC 실패")
	}
	defer procReleaseDC.Call(0, screenDC)
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, rect, fmt.Errorf("CreateCompatibleDC 실패")
	}
	defer procDeleteDC.Call(memDC)

	var bmi bitmapInfo
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(width)
	bmi.BmiHeader.BiHeight = -int32(height)
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = biRGB
	var bits unsafe.Pointer
	hBitmap, _, _ := procCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bmi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hBitmap == 0 {
		return nil, rect, fmt.Errorf("CreateDIBSection 실패")
	}
	defer procDeleteObject.Call(hBitmap)
	procSelectObject.Call(memDC, hBitmap)

	if ret, _, err := procPrintWindow.Call(uintptr(hwnd), memDC, pwRenderFullContent); ret == 0 {
		return nil, rect, fmt.Errorf("PrintWindow 실패: %v", err)
	}
	dataSize := width * height * 4
	data := unsafe.Slice((*byte)(bits), dataSize)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < dataSize; i += 4 {
		img.Pix[i+0] = data[i+2]
		img.Pix[i+1] = data[i+1]
		img.Pix[i+2] = data[i+0]
		img.Pix[i+3] = 255
	}
	return img, rect, nil
}
