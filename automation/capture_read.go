package automation

import (
	"fmt"
	"image"
	"log"
	"time"
)

// 캡처 방식에 대한 메모
//
// CaptureWindowRaw 는 두 가지를 한다: (1) 창을 전면으로 끌어오고 (2) 화면 DC 를 창 RECT
// 위치에서 BitBlt 한다. 그래서 두 가지 문제가 있다.
//
//   - 앞 창이 덮고 있으면 그 창의 픽셀이 찍힌다. 여러 창을 연달아 읽을 때
//     SetForegroundWindow 가 제때 반영되지 않으면 직전 창 화면이 그대로 찍혀
//     결과가 한 칸씩 밀린다 (창2 자리에 창1 닉네임 — 실제로 발생했다).
//   - 주기적으로 도는 감시기가 쓰면 백그라운드로 돌리라고 해둔 창을 계속 전면으로 끌어온다.
//
// CaptureWindowQuiet(PrintWindow, PW_RENDERFULLCONTENT)는 Z-order 와 무관하게 해당 창만
// 그리므로 둘 다 없다. 다만 게임이 관리자 권한으로 떠 있으면 UIPI 로 막히고(err=5)
// 최소화된 창도 못 찍는다. 그런 상황은 애초에 백그라운드 입력도 안 되는 환경이라,
// 그때만 기존 방식으로 떨어진다.

// CaptureWindowQuiet 창을 앞으로 끌어오지 않는 캡처. 실패하거나 결과가 사실상 단색이면 오류.
func (wm *WindowManager) CaptureWindowQuiet(hwnd uint64) (*image.RGBA, error) {
	img, _, err := wm.CaptureWindowBG(hwnd)
	if err != nil {
		return nil, err
	}
	if imageMostlyBlank(img) {
		return nil, fmt.Errorf("PrintWindow 결과가 비어 있음")
	}
	return img, nil
}

// captureRead 읽기(인식)용 창 캡처.
// activate=false 면 무슨 일이 있어도 창을 전면으로 끌어오지 않는다 — 실패하면 오류를 낸다.
func (om *OCRManager) captureRead(hwnd uint64, activate bool) (*image.RGBA, error) {
	if activate {
		om.wm.ActivateWindow(hwnd)
		time.Sleep(400 * time.Millisecond)
	}
	img, qerr := om.wm.CaptureWindowQuiet(hwnd)
	if qerr == nil {
		return img, nil
	}
	if !activate {
		return nil, fmt.Errorf("백그라운드 캡처 실패: %v", qerr)
	}
	log.Printf("[캡처] PrintWindow 실패 (hwnd=%d): %v → 화면 캡처로 대체", hwnd, qerr)
	raw, _, err := om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		return nil, fmt.Errorf("창 캡처 실패: %v", err)
	}
	return raw, nil
}

// imageMostlyBlank PrintWindow 가 실패 대신 새까만(또는 단색) 비트맵을 돌려주는 경우를 거른다.
// 전체를 다 볼 필요는 없어서 격자로 성기게 훑는다.
func imageMostlyBlank(img *image.RGBA) bool {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 8 || h < 8 {
		return true
	}
	stepX, stepY := w/64+1, h/64+1
	var first uint32
	n, same, dark := 0, 0, 0
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r, g, bb, _ := img.At(x, y).RGBA()
			v := (r>>8)<<16 | (g>>8)<<8 | (bb >> 8)
			if n == 0 {
				first = v
			}
			if v == first {
				same++
			}
			if (r>>8)+(g>>8)+(bb>>8) < 24 {
				dark++
			}
			n++
		}
	}
	if n == 0 {
		return true
	}
	return dark*100/n > 98 || same*100/n > 99
}
