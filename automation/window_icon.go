package automation

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// 작업표시줄/Alt+Tab 아이콘 설정용.
// 탐색기에 보이는 "파일 아이콘"은 exe의 PE 리소스에서 오지만,
// 작업표시줄 아이콘은 창(HWND)에 따로 설정해야 한다.
// webview가 만든 창은 아이콘이 지정돼 있지 않아 기본 아이콘으로 표시된다.
var (
	shell32            = syscall.NewLazyDLL("shell32.dll")
	procExtractIconExW = shell32.NewProc("ExtractIconExW")
	procSendMessageW   = user32.NewProc("SendMessageW")
)

const (
	wmSetIcon    = 0x0080
	iconSmallIdx = 0 // 제목표시줄/Alt+Tab 작은 아이콘
	iconBigIdx   = 1 // 작업표시줄 큰 아이콘
)

// SetWindowIconFromExe 실행 파일에 임베드된 아이콘을 지정한 창에 적용한다.
// 리소스 ID를 몰라도 되도록 실행 파일 경로에서 직접 추출한다.
func SetWindowIconFromExe(hwnd uintptr) error {
	if hwnd == 0 {
		return fmt.Errorf("창 핸들이 없습니다")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("실행 파일 경로 조회 실패: %v", err)
	}
	pathPtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}

	var hLarge, hSmall uintptr
	ret, _, _ := procExtractIconExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0, // 첫 번째 아이콘
		uintptr(unsafe.Pointer(&hLarge)),
		uintptr(unsafe.Pointer(&hSmall)),
		1,
	)
	if ret == 0 || (hLarge == 0 && hSmall == 0) {
		return fmt.Errorf("실행 파일에 아이콘이 없습니다 (%s)", exePath)
	}

	if hBig := hLarge; hBig != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconBigIdx, hBig)
	}
	if hSml := hSmall; hSml != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconSmallIdx, hSml)
	}
	return nil
}