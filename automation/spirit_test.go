package automation

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"
)

// 영술사 빙의 화면을 그린다: 흰 바탕 + 캐릭터를 가운데 둔 17x15칸 파란 테두리(2px) + 분홍 커서 칸.
// 실제 스샷(1111.png = 커서가 캐릭터에 가려짐, 2222.png = 커서 오른쪽 6칸)과 같은 색·크기.
func drawSpirit(possessed bool, charX, charY, curDX, curDY int, cursorVisible bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1600, 900))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{240, 240, 240, 255}}, image.Point{}, draw.Src)
	if !possessed {
		return img
	}
	blue := color.RGBA{0, 0, 171, 255}
	left, top := charX-spiritCenterOffX, charY-spiritCenterOffY
	right, bottom := left+2*spiritHalfW*spiritCell+spiritCell, top+2*spiritHalfH*spiritCell+spiritCell
	for x := left; x <= right; x++ {
		for _, y := range []int{top, top + 1, bottom - 1, bottom} {
			if image.Pt(x, y).In(img.Bounds()) {
				img.Set(x, y, blue)
			}
		}
	}
	for y := top; y <= bottom; y++ {
		for _, x := range []int{left, left + 1, right - 1, right} {
			if image.Pt(x, y).In(img.Bounds()) {
				img.Set(x, y, blue)
			}
		}
	}
	if cursorVisible && (curDX != 0 || curDY != 0) {
		x0, y0 := charX-spiritCell/2+curDX*spiritCell, charY-spiritCell/2+curDY*spiritCell
		draw.Draw(img, image.Rect(x0, y0, x0+50, y0+50), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(x0+2, y0+2, x0+48, y0+48), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(x0+4, y0+4, x0+46, y0+46), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(x0+6, y0+6, x0+44, y0+44), &image.Uniform{color.RGBA{250, 166, 166, 255}}, image.Point{}, draw.Src)
	}
	return img
}

func TestDetectSpiritView(t *testing.T) {
	full := image.Rect(0, 0, 1600, 900)
	cases := []struct {
		name           string
		img            *image.RGBA
		grid, cursor   bool
		dx, dy, cx, cy int
	}{
		{"커서 오른쪽 6칸 (2222.png)", drawSpirit(true, 696, 456, 6, 0, true), true, true, 6, 0, 696, 456},
		{"커서 왼쪽 위", drawSpirit(true, 696, 456, -3, -2, true), true, true, -3, -2, 696, 456},
		{"커서가 캐릭터에 가려짐 (1111.png)", drawSpirit(true, 696, 456, 0, 0, true), true, false, 0, 0, 696, 456},
		{"빙의 안 됨", drawSpirit(false, 696, 456, 0, 0, false), false, false, 0, 0, 0, 0},
		{"왼쪽이 화면 밖", drawSpirit(true, 300, 456, 2, 1, true), true, true, 2, 1, 300, 456},
	}
	for _, c := range cases {
		v := DetectSpiritView(c.img, full)
		if v.Grid != c.grid || v.Cursor != c.cursor || v.DX != c.dx || v.DY != c.dy {
			t.Errorf("%s: got %+v, want grid=%v cursor=%v d=(%d,%d)", c.name, v, c.grid, c.cursor, c.dx, c.dy)
			continue
		}
		if c.grid && (v.CX != c.cx || v.CY != c.cy) {
			t.Errorf("%s: 캐릭터 칸 (%d,%d), want (%d,%d)", c.name, v.CX, v.CY, c.cx, c.cy)
		}
	}
}

// spiritSim 영술사 빙의 이동을 흉내 내는 가짜 게임 — 방향키는 커서, Q는 커서 칸으로 이동.
type spiritSim struct {
	possessed      bool
	x, y           int // 캐릭터 맵 좌표
	cdx, cdy       int // 커서 - 캐릭터 (칸)
	followCursor   bool // Q 뒤에도 커서가 캐릭터 기준 같은 자리에 남는 게임이라면 true
	blocked        bool // Q 를 눌러도 못 감(벽)
	keys           []string
}

func (s *spiritSim) io() SpiritIO {
	return SpiritIO{
		Capture: func() (*image.RGBA, error) { return drawSpirit(s.possessed, 696, 456, s.cdx, s.cdy, true), nil },
		Region:  func(img *image.RGBA) image.Rectangle { return img.Bounds() },
		Coords:  func(img *image.RGBA) (GameCoords, error) { return GameCoords{X: s.x, Y: s.y}, nil },
		Tap: func(k string) {
			s.keys = append(s.keys, k)
			switch k {
			case "5":
				s.possessed = true
			case "left":
				s.cdx = spiritClamp(s.cdx-1, spiritHalfW)
			case "right":
				s.cdx = spiritClamp(s.cdx+1, spiritHalfW)
			case "up":
				s.cdy = spiritClamp(s.cdy-1, spiritHalfH)
			case "down":
				s.cdy = spiritClamp(s.cdy+1, spiritHalfH)
			case "q":
				if s.possessed && !s.blocked {
					s.x, s.y = s.x+s.cdx, s.y+s.cdy
					if !s.followCursor {
						s.cdx, s.cdy = 0, 0
					}
				}
			}
		},
		Sleep: func(time.Duration) bool { return true },
		Log:   func(string) {},
	}
}

func TestSpiritMoveTo(t *testing.T) {
	cases := []struct {
		name               string
		sim                spiritSim
		wantX, wantY       int
	}{
		{"빙의 중, 커서가 따라옴", spiritSim{possessed: true, x: 30, y: 40}, 34, 37},
		{"Q 뒤 커서가 떨어진 자리에 남음 → 되돌림", spiritSim{possessed: true, x: 30, y: 40, followCursor: true}, 34, 37},
		{"빙의 안 됨 → 5번 누르고 이동", spiritSim{x: 38, y: 37}, 34, 37},
		{"멀면 네모 안까지만(좌우 8칸)", spiritSim{possessed: true, x: 20, y: 37}, 28, 37},
		{"벽이라 못 감 → 커서만 되돌림", spiritSim{possessed: true, x: 30, y: 37, blocked: true}, 30, 37},
	}
	for _, c := range cases {
		s := c.sim
		SpiritMoveTo(s.io(), 34, 37, "5", "테스트")
		if s.x != c.wantX || s.y != c.wantY {
			t.Errorf("%s: 위치 (%d,%d), want (%d,%d) keys=%v", c.name, s.x, s.y, c.wantX, c.wantY, s.keys)
		}
		if s.cdx != 0 || s.cdy != 0 {
			t.Errorf("%s: 커서가 캐릭터 칸에 없음 (%+d,%+d) keys=%v", c.name, s.cdx, s.cdy, s.keys)
		}
	}
	// 이미 중앙(±1)이면 키를 안 누른다
	s := spiritSim{possessed: true, x: 35, y: 36}
	if SpiritMoveTo(s.io(), 34, 37, "5", "테스트") || len(s.keys) != 0 {
		t.Errorf("중앙인데 키를 누름: %v", s.keys)
	}
}
