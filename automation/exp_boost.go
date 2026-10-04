package automation

import (
	"image"
	"image/color"
	"strings"
)

// 경험치 칸 3개 — 왼쪽 위 버프 창의 "경험치 [나나노][요강][물약]".
// 시련은 3칸이 다 켜져 있을 때만 들어간다(사용자 2026-10-04: 경험치가 꺼져 있으면 입장하지 말 것).
//
// 칸은 위/아래 두 톤으로 칠해진 UI 그림이라 배경과 상관없이 색이 정해져 있다(스샷 765장으로 확인):
//   켜짐 = 1번 파랑(위 47,140,154 / 아래 28,110,152), 2번 주황(202,154,42 / 196,114,25),
//          3번 초록(82,167,35 / 86,139,22) — 칸 안엔 남은 시간("60분")
//   꺼짐 = 갈색빛 회색(65,59,49 / 54,45,39) — 칸 안엔 이름(나나노/요강/물약)
// 창마다 버프 창 위치가 조금씩 달라(위쪽 파티 체력바 등) 위치를 고정하지 않고 찾는다:
// 같은 칸 색이 30~40px 이어지는 구간 세 개가 43px 간격으로 나란한 줄 + 칸마다 왼쪽 테두리
// (어두운 바깥선 → 밝은 안쪽선). 맵 바닥 타일이 꺼진 칸 색과 비슷해서 테두리 확인이 꼭 필요하다.

// ExpBoost 경험치 칸 3개 상태
type ExpBoost struct {
	Found  bool    // 칸 3개를 찾았는지 (못 찾으면 창고·상점 창이 덮었거나 맵 이동 암전 등)
	Active [3]bool // 칸마다 켜졌는지
}

// expBoostNames 칸 이름 (꺼졌을 때 칸에 보이는 글자)
var expBoostNames = [3]string{"나나노", "요강", "물약"}

// AllActive 3칸 다 켜졌는지
func (e ExpBoost) AllActive() bool { return e.Found && e.Active[0] && e.Active[1] && e.Active[2] }

// OffNames 꺼진 칸 이름들
func (e ExpBoost) OffNames() []string {
	var out []string
	for i, on := range e.Active {
		if !on {
			out = append(out, expBoostNames[i])
		}
	}
	return out
}

// String 로그용 ("3칸 켜짐" / "나나노·요강 꺼짐" / "칸 못 찾음")
func (e ExpBoost) String() string {
	switch {
	case !e.Found:
		return "칸 못 찾음"
	case e.AllActive():
		return "3칸 켜짐"
	}
	return strings.Join(e.OffNames(), "·") + " 꺼짐"
}

// ReadExpBoost 창 전체 이미지에서 경험치 칸을 읽는다 — 클라이언트 영역 왼쪽 위(가로 420, 세로 300)를 훑는다.
func (om *OCRManager) ReadExpBoost(img image.Image, hwnd uint64) ExpBoost {
	if img == nil {
		return ExpBoost{}
	}
	offX, offY, _, _ := om.clientBox(img, hwnd)
	return DetectExpBoost(img, image.Rect(offX, offY, offX+420, offY+300))
}

type expFill int

const (
	expFillNone expFill = iota
	expFillBlue
	expFillOrange
	expFillGreen
	expFillGray
)

func expNear(c color.RGBA, r, g, b uint8, tol int) bool {
	d := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}
		return int(b - a)
	}
	return d(c.R, r) <= tol && d(c.G, g) <= tol && d(c.B, b) <= tol
}

func expClassify(c color.RGBA) expFill {
	const tol = 8
	switch {
	case expNear(c, 47, 140, 154, tol), expNear(c, 28, 110, 152, tol):
		return expFillBlue
	case expNear(c, 202, 154, 42, tol), expNear(c, 202, 148, 42, tol), expNear(c, 196, 114, 25, tol):
		return expFillOrange
	case expNear(c, 82, 167, 35, tol), expNear(c, 86, 139, 22, tol):
		return expFillGreen
	case expNear(c, 65, 59, 49, 6), expNear(c, 54, 45, 39, 6):
		return expFillGray
	}
	return expFillNone
}

func expLum(c color.RGBA) int { return (int(c.R)*3 + int(c.G)*6 + int(c.B)) / 10 }

func expMaxCh(c color.RGBA) int {
	m := c.R
	if c.G > m {
		m = c.G
	}
	if c.B > m {
		m = c.B
	}
	return int(m)
}

type expRun struct {
	x0, x1 int // [x0, x1)
	fill   expFill
}

// DetectExpBoost img 의 r 영역에서 경험치 칸 3개를 찾아 상태를 돌려준다.
func DetectExpBoost(img image.Image, r image.Rectangle) ExpBoost {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return ExpBoost{}
	}
	at := expPixelFunc(img)
	// leftBorderOK 칸 채움이 x0 에서 시작할 때 왼쪽 테두리: 어두운 바깥선 바로 오른쪽이 밝은 안쪽선.
	// 채움 첫 픽셀이 글자에 걸릴 수 있어 x0-5..x0-2 안에서 찾는다.
	leftBorderOK := func(x0, y int) bool {
		for dx := 2; dx <= 5; dx++ {
			if x0-dx < r.Min.X {
				break
			}
			dark, edge := at(x0-dx, y), at(x0-dx+1, y)
			if expMaxCh(dark) <= 50 && expLum(edge) >= expLum(dark)+35 {
				return true
			}
		}
		return false
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		// 이 줄의 칸 색 구간들 — 같은 색이 1~3px(글자 한 획) 끊겨도 이어 본다
		var runs []expRun
		for x := r.Min.X; x < r.Max.X; {
			f := expClassify(at(x, y))
			if f == expFillNone {
				x++
				continue
			}
			start, last, gap := x, x, 0
			for x++; x < r.Max.X; x++ {
				if expClassify(at(x, y)) == f {
					last, gap = x, 0
					continue
				}
				if gap++; gap > 3 {
					break
				}
			}
			if last-start+1 >= 24 {
				runs = append(runs, expRun{start, last + 1, f})
			}
			x = last + 1
		}
		for i := 0; i+2 < len(runs); i++ {
			a, b, c := runs[i], runs[i+1], runs[i+2]
			if d := b.x0 - a.x0; d < 38 || d > 48 {
				continue
			}
			if d := c.x0 - b.x0; d < 38 || d > 48 {
				continue
			}
			okWidth := func(r expRun) bool { n := r.x1 - r.x0; return n >= 24 && n <= 42 }
			if !okWidth(a) || !okWidth(b) || !okWidth(c) {
				continue
			}
			// 칸마다 정해진 색(또는 꺼짐 회색)이어야 한다 — 세 칸 다 파랑 같은 조합은 경험치 칸이 아니다
			okColor := func(r expRun, want expFill) bool { return r.fill == want || r.fill == expFillGray }
			if !okColor(a, expFillBlue) || !okColor(b, expFillOrange) || !okColor(c, expFillGreen) {
				continue
			}
			// 왼쪽 버프 창 안이어야 하고(맵 바닥 타일 오인 방지), 세 칸 모두 테두리 모양이 맞아야 한다
			if a.x0-r.Min.X > 260 {
				continue
			}
			if !leftBorderOK(a.x0, y) || !leftBorderOK(b.x0, y) || !leftBorderOK(c.x0, y) {
				continue
			}
			return ExpBoost{
				Found:  true,
				Active: [3]bool{a.fill == expFillBlue, b.fill == expFillOrange, c.fill == expFillGreen},
			}
		}
	}
	return ExpBoost{}
}

// expPixelFunc 픽셀 읽기 — 캡처는 *image.RGBA 라 바로 읽는다(1분마다라도 일반 At 보다 수십 배 빠름)
func expPixelFunc(img image.Image) func(x, y int) color.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return func(x, y int) color.RGBA {
			i := rgba.PixOffset(x, y)
			return color.RGBA{rgba.Pix[i], rgba.Pix[i+1], rgba.Pix[i+2], 255}
		}
	}
	return func(x, y int) color.RGBA {
		r, g, b, _ := img.At(x, y).RGBA()
		return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
	}
}
