package automation

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/go-vgo/robotgo"
)

// 영술사 빙의 이동 (사용자 2026-10-06).
// 영술사는 빙의:도깨비불(기본 5번)을 쓰면 걸을 수 없다. 대신 화면에 캐릭터를 가운데 둔 17x15칸(한 칸 48px)
// 파란 테두리와 분홍 커서 칸이 뜨고, 방향키는 커서를 움직이며 Q를 누르면 커서 칸으로 간다(빨간 칸 = 벽).
// 스킬은 커서 기준이라 옮긴 뒤엔 커서를 꼭 캐릭터 칸으로 되돌린다. 커서가 캐릭터 칸에 있으면
// 캐릭터 그림에 가려 안 보인다.

const (
	spiritCell  = 48
	spiritHalfW = 8 // 가운데 칸에서 좌우 8칸 (17칸)
	spiritHalfH = 7 // 위아래 7칸 (15칸)
	// 테두리 바깥선에서 가운데 칸 중심까지 (8.5칸, 7.5칸)
	spiritCenterOffX = spiritHalfW*spiritCell + spiritCell/2
	spiritCenterOffY = spiritHalfH*spiritCell + spiritCell/2
	spiritKeyGap     = 120 * time.Millisecond // 커서 방향키 사이 (빠르면 일부 키를 놓칠 수 있어 넉넉히)
)

// SpiritView 빙의 화면 읽기 결과
type SpiritView struct {
	Grid   bool // 파란 테두리를 찾음 (빙의 중)
	Cursor bool // 커서가 보임 (안 보이면 캐릭터 칸에 가려진 것 = 캐릭터 칸)
	DX, DY int  // 커서 - 캐릭터 (칸)
	CX, CY int  // 캐릭터 칸 중심 (화면 픽셀)
}

func isSpiritBlue(c color.RGBA) bool {
	return c.R <= 12 && c.G <= 12 && c.B >= 160 && c.B <= 185
}

func isSpiritPink(c color.RGBA) bool {
	return c.R >= 243 && c.G >= 155 && c.G <= 175 && c.B >= 155 && c.B <= 175 && int(c.R)-int(c.G) >= 70
}

// DetectSpiritView r(클라이언트 영역) 안에서 파란 테두리와 커서를 찾는다.
// 아래쪽 테두리는 채팅창에 가려 안 보일 때가 많아 위쪽 테두리를 기준으로 삼는다.
func DetectSpiritView(img image.Image, r image.Rectangle) SpiritView {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return SpiritView{}
	}
	at := expPixelFunc(img)

	// 위쪽 테두리: 파란 픽셀이 가로로 300px 넘게 이어진 맨 위 줄 — 그 구간의 시작/끝이 좌우 바깥선
	top, runStart, runEnd := -1, 0, 0
	for y := r.Min.Y; y < r.Max.Y && top < 0; y++ {
		run, bestLen, bestEnd := 0, 0, 0
		for x := r.Min.X; x < r.Max.X; x++ {
			if isSpiritBlue(at(x, y)) {
				if run++; run > bestLen {
					bestLen, bestEnd = run, x
				}
			} else {
				run = 0
			}
		}
		if bestLen >= 300 {
			top, runStart, runEnd = y, bestEnd-bestLen+1, bestEnd
		}
	}
	if top < 0 {
		return SpiritView{}
	}
	fullW := 2*spiritHalfW*spiritCell + spiritCell + 1 // 817 = 17칸 + 바깥선 1px
	var cx int
	switch {
	case runEnd-runStart+1 >= fullW-4: // 한 줄이 다 보임
		cx = runStart + spiritCenterOffX
	case runStart > r.Min.X+2: // 오른쪽이 화면 밖 — 왼쪽 끝 기준
		cx = runStart + spiritCenterOffX
	case runEnd < r.Max.X-3: // 왼쪽이 화면 밖 — 오른쪽 끝 기준
		cx = runEnd - spiritCenterOffX
	default:
		return SpiritView{}
	}
	cy := top + spiritCenterOffY
	v := SpiritView{Grid: true, CX: cx, CY: cy}

	// 커서: 테두리 안 분홍 픽셀을 칸별로 세어 가장 많은 칸 (가려지면 덜 보이므로 넉넉히 250개)
	counts := map[[2]int]int{}
	gx0, gy0 := cx-spiritCenterOffX, cy-spiritCenterOffY
	gx1, gy1 := gx0+fullW, gy0+2*spiritHalfH*spiritCell+spiritCell+1
	for y := maxInt(gy0, r.Min.Y); y < minInt(gy1, r.Max.Y); y++ {
		for x := maxInt(gx0, r.Min.X); x < minInt(gx1, r.Max.X); x++ {
			if !isSpiritPink(at(x, y)) {
				continue
			}
			cell := [2]int{floorDiv(x-(cx-spiritCell/2), spiritCell), floorDiv(y-(cy-spiritCell/2), spiritCell)}
			counts[cell]++
		}
	}
	best, bestN := [2]int{}, 0
	for cell, n := range counts {
		if n > bestN {
			best, bestN = cell, n
		}
	}
	if bestN >= 100 { // 캐릭터·빙의 오라에 일부 가려져도 찾게 넉넉히(커서 칸이 다 보이면 1400개쯤)
		v.Cursor, v.DX, v.DY = true, best[0], best[1]
	}
	return v
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SpiritIO 영술사 이동에 쓰는 창 입출력 (실제 창은 NewSpiritIO — 테스트는 가짜로 채운다)
type SpiritIO struct {
	Capture func() (*image.RGBA, error)               // 창을 띄우지 않는 캡처
	Region  func(img *image.RGBA) image.Rectangle     // 클라이언트(게임 화면) 영역
	Coords  func(img *image.RGBA) (GameCoords, error) // 우하단 HUD 좌표
	Tap     func(key string)                          // 키 1회 — 백그라운드(PostMessage, 창을 안 띄움)
	// TapFG 포그라운드로 누르기(창을 앞으로 가져와 robotgo) — 백그라운드 키로 커서·Q가 안 먹을 때만 쓴다. nil 이면 안 씀.
	TapFG func(key string)
	Sleep func(d time.Duration) bool // 중지되면 false
	Log   func(msg string)
	// Save 진단(영술사 테스트) 때 단계별 화면을 남긴다 — nil 이면 안 남김
	Save func(name string, img *image.RGBA)
}

// NewSpiritIO 실제 창용 입출력 — 키는 먼저 백그라운드로 누르고, 커서·Q가 안 먹을 때만 창을 앞으로 가져와 누른다.
func NewSpiritIO(wm *WindowManager, om *OCRManager, hwnd uint64, sleep func(time.Duration) bool, logf func(string)) SpiritIO {
	bgErrLogged := false
	return SpiritIO{
		Capture: func() (*image.RGBA, error) { return wm.CaptureWindowQuiet(hwnd) },
		Region: func(img *image.RGBA) image.Rectangle {
			x, y, w, h := om.clientBox(img, hwnd)
			return image.Rect(x, y, x+w, y+h)
		},
		Coords: func(img *image.RGBA) (GameCoords, error) {
			c, _, err := om.ReadCoordinatesFromImage(img)
			return c, err
		},
		Tap: func(k string) {
			if err := BgKeyTap(hwnd, k); err != nil && !bgErrLogged {
				bgErrLogged = true
				logf(fmt.Sprintf("[영술사] 백그라운드 키 '%s' 입력 실패: %v", k, err))
			}
		},
		TapFG: func(k string) {
			if wm.GetForegroundWindow() != hwnd {
				wm.ActivateWindow(hwnd)
				time.Sleep(300 * time.Millisecond)
			}
			robotgo.KeyTap(k)
		},
		Sleep: sleep,
		Log:   logf,
	}
}

// SpiritMoveTo 영술사를 (tx,ty) 로 옮긴다(±1 이면 그대로). 빙의 중이 아니면 possessKey(빙의:도깨비불)부터 누른다.
// 방향키로 커서를 목표 칸까지 옮기고 Q → 좌표가 바뀌었으면 성공. 커서는 캐릭터 가까이(빙의 오라 — 좌우 약 2칸,
// 위로 약 4칸)에선 가려져 안 보이므로 성공 여부는 좌표로 본다(사용자 2026-10-07: 커서는 움직이는데 테스트가
// 못 봄). 백그라운드로 안 되면 그때만 창을 앞으로 가져와 다시 한다. 키를 눌렀으면 true.
func SpiritMoveTo(io SpiritIO, tx, ty int, possessKey, who string) bool {
	img, err := io.Capture()
	if err != nil {
		return false
	}
	c, err := io.Coords(img)
	if err != nil || (c.X == 0 && c.Y == 0) {
		io.Log(fmt.Sprintf("[영술사] %s 좌표를 못 읽어 이동 생략", who))
		return false
	}
	dx, dy := tx-c.X, ty-c.Y
	if meAbs(dx) <= 1 && meAbs(dy) <= 1 {
		return false
	}
	v := DetectSpiritView(img, io.Region(img))
	if !v.Grid {
		key := SpiritKeyOr(possessKey)
		io.Log(fmt.Sprintf("[영술사] %s 빙의 칸(파란 네모)이 안 보임 — %s(빙의:도깨비불) 누르고 이동", who, key))
		io.Tap(key)
		if !io.Sleep(1500 * time.Millisecond) {
			return true
		}
		var ok bool
		if v, ok = spiritLook(io); (!ok || !v.Grid) && io.TapFG != nil {
			io.Log(fmt.Sprintf("[영술사] %s 그래도 안 보여 창을 앞으로 가져와 %s 다시", who, key))
			io.TapFG(key)
			if !io.Sleep(1500 * time.Millisecond) {
				return true
			}
			v, ok = spiritLook(io)
		}
		if !ok || !v.Grid {
			io.Log(fmt.Sprintf("[영술사] %s 빙의 칸이 안 보여 이동 생략", who))
			return true
		}
	}
	// 한 번에 갈 수 있는 건 네모 안(좌우 8칸, 위아래 7칸) — 더 멀면 다음 점검 때 이어서 간다
	mx, my := spiritClamp(dx, spiritHalfW), spiritClamp(dy, spiritHalfH)
	io.Log(fmt.Sprintf("[영술사] %s (%d,%d) → (%d,%d): 커서 %+d,%+d 칸 → Q", who, c.X, c.Y, tx, ty, mx, my))
	// 지금 커서 자리 (안 보이면 캐릭터 칸에 있는 것)
	cx, cy := 0, 0
	if v.Cursor {
		cx, cy = v.DX, v.DY
	}

	// 1) 백그라운드: 방향키 → Q → 좌표 확인
	if !spiritArrows(io, io.Tap, mx-cx, my-cy) {
		return true
	}
	io.Tap("q")
	if !io.Sleep(900 * time.Millisecond) {
		return true
	}
	if c2, moved := spiritMovedFrom(io, c); moved {
		io.Log(fmt.Sprintf("[영술사] %s 이동 완료 → (%d,%d)", who, c2.X, c2.Y))
		spiritCursorHome(io, io.Tap, who)
		return true
	}
	if io.TapFG == nil {
		io.Log(fmt.Sprintf("[영술사] %s Q를 눌렀는데 그대로 (%d,%d) — 커서 칸이 벽이거나 키가 안 먹음", who, c.X, c.Y))
		spiritCursorHome(io, io.Tap, who)
		return true
	}
	// 2) 그대로인데 커서가 목표 칸에 보인다 — 방향키는 먹었다. Q 만 포그라운드로 한 번 더(그래도 그대로면 벽)
	if now := spiritLookCursor(io); now.Cursor && now.DX == mx && now.DY == my {
		io.Log(fmt.Sprintf("[영술사] %s 커서는 목표 칸인데 그대로 — 창을 앞으로 가져와 Q 다시", who))
		io.TapFG("q")
		if !io.Sleep(900 * time.Millisecond) {
			return true
		}
		if c2, moved := spiritMovedFrom(io, c); moved {
			io.Log(fmt.Sprintf("[영술사] %s 이동 완료 → (%d,%d) (포그라운드 Q)", who, c2.X, c2.Y))
		} else {
			io.Log(fmt.Sprintf("[영술사] %s 그래도 그대로 — 커서 칸이 벽(빨간 칸)인 것 같음", who))
		}
		spiritCursorHome(io, io.Tap, who)
		return true
	}
	// 3) 커서가 목표 칸에 안 보인다 — 백그라운드 방향키가 안 먹었거나, 캐릭터 가까이라 가려진 것.
	//    백그라운드로 거꾸로 눌러(먹었다면 원위치, 안 먹었다면 그대로) 커서를 처음 자리에 두고 포그라운드로 다시.
	io.Log(fmt.Sprintf("[영술사] %s 백그라운드로는 안 움직임 — 창을 앞으로 가져와 다시", who))
	if !spiritArrows(io, io.Tap, cx-mx, cy-my) || !io.Sleep(200*time.Millisecond) {
		return true
	}
	if !spiritArrows(io, io.TapFG, mx-cx, my-cy) {
		return true
	}
	io.TapFG("q")
	if !io.Sleep(900 * time.Millisecond) {
		return true
	}
	if c2, moved := spiritMovedFrom(io, c); moved {
		io.Log(fmt.Sprintf("[영술사] %s 이동 완료 → (%d,%d) (포그라운드 입력)", who, c2.X, c2.Y))
	} else {
		io.Log(fmt.Sprintf("[영술사] %s 포그라운드로도 그대로 (%d,%d) — 커서 칸이 벽이거나 커서가 키를 안 받음", who, c.X, c.Y))
	}
	spiritCursorHome(io, io.TapFG, who)
	return true
}

// spiritLook 지금 화면의 빙의 상태
func spiritLook(io SpiritIO) (SpiritView, bool) {
	img, err := io.Capture()
	if err != nil {
		return SpiritView{}, false
	}
	return DetectSpiritView(img, io.Region(img)), true
}

// spiritCursorHome 커서를 캐릭터 칸으로 되돌린다(스킬이 커서 자리에 나가므로) — 보이는 동안 두 번까지.
// 안 보이면 캐릭터 칸(또는 가까이에 가려짐)으로 보고 그대로 둔다.
func spiritCursorHome(io SpiritIO, tap func(string), who string) {
	for try := 0; try < 2; try++ {
		v, ok := spiritLook(io)
		if !ok || !v.Grid || !v.Cursor || (v.DX == 0 && v.DY == 0) {
			return
		}
		io.Log(fmt.Sprintf("[영술사] %s 커서가 캐릭터에서 %+d,%+d 칸 — 되돌림", who, v.DX, v.DY))
		if !spiritArrows(io, tap, -v.DX, -v.DY) || !io.Sleep(300*time.Millisecond) {
			return
		}
	}
}

// spiritMovedFrom 좌표가 c0 에서 바뀌었는지 (지금 좌표도 돌려준다)
func spiritMovedFrom(io SpiritIO, c0 GameCoords) (GameCoords, bool) {
	img, err := io.Capture()
	if err != nil {
		return c0, false
	}
	c, err := io.Coords(img)
	if err != nil || (c.X == 0 && c.Y == 0) {
		return c0, false
	}
	return c, c.X != c0.X || c.Y != c0.Y
}

// SpiritDiagnose '영술사 테스트' — 이동이 왜 안 되는지 단계별로 본다. 커서를 오른쪽 4칸(빙의 오라 밖)으로
// 옮기고 **커서가 보이든 말든 Q 를 눌러** 좌표가 바뀌는지 본다 — 커서를 못 보는 건지 Q 가 안 먹는 건지 가른다.
// 백그라운드로 안 되면 포그라운드로 같은 걸 한다. 단계마다 찍은 화면은 io.Save 로 남긴다(사용자 2026-10-07:
// 커서는 움직이는데 Q 를 안 누름 — 봇이 실제로 본 화면이 필요).
func SpiritDiagnose(io SpiritIO) []string {
	const probe = 4
	var out []string
	add := func(format string, a ...interface{}) { out = append(out, fmt.Sprintf(format, a...)) }
	save := func(name string) (SpiritView, bool) {
		img, err := io.Capture()
		if err != nil {
			return SpiritView{}, false
		}
		if io.Save != nil {
			io.Save(name, img)
		}
		return DetectSpiritView(img, io.Region(img)), true
	}
	img, err := io.Capture()
	if err != nil {
		add("화면을 못 읽음: %v (창이 최소화돼 있으면 안 됩니다)", err)
		return out
	}
	if io.Save != nil {
		io.Save("1_시작", img)
	}
	c, cerr := io.Coords(img)
	if cerr != nil || (c.X == 0 && c.Y == 0) {
		add("좌표를 못 읽음 (오른쪽 아래 X·Y 표시)")
	} else {
		add("좌표 (%d,%d)", c.X, c.Y)
	}
	v := DetectSpiritView(img, io.Region(img))
	if !v.Grid {
		add("파란 네모가 안 보임 — 빙의 상태가 아니거나 화면에서 못 찾음. 빙의:도깨비불을 쓴 뒤 다시 눌러 주세요")
		return out
	}
	add("파란 네모 찾음 — 커서 %s", spiritCursorText(v))

	// try 한 가지 입력 방식으로: 오른쪽 4칸 → (화면 저장·커서 확인) → Q → 좌표. 이동했으면 왼쪽 4칸 + Q 로 제자리,
	// 아니면 왼쪽 4칸으로 커서만 원위치. 이동했으면 true.
	try := func(name, tag string, tap func(string)) bool {
		spiritArrows(io, tap, probe, 0)
		io.Sleep(400 * time.Millisecond)
		after, _ := save(tag + "_오른쪽4칸")
		if !after.Cursor {
			after = spiritLookCursor(io)
		}
		add("%s 방향키(→ %d칸) → 커서 %s", name, probe, spiritCursorText(after))
		tap("q")
		io.Sleep(1000 * time.Millisecond)
		save(tag + "_Q")
		c2, moved := spiritMovedFrom(io, c)
		if !moved {
			add("%s Q → 좌표 그대로", name)
			spiritArrows(io, tap, -probe, 0)
			io.Sleep(300 * time.Millisecond)
			return false
		}
		add("%s Q → 좌표 (%d,%d) — 이동함", name, c2.X, c2.Y)
		spiritArrows(io, tap, -probe, 0)
		io.Sleep(300 * time.Millisecond)
		tap("q")
		io.Sleep(1000 * time.Millisecond)
		if c3, back := spiritMovedFrom(io, c2); back {
			add("%s 제자리로 Q → 좌표 (%d,%d)", name, c3.X, c3.Y)
		}
		spiritCursorHome(io, tap, "테스트")
		return true
	}
	if try("백그라운드", "2_백그라운드", io.Tap) {
		return out
	}
	if io.TapFG != nil && try("포그라운드", "3_포그라운드", io.TapFG) {
		return out
	}
	add("방향키 + Q 로 이동이 안 됨 — 저장된 화면을 보내 주세요(오른쪽 4칸 커서가 찍혔는지, Q 뒤 모습)")
	return out
}

// spiritLookCursor 커서를 찾을 때 몇 번 다시 본다(깜빡이거나 그리는 중일 수 있어 0.12초 간격 3번)
func spiritLookCursor(io SpiritIO) SpiritView {
	var v SpiritView
	for i := 0; i < 3; i++ {
		if i > 0 && !io.Sleep(120*time.Millisecond) {
			break
		}
		if cur, ok := spiritLook(io); ok {
			v = cur
			if cur.Cursor {
				break
			}
		}
	}
	return v
}

func spiritCursorText(v SpiritView) string {
	if !v.Cursor {
		return "캐릭터 칸(가려짐)"
	}
	return fmt.Sprintf("캐릭터에서 %+d,%+d 칸", v.DX, v.DY)
}

func spiritClamp(v, max int) int {
	if v > max {
		return max
	}
	if v < -max {
		return -max
	}
	return v
}

// spiritArrows 커서를 (ax, ay) 칸 움직인다 (오른쪽·아래가 +). 중지되면 false.
func spiritArrows(io SpiritIO, tap func(string), ax, ay int) bool {
	press := func(n int, pos, neg string) bool {
		key := pos
		if n < 0 {
			key, n = neg, -n
		}
		for i := 0; i < n; i++ {
			tap(key)
			if !io.Sleep(spiritKeyGap) {
				return false
			}
		}
		return true
	}
	return press(ax, "right", "left") && press(ay, "down", "up")
}

// SpiritKeyOr 빙의:도깨비불 키 — 비어 있으면 5 (사용자 마법창 기준)
func SpiritKeyOr(k string) string {
	if k = strings.TrimSpace(k); k == "" {
		return "5"
	}
	return k
}
