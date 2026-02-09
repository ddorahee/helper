package automation

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/image/draw"
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procShowWindow               = user32.NewProc("ShowWindow")
	procGetWindowRectProc        = user32.NewProc("GetWindowRect")
	procIsWindow                 = user32.NewProc("IsWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procKeybdEvent               = user32.NewProc("keybd_event")
	procIsIconic                 = user32.NewProc("IsIconic")
	procGetDC                    = user32.NewProc("GetDC")
	procReleaseDC                = user32.NewProc("ReleaseDC")
	procPrintWindow              = user32.NewProc("PrintWindow")

	gdi32                        = syscall.NewLazyDLL("gdi32.dll")
	procCreateCompatibleDC       = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection         = gdi32.NewProc("CreateDIBSection")
	procSelectObject             = gdi32.NewProc("SelectObject")
	procDeleteDC                 = gdi32.NewProc("DeleteDC")
	procDeleteObject             = gdi32.NewProc("DeleteObject")
	procBitBlt                   = gdi32.NewProc("BitBlt")
)

const (
	swRestore      = 9
	vkMenu         = 0x12 // ALT key
	srccopy        = 0x00CC0020
	dibRGBColors   = 0
	biRGB          = 0
)

// BITMAPINFOHEADER Win32 구조체
type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]uint32
}

// WindowRect 창 위치/크기
type WindowRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// GameWindow 발견된 게임 창 정보
type GameWindow struct {
	HWND  uint64     `json:"hwnd"`
	Title string     `json:"title"`
	PID   uint32     `json:"pid"`
	Rect  WindowRect `json:"rect"`
}

// WindowManager Win32 창 관리
type WindowManager struct{}

// NewWindowManager 새로운 WindowManager 생성
func NewWindowManager() *WindowManager {
	return &WindowManager{}
}

// FindGameWindows "바람의나라" 제목의 모든 창 검색
func (wm *WindowManager) FindGameWindows() ([]GameWindow, error) {
	var windows []GameWindow

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		textLen, _, _ := procGetWindowTextLengthW.Call(hwnd)
		if textLen == 0 {
			return 1
		}

		buf := make([]uint16, textLen+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), textLen+1)
		title := syscall.UTF16ToString(buf)

		if title == "바람의나라" {
			var pid uint32
			procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

			var rect WindowRect
			procGetWindowRectProc.Call(hwnd, uintptr(unsafe.Pointer(&rect)))

			windows = append(windows, GameWindow{
				HWND:  uint64(hwnd),
				Title: title,
				PID:   pid,
				Rect:  rect,
			})
		}
		return 1
	})

	ret, _, err := procEnumWindows.Call(cb, 0)
	if ret == 0 {
		return nil, fmt.Errorf("EnumWindows 실패: %v", err)
	}

	return windows, nil
}

// ActivateWindow 창을 전면으로 가져오기
func (wm *WindowManager) ActivateWindow(hwnd uint64) error {
	h := uintptr(hwnd)

	if !wm.IsWindowValid(hwnd) {
		return fmt.Errorf("유효하지 않은 윈도우: %d", hwnd)
	}

	// 최소화되어 있으면 복원
	isMinimized, _, _ := procIsIconic.Call(h)
	if isMinimized != 0 {
		procShowWindow.Call(h, swRestore)
	}

	// ALT 키 트릭으로 SetForegroundWindow 제한 우회
	procKeybdEvent.Call(uintptr(vkMenu), 0, 0, 0)         // ALT down
	procKeybdEvent.Call(uintptr(vkMenu), 0, uintptr(2), 0) // ALT up

	procSetForegroundWindow.Call(h)
	procBringWindowToTop.Call(h)

	return nil
}

// GetWindowRect 창 위치/크기 가져오기
func (wm *WindowManager) GetWindowRect(hwnd uint64) (WindowRect, error) {
	var rect WindowRect
	ret, _, err := procGetWindowRectProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		return rect, fmt.Errorf("GetWindowRect 실패: %v", err)
	}
	return rect, nil
}

// IsWindowValid 윈도우 유효성 확인
func (wm *WindowManager) IsWindowValid(hwnd uint64) bool {
	ret, _, _ := procIsWindow.Call(uintptr(hwnd))
	return ret != 0
}

// CaptureWindow 창 스크린샷을 JPEG base64 문자열로 반환
// 창을 앞으로 가져온 후 화면 DC에서 캡처 (DirectX 게임 대응)
func (wm *WindowManager) CaptureWindow(hwnd uint64) (string, error) {
	// 창을 앞으로 가져오기 (게임은 전면에 있어야 캡처 가능)
	wm.ActivateWindow(hwnd)
	time.Sleep(300 * time.Millisecond)

	// 창 크기/위치 가져오기 (활성화 후 다시)
	rect, err := wm.GetWindowRect(hwnd)
	if err != nil {
		return "", err
	}
	width := int(rect.Right - rect.Left)
	height := int(rect.Bottom - rect.Top)
	if width <= 0 || height <= 0 {
		return "", fmt.Errorf("유효하지 않은 창 크기: %dx%d", width, height)
	}

	// 화면 전체 DC (NULL = 전체 화면)
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return "", fmt.Errorf("GetDC(screen) 실패")
	}
	defer procReleaseDC.Call(0, screenDC)

	// 호환 DC 생성
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return "", fmt.Errorf("CreateCompatibleDC 실패")
	}
	defer procDeleteDC.Call(memDC)

	// DIB Section 생성
	var bmi bitmapInfo
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(width)
	bmi.BmiHeader.BiHeight = -int32(height) // top-down
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = biRGB

	var bits unsafe.Pointer
	hBitmap, _, _ := procCreateDIBSection.Call(
		memDC,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if hBitmap == 0 {
		return "", fmt.Errorf("CreateDIBSection 실패")
	}
	defer procDeleteObject.Call(hBitmap)

	procSelectObject.Call(memDC, hBitmap)

	// 화면 DC에서 창 영역을 직접 복사
	procBitBlt.Call(
		memDC, 0, 0, uintptr(width), uintptr(height),
		screenDC, uintptr(rect.Left), uintptr(rect.Top),
		srccopy,
	)

	// BGRA → RGBA 변환
	dataSize := width * height * 4
	data := unsafe.Slice((*byte)(bits), dataSize)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < dataSize; i += 4 {
		img.Pix[i+0] = data[i+2] // R ← B
		img.Pix[i+1] = data[i+1] // G
		img.Pix[i+2] = data[i+0] // B ← R
		img.Pix[i+3] = 255       // A
	}

	// 썸네일 축소 (max 320px 너비)
	thumbW := 320
	thumbH := int(float64(height) * (float64(thumbW) / float64(width)))
	thumb := image.NewRGBA(image.Rect(0, 0, thumbW, thumbH))
	draw.BiLinear.Scale(thumb, thumb.Bounds(), img, img.Bounds(), draw.Over, nil)

	// JPEG 인코딩 (PNG보다 훨씬 작음)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 60}); err != nil {
		return "", fmt.Errorf("JPEG 인코딩 실패: %v", err)
	}

	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/jpeg;base64," + b64, nil
}
