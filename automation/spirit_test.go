package automation

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
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
	possessed    bool
	x, y         int  // 캐릭터 맵 좌표
	cdx, cdy     int  // 커서 - 캐릭터 (칸)
	followCursor bool // Q 뒤에도 커서가 캐릭터 기준 같은 자리에 남는 게임이라면 true
	blocked      bool // Q 를 눌러도 못 감(벽)
	bgIgnored    bool // 백그라운드(PostMessage) 키를 게임이 무시함 — 포그라운드로만 먹음
	hideNear     int  // 캐릭터에서 이 칸 수 안의 커서는 빙의 오라에 가려 안 보임 (실제 게임 ±2쯤)
	fgUsed       bool
	keys         []string
}

func (s *spiritSim) apply(k string) {
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
}

func (s *spiritSim) io() SpiritIO {
	return SpiritIO{
		Capture: func() (*image.RGBA, error) {
			visible := !(meAbs(s.cdx) <= s.hideNear && meAbs(s.cdy) <= s.hideNear)
			return drawSpirit(s.possessed, 696, 456, s.cdx, s.cdy, visible), nil
		},
		Region: func(img *image.RGBA) image.Rectangle { return img.Bounds() },
		Coords: func(img *image.RGBA) (GameCoords, error) { return GameCoords{X: s.x, Y: s.y}, nil },
		Tap: func(k string) {
			s.keys = append(s.keys, k)
			if !s.bgIgnored {
				s.apply(k)
			}
		},
		TapFG: func(k string) {
			s.keys = append(s.keys, "fg:"+k)
			s.fgUsed = true
			s.apply(k)
		},
		Sleep: func(time.Duration) bool { return true },
		Log:   func(string) {},
	}
}

func TestSpiritMoveTo(t *testing.T) {
	cases := []struct {
		name         string
		sim          spiritSim
		wantX, wantY int
		wantFG       bool
	}{
		{"빙의 중, 커서가 따라옴", spiritSim{possessed: true, x: 30, y: 40}, 34, 37, false},
		{"Q 뒤 커서가 떨어진 자리에 남음 → 되돌림", spiritSim{possessed: true, x: 30, y: 40, followCursor: true}, 34, 37, false},
		{"빙의 안 됨 → 5번 누르고 이동", spiritSim{x: 38, y: 37}, 34, 37, false},
		{"멀면 네모 안까지만(좌우 8칸)", spiritSim{possessed: true, x: 20, y: 37}, 28, 37, false},
		{"벽 → 포그라운드 Q 로 한 번 더 해도 그대로, 커서만 되돌림", spiritSim{possessed: true, x: 30, y: 37, blocked: true}, 30, 37, true},
		{"백그라운드 키가 안 먹음 → 포그라운드로 다시", spiritSim{possessed: true, x: 30, y: 40, bgIgnored: true}, 34, 37, true},
		{"빙의 안 됨 + 백그라운드 안 먹음 → 포그라운드로 5번부터", spiritSim{x: 38, y: 37, bgIgnored: true}, 34, 37, true},
		{"가까운 목표(커서가 오라에 가려짐) + 백그라운드 됨", spiritSim{possessed: true, x: 32, y: 37, hideNear: 2}, 34, 37, false},
		{"가까운 목표(커서가 가려짐) + 백그라운드 안 먹음 → 포그라운드", spiritSim{possessed: true, x: 32, y: 36, hideNear: 2, bgIgnored: true}, 34, 37, true},
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
		if s.fgUsed != c.wantFG {
			t.Errorf("%s: 포그라운드 사용=%v, want %v keys=%v", c.name, s.fgUsed, c.wantFG, s.keys)
		}
	}
	// 이미 중앙(±1)이면 키를 안 누른다
	s := spiritSim{possessed: true, x: 35, y: 36}
	if SpiritMoveTo(s.io(), 34, 37, "5", "테스트") || len(s.keys) != 0 {
		t.Errorf("중앙인데 키를 누름: %v", s.keys)
	}
}

// '영술사 테스트' 진단 — 오른쪽 4칸 → 커서가 보이든 말든 Q → 좌표로 이동 여부. 이동하면 제자리로 돌아온다.
func TestSpiritDiagnose(t *testing.T) {
	has := func(lines []string, sub string) bool {
		for _, l := range lines {
			if strings.Contains(l, sub) {
				return true
			}
		}
		return false
	}
	s := spiritSim{possessed: true, x: 30, y: 37, hideNear: 2}
	io := s.io()
	saved := 0
	io.Save = func(string, *image.RGBA) { saved++ }
	lines := SpiritDiagnose(io)
	if !has(lines, "백그라운드 방향키(→ 4칸) → 커서 캐릭터에서 +4,+0 칸") ||
		!has(lines, "백그라운드 Q → 좌표 (34,37) — 이동함") || !has(lines, "백그라운드 제자리로 Q → 좌표 (30,37)") {
		t.Errorf("백그라운드 정상: %v", lines)
	}
	if s.x != 30 || s.cdx != 0 || s.cdy != 0 || saved < 3 {
		t.Errorf("진단 뒤 제자리·커서 캐릭터 칸·화면 저장: x=%d 커서 %+d,%+d 저장 %d장", s.x, s.cdx, s.cdy, saved)
	}
	s = spiritSim{possessed: true, x: 30, y: 37, hideNear: 2, bgIgnored: true}
	lines = SpiritDiagnose(s.io())
	if !has(lines, "백그라운드 Q → 좌표 그대로") || !has(lines, "포그라운드 Q → 좌표 (34,37) — 이동함") {
		t.Errorf("백그라운드 안 먹음: %v", lines)
	}
	// 커서가 캡처에 전혀 안 찍혀도 Q 는 누르고 좌표로 판단한다
	s = spiritSim{possessed: true, x: 30, y: 37, hideNear: 99}
	lines = SpiritDiagnose(s.io())
	if !has(lines, "커서 캐릭터 칸(가려짐)") || !has(lines, "백그라운드 Q → 좌표 (34,37) — 이동함") {
		t.Errorf("커서가 안 찍힘: %v", lines)
	}
	s = spiritSim{x: 30, y: 37}
	if lines = SpiritDiagnose(s.io()); !has(lines, "파란 네모가 안 보임") {
		t.Errorf("빙의 안 됨: %v", lines)
	}
}
