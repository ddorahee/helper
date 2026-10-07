package automation

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 영술사 빙의 화면을 그린다: 바탕 + 캐릭터를 가운데 둔 17x15칸 파란 테두리(2px) + 커서 칸(검·흰·검 테두리 + 속 색).
// 크기·자리는 앱 캡처 실측(2026-10-07)과 같다: 테두리 바깥선 = 캐릭터 칸 중심 - (408,360), 커서 테두리 = 칸 -1~48px.
func drawSpirit(possessed bool, charX, charY, curDX, curDY int, cursorDrawn bool) *image.RGBA {
	return drawSpiritOn(color.RGBA{240, 240, 240, 255}, possessed, charX, charY, curDX, curDY, cursorDrawn, color.RGBA{250, 166, 166, 255})
}

func drawSpiritOn(bg color.RGBA, possessed bool, charX, charY, curDX, curDY int, cursorDrawn bool, fill color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1600, 900))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	if !possessed {
		return img
	}
	blue := color.RGBA{0, 0, 171, 255}
	left, top := charX-spiritCenterOffX, charY-spiritCenterOffY
	right, bottom := left+spiritFullW-1, top+2*spiritHalfH*spiritCell+spiritCell
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
	if cursorDrawn {
		x0, y0 := charX-spiritCell/2+curDX*spiritCell, charY-spiritCell/2+curDY*spiritCell
		fillRect := func(a, b int, c color.RGBA) {
			draw.Draw(img, image.Rect(x0+a, y0+a, x0+b, y0+b), &image.Uniform{c}, image.Point{}, draw.Src)
		}
		fillRect(-1, 49, color.RGBA{0, 0, 0, 255})
		fillRect(1, 47, color.RGBA{255, 255, 255, 255})
		fillRect(3, 45, color.RGBA{0, 0, 0, 255})
		fillRect(5, 43, fill)
	}
	return img
}

func TestDetectSpiritView(t *testing.T) {
	full := image.Rect(0, 0, 1600, 900)
	// 임무창이 위쪽 테두리 왼쪽을 가린 화면 (실측: 296~300 | 임무창 | 551~1112)
	panel := drawSpirit(true, 704, 487, 4, 0, true)
	draw.Draw(panel, image.Rect(301, 70, 551, 300), &image.Uniform{color.RGBA{30, 30, 40, 255}}, image.Point{}, draw.Src)
	// 커서가 맨 윗줄에서 위쪽 테두리를 덮은 화면 (실측: 2_백그라운드 — 커서 -2,-7)
	topRow := drawSpirit(true, 704, 487, -2, -7, true)
	draw.Draw(topRow, image.Rect(301, 70, 551, 300), &image.Uniform{color.RGBA{30, 30, 40, 255}}, image.Point{}, draw.Src)
	cases := []struct {
		name           string
		img            *image.RGBA
		grid, cursor   bool
		dx, dy, cx, cy int
	}{
		{"커서 오른쪽 6칸 (2222.png)", drawSpirit(true, 696, 456, 6, 0, true), true, true, 6, 0, 696, 456},
		{"커서 왼쪽 위", drawSpirit(true, 696, 456, -3, -2, true), true, true, -3, -2, 696, 456},
		{"커서가 캐릭터 칸에 그려짐", drawSpirit(true, 696, 456, 0, 0, true), true, true, 0, 0, 696, 456},
		{"커서가 안 그려짐 (1111.png)", drawSpirit(true, 696, 456, 0, 0, false), true, false, 0, 0, 696, 456},
		{"빙의 안 됨", drawSpirit(false, 696, 456, 0, 0, false), false, false, 0, 0, 0, 0},
		{"왼쪽이 화면 밖", drawSpirit(true, 300, 456, 2, 1, true), true, true, 2, 1, 300, 456},
		{"오른쪽이 화면 밖", drawSpirit(true, 1400, 456, -5, 3, true), true, true, -5, 3, 1400, 456},
		{"임무창이 위쪽 테두리를 가림", panel, true, true, 4, 0, 704, 487},
		{"커서가 위쪽 테두리를 덮음 + 임무창", topRow, true, true, -2, -7, 704, 487},
		{"어두운 바닥 (검은 줄만 맞는 곳은 커서 아님)", drawSpiritOn(color.RGBA{0, 0, 0, 255}, true, 696, 456, 0, 0, false, color.RGBA{}), true, false, 0, 0, 696, 456},
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
	// 속 색
	green := drawSpiritOn(color.RGBA{97, 67, 39, 255}, true, 704, 487, 4, 0, true, color.RGBA{48, 119, 19, 255})
	if v := DetectSpiritView(green, full); !v.Cursor || v.Tint != "초록" {
		t.Errorf("초록 커서: %+v", v)
	}
	if v := DetectSpiritView(drawSpirit(true, 704, 487, 4, 0, true), full); v.Tint != "빨강" {
		t.Errorf("분홍 커서는 빨강: %+v", v)
	}
}

// 실제 화면에서 잘라 낸 커서 칸 (앱 캡처 2026-10-07 고구려-일본선착장, 사용자 스샷 2222.png 환상의시련장).
// 자른 그림의 (0,0) 이 원본의 (ox,oy) — 캐릭터 칸 중심은 원본 기준 (cx,cy).
func TestSpiritCursorRealCrops(t *testing.T) {
	cases := []struct {
		file           string
		ox, oy, cx, cy int
		found          bool
		dx, dy         int
		tint           string
	}{
		{"green_r4.png", 840, 430, 704, 487, true, 4, 0, "초록"},   // 포그라운드 → 4번 뒤
		{"red_center.png", 650, 430, 704, 487, true, 0, 0, "빨강"}, // Q 로 간 뒤 새 캐릭터 칸
		{"wall_top.png", 555, 100, 704, 487, true, -2, -7, "빨강"}, // 맨 윗줄 벽 칸 (위쪽 테두리를 덮음)
		{"pink_r6.png", 930, 430, 697, 487, true, 6, 0, "빨강"},    // 2222.png 흰 바닥 분홍 커서
		{"none_center.png", 650, 430, 704, 487, false, 0, 0, ""}, // 시작 화면 — 커서 안 그려짐
		{"white_floor.png", 400, 300, 697, 487, false, 0, 0, ""}, // 흰 바닥 (흰 줄만 맞음)
	}
	for _, c := range cases {
		f, err := os.Open(filepath.Join("testdata", "spirit", c.file))
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		found, dx, dy, tint := spiritFindCursor(expPixelFunc(img), img.Bounds(), c.cx-c.ox, c.cy-c.oy)
		if found != c.found || dx != c.dx || dy != c.dy || tint != c.tint {
			t.Errorf("%s: found=%v d=(%d,%d) %q, want %v (%d,%d) %q", c.file, found, dx, dy, tint, c.found, c.dx, c.dy, c.tint)
		}
	}
}

// spiritSim 영술사 빙의 이동을 흉내 내는 가짜 게임 — 방향키는 커서, Q는 커서 칸으로 이동(간 뒤 커서는 새 캐릭터 칸).
type spiritSim struct {
	possessed bool
	x, y      int // 캐릭터 맵 좌표
	cdx, cdy  int // 커서 - 캐릭터 (칸)
	// 게임이 이럴 수도 있는 동작들 — 대비용 (2차 테스트 2026-10-07: 실제 게임은 백그라운드 키도 됨)
	bgIgnored       bool            // 백그라운드(PostMessage) 방향키·Q 를 무시 — 포그라운드로만 먹음
	hideInactive    bool            // 비활성 창은 커서를 안 그림 (활성 위장을 받으면 그림)
	resetOnActivate bool            // 창을 앞으로 가져오면 커서가 캐릭터 칸으로
	walls           map[[2]int]bool // 못 가는 칸 (맵 좌표)
	dropEvery       int             // 백그라운드 방향키를 n번에 한 번 놓침
	noFG            bool            // 포그라운드 입력 없음
	active, awake   bool
	fgUsed          bool
	arrows          int
	keys            []string
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
		to := [2]int{s.x + s.cdx, s.y + s.cdy}
		if s.possessed && (s.cdx != 0 || s.cdy != 0) && !s.walls[to] {
			s.x, s.y, s.cdx, s.cdy = to[0], to[1], 0, 0
		}
	}
}

func (s *spiritSim) activate() {
	if !s.active {
		s.active = true
		if s.resetOnActivate {
			s.cdx, s.cdy = 0, 0
		}
	}
}

func (s *spiritSim) io() SpiritIO {
	io := SpiritIO{
		Capture: func() (*image.RGBA, error) {
			drawn := !s.hideInactive || s.active || s.awake
			return drawSpirit(s.possessed, 696, 456, s.cdx, s.cdy, drawn), nil
		},
		Region: func(img *image.RGBA) image.Rectangle { return img.Bounds() },
		Coords: func(img *image.RGBA) (GameCoords, error) { return GameCoords{X: s.x, Y: s.y}, nil },
		Tap: func(k string) {
			s.keys = append(s.keys, k)
			s.awake = true // 백그라운드 키는 활성 위장과 함께 간다
			if k == "5" {
				s.apply(k) // 스킬 키는 백그라운드로도 먹음
				return
			}
			if s.bgIgnored {
				return
			}
			if k != "q" {
				if s.arrows++; s.dropEvery > 0 && s.arrows%s.dropEvery == 0 {
					return
				}
			}
			s.apply(k)
		},
		Wake:  func() { s.awake = true },
		Sleep: func(time.Duration) bool { return true },
		Log:   func(string) {},
	}
	if !s.noFG {
		io.Activate = s.activate
		io.TapFG = func(k string) {
			s.keys = append(s.keys, "fg:"+k)
			s.fgUsed = true
			s.activate()
			s.apply(k)
		}
	}
	return io
}

func TestSpiritMoveTo(t *testing.T) {
	cases := []struct {
		name         string
		sim          spiritSim
		wantX, wantY int
		wantFG       bool
	}{
		{"백그라운드로 됨", spiritSim{possessed: true, x: 30, y: 40}, 34, 37, false},
		{"빙의 안 됨 → 5번 누르고 이동", spiritSim{x: 38, y: 37}, 34, 37, false},
		{"멀면 네모 안까지만(좌우 8칸)", spiritSim{possessed: true, x: 20, y: 37}, 28, 37, false},
		{"커서가 전에 옮긴 자리에 남아 있음 → 화면 보고 고쳐 감", spiritSim{possessed: true, x: 30, y: 40, cdx: -2, cdy: -7, hideInactive: true}, 34, 37, false},
		{"백그라운드 방향키를 가끔 놓침 → 다시 눌러 맞춤", spiritSim{possessed: true, x: 30, y: 40, dropEvery: 3}, 34, 37, false},
		{"목표 칸이 벽 → 한 칸 당겨서", spiritSim{possessed: true, x: 30, y: 37, walls: map[[2]int]bool{{34, 37}: true}}, 33, 37, false},
		// 대비: 게임이 백그라운드 키를 안 받는다면(창을 앞으로 가져오면 커서가 캐릭터 칸에서 시작) 포그라운드로
		{"백그라운드 안 먹음 → 포그라운드", spiritSim{possessed: true, x: 30, y: 40, cdx: -2, cdy: -7, bgIgnored: true, hideInactive: true, resetOnActivate: true}, 34, 37, true},
		{"백그라운드 안 먹음 + 커서가 남은 자리 그대로", spiritSim{possessed: true, x: 30, y: 40, cdx: -2, cdy: -7, bgIgnored: true}, 34, 37, true},
		{"빙의 안 됨 + 백그라운드 안 먹음", spiritSim{x: 38, y: 37, bgIgnored: true, hideInactive: true}, 34, 37, true},
		{"백그라운드 안 먹고 포그라운드도 없음 → 그대로", spiritSim{possessed: true, x: 30, y: 37, bgIgnored: true, noFG: true}, 30, 37, false},
	}
	for _, c := range cases {
		s := c.sim
		SpiritMoveTo(s.io(), 34, 37, "5", "테스트")
		if s.x != c.wantX || s.y != c.wantY {
			t.Errorf("%s: 위치 (%d,%d), want (%d,%d) keys=%v", c.name, s.x, s.y, c.wantX, c.wantY, s.keys)
		}
		if !c.sim.noFG && (s.cdx != 0 || s.cdy != 0) {
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

// 창별 기억: 백그라운드가 안 먹는 창은 다음 이동부터 바로 포그라운드, 이동이 안 되면 잠깐 쉼
func TestSpiritMoveMode(t *testing.T) {
	mode := &SpiritMode{}
	s := spiritSim{possessed: true, x: 30, y: 40, bgIgnored: true, hideInactive: true, resetOnActivate: true}
	io := s.io()
	io.Mode = mode
	SpiritMoveTo(io, 34, 37, "5", "테스트")
	if s.x != 34 || s.y != 37 || !mode.ForegroundOnly() {
		t.Fatalf("첫 이동: (%d,%d) 포그라운드 기억=%v keys=%v", s.x, s.y, mode.ForegroundOnly(), s.keys)
	}
	s.keys, s.active = nil, false
	SpiritMoveTo(io, 30, 37, "5", "테스트")
	if s.x != 30 || s.y != 37 {
		t.Fatalf("두 번째 이동: (%d,%d) keys=%v", s.x, s.y, s.keys)
	}
	for _, k := range s.keys {
		if !strings.HasPrefix(k, "fg:") {
			t.Fatalf("기억한 창인데 백그라운드 키를 누름: %v", s.keys)
		}
	}

	// 사방이 벽 → 이동 실패 → 10초 안엔 다시 안 함
	walls := map[[2]int]bool{}
	for dx := -8; dx <= 8; dx++ {
		for dy := -7; dy <= 7; dy++ {
			walls[[2]int{30 + dx, 37 + dy}] = true
		}
	}
	s2 := spiritSim{possessed: true, x: 30, y: 37, walls: walls}
	io2 := s2.io()
	io2.Mode = &SpiritMode{}
	SpiritMoveTo(io2, 34, 37, "5", "테스트")
	if s2.x != 30 || s2.cdx != 0 || s2.cdy != 0 {
		t.Fatalf("벽: (%d,%d) 커서 %+d,%+d", s2.x, s2.y, s2.cdx, s2.cdy)
	}
	s2.keys = nil
	if SpiritMoveTo(io2, 34, 37, "5", "테스트") || len(s2.keys) != 0 {
		t.Errorf("실패 직후 다시 누름: %v", s2.keys)
	}
}

// '영술사 테스트' 진단 — 백그라운드 방향키 한 칸씩 확인 → 사냥 때와 같은 이동으로 오른쪽 3칸 → 제자리
func TestSpiritDiagnose(t *testing.T) {
	has := func(lines []string, sub string) bool {
		for _, l := range lines {
			if strings.Contains(l, sub) {
				return true
			}
		}
		return false
	}
	s := spiritSim{possessed: true, x: 30, y: 37, hideInactive: true}
	io := s.io()
	saved := map[string]int{}
	io.Save = func(name string, _ *image.RGBA) { saved[name]++ }
	lines := SpiritDiagnose(io)
	if !has(lines, "백그라운드 방향키: 커서가 움직임") || !has(lines, "오른쪽3칸: (30,37) → (33,37)") ||
		!has(lines, "제자리: (33,37) → (30,37)") || !has(lines, "결론: 백그라운드로 이동됨") {
		t.Errorf("백그라운드 정상: %v", lines)
	}
	if s.x != 30 || s.cdx != 0 || s.cdy != 0 || s.fgUsed {
		t.Errorf("진단 뒤 제자리·커서 캐릭터 칸·백그라운드만: x=%d 커서 %+d,%+d fg=%v", s.x, s.cdx, s.cdy, s.fgUsed)
	}
	if saved["시작"] == 0 || saved["백그라운드_오른쪽"] == 0 || saved["오른쪽3칸"] == 0 || saved["제자리"] == 0 {
		t.Errorf("단계별 화면 저장: %v", saved)
	}

	// 대비: 백그라운드 키를 안 받고, 비활성이면 커서를 안 그리고, 창을 가져오면 커서가 캐릭터 칸으로 가는 게임이라면
	s = spiritSim{possessed: true, x: 46, y: 12, cdx: -2, cdy: -7, bgIgnored: true, hideInactive: true, resetOnActivate: true}
	lines = SpiritDiagnose(s.io())
	if !has(lines, "백그라운드 → 1칸 → 커서 캐릭터에서 -2,-7 칸") || !has(lines, "게임이 백그라운드 방향키를 안 받음") ||
		!has(lines, "오른쪽3칸: (46,12) → (49,12)") || !has(lines, "제자리: (49,12) → (46,12)") ||
		!has(lines, "결론: 백그라운드 키는 빙의 커서가 안 받음") {
		t.Errorf("백그라운드 안 먹음: %v", lines)
	}
	if s.x != 46 || s.cdx != 0 || s.cdy != 0 {
		t.Errorf("진단 뒤 제자리·커서 캐릭터 칸: x=%d 커서 %+d,%+d", s.x, s.cdx, s.cdy)
	}

	s = spiritSim{x: 30, y: 37}
	if lines = SpiritDiagnose(s.io()); !has(lines, "파란 네모가 안 보임") {
		t.Errorf("빙의 안 됨: %v", lines)
	}
}
