package automation

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-vgo/robotgo"
)

// 영술사 빙의 이동 (사용자 2026-10-06).
// 영술사는 빙의:도깨비불(기본 5번)을 쓰면 걸을 수 없다. 대신 화면에 캐릭터를 가운데 둔 17x15칸(한 칸 48px)
// 파란 테두리와 커서 칸이 뜨고, 방향키는 커서를 움직이며 Q를 누르면 커서 칸으로 간다(빨간 칸 = 벽).
// 스킬은 커서 기준이라 옮긴 뒤엔 커서를 꼭 캐릭터 칸으로 되돌린다.
//
// 커서(앱 캡처 실측 2026-10-07): 검·흰·검 2px 테두리 네모. 속은 바닥 무늬를 물들인 색이라 칸마다 다르다
// (갈 수 있는 칸은 초록, 캐릭터 칸·못 가는 칸은 빨강, 흰 바닥에선 분홍) → 테두리로 찾는다. 커서는 캐릭터 위에도
// 그려지지만 안 그려질 때가 있다(창이 비활성일 때 등). 또 커서는 전에 옮긴 자리에 남아 있을 수 있어(테스트에서
// 캐릭터 기준 -2,-7 칸) 자리를 짐작하지 않고 누를 때마다 화면으로 확인한다.

const (
	spiritCell  = 48
	spiritHalfW = 8 // 가운데 칸에서 좌우 8칸 (17칸)
	spiritHalfH = 7 // 위아래 7칸 (15칸)
	// 테두리 바깥선에서 가운데 칸 중심까지 (8.5칸, 7.5칸)
	spiritCenterOffX = spiritHalfW*spiritCell + spiritCell/2
	spiritCenterOffY = spiritHalfH*spiritCell + spiritCell/2
	spiritFullW      = 2*spiritHalfW*spiritCell + spiritCell + 1 // 817 = 17칸 + 바깥선 1px
	spiritKeyGap     = 120 * time.Millisecond                    // 커서 방향키 사이 (빠르면 일부 키를 놓칠 수 있어 넉넉히)
	spiritRingMin    = 0.6                                       // 커서 테두리로 볼 최소 일치 비율 (흰 줄·검은 줄 각각)
	spiritFailWait   = 10 * time.Second                          // 이동이 안 되면 이만큼 쉬었다 다시 (2초 점검마다 창을 끌어오지 않게)
)

// SpiritView 빙의 화면 읽기 결과
type SpiritView struct {
	Grid   bool   // 파란 테두리를 찾음 (빙의 중)
	Cursor bool   // 커서 테두리를 찾음 — 안 그려져 있으면 false
	DX, DY int    // 커서 - 캐릭터 (칸)
	CX, CY int    // 캐릭터 칸 중심 (화면 픽셀)
	Tint   string // 커서 칸 속 색: "초록"(갈 수 있는 칸) / "빨강"(캐릭터 칸·못 가는 칸) / "" — 기록용
}

func isSpiritBlue(c color.RGBA) bool {
	return c.R <= 12 && c.G <= 12 && c.B >= 160 && c.B <= 185
}

// DetectSpiritView r(클라이언트 영역) 안에서 파란 테두리와 커서를 찾는다.
func DetectSpiritView(img image.Image, r image.Rectangle) SpiritView {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return SpiritView{}
	}
	at := expPixelFunc(img)
	cx, cy, ok := spiritGrid(at, r)
	if !ok {
		return SpiritView{}
	}
	v := SpiritView{Grid: true, CX: cx, CY: cy}
	v.Cursor, v.DX, v.DY, v.Tint = spiritFindCursor(at, r, cx, cy)
	return v
}

// spiritGrid 파란 테두리에서 캐릭터 칸 중심을 찾는다. 아래쪽 테두리는 채팅창에 가려 안 보일 때가 많아 위쪽 테두리를
// 기준으로 삼는다. 위쪽 테두리 줄은 임무창·커서에 군데군데 가려지므로(실측: 296~300 | 임무창 | 551~1112) 가장 긴
// 한 토막이 아니라 그 줄의 파란 픽셀 전체를 보고, 한쪽 끝이 가려졌으면 세로 테두리가 이어지는 쪽 끝을 기준으로 한다.
func spiritGrid(at func(x, y int) color.RGBA, r image.Rectangle) (cx, cy int, ok bool) {
	top, lo, hi := -1, 0, 0
	for y := r.Min.Y; y < r.Max.Y && top < 0; y++ {
		n, l, h := 0, -1, -1
		for x := r.Min.X; x < r.Max.X; x++ {
			if isSpiritBlue(at(x, y)) {
				n++
				if l < 0 {
					l = x
				}
				h = x
			}
		}
		if n >= 200 && h-l+1 <= spiritFullW+2 {
			top, lo, hi = y, l, h
		}
	}
	if top < 0 {
		return 0, 0, false
	}
	// 세로 테두리(2px)가 위쪽 테두리 아래로 이어지는지
	vert := func(x int) bool {
		n, hit := 0, 0
		for y := top + 3; y < top+300 && y < r.Max.Y; y++ {
			n++
			if isSpiritBlue(at(x, y)) || (x+1 < r.Max.X && isSpiritBlue(at(x+1, y))) {
				hit++
			}
		}
		return n > 0 && hit*2 >= n
	}
	switch {
	case hi-lo+1 >= spiritFullW-2: // 양쪽 끝이 다 보임
		cx = lo + spiritCenterOffX
	case lo > r.Min.X+1 && vert(lo): // 왼쪽 끝 확인 (오른쪽이 화면 밖이거나 가려짐)
		cx = lo + spiritCenterOffX
	case hi < r.Max.X-2 && vert(hi-1): // 오른쪽 끝 확인
		cx = hi - spiritCenterOffX
	default:
		return 0, 0, false
	}
	return cx, top + spiritCenterOffY, true
}

// spiritRing 커서 테두리 (칸 왼쪽 위 기준 px, 실측): -1~0 검정, 1~2 흰색, 3~4 검정 … 43~44 검정, 45~46 흰색, 47~48 검정
var spiritRing = [...]struct {
	off   int
	white bool
}{{-1, false}, {0, false}, {1, true}, {2, true}, {3, false}, {4, false},
	{43, false}, {44, false}, {45, true}, {46, true}, {47, false}, {48, false}}

// spiritRingScore 왼쪽 위가 (x0,y0) 인 칸에 커서 테두리가 있는 정도 — 흰 줄·검은 줄 일치 비율 중 작은 쪽.
// 흰 바닥(흰 줄만 맞음)이나 어두운 곳(검은 줄만 맞음)은 낮게 나온다. 화면에 보이는 부분이 적으면 0.
func spiritRingScore(at func(x, y int) color.RGBA, r image.Rectangle, x0, y0 int) float64 {
	var wOK, wN, bOK, bN int
	test := func(x, y int, white bool) {
		if x < r.Min.X || y < r.Min.Y || x >= r.Max.X || y >= r.Max.Y {
			return
		}
		c := at(x, y)
		if white {
			wN++
			if c.R >= 220 && c.G >= 220 && c.B >= 220 {
				wOK++
			}
		} else {
			bN++
			if c.R <= 50 && c.G <= 50 && c.B <= 50 {
				bOK++
			}
		}
	}
	for t := 6; t <= 41; t++ { // 네 변 (모서리 빼고)
		for _, p := range spiritRing {
			test(x0+t, y0+p.off, p.white)
			test(x0+p.off, y0+t, p.white)
		}
	}
	if wN < 60 || bN < 120 {
		return 0
	}
	w, b := float64(wOK)/float64(wN), float64(bOK)/float64(bN)
	if b < w {
		return b
	}
	return w
}

// spiritFindCursor 네모 안 모든 칸에서 커서 테두리를 찾는다(1px 어긋남까지). 가장 잘 맞는 칸.
func spiritFindCursor(at func(x, y int) color.RGBA, r image.Rectangle, cx, cy int) (found bool, dx, dy int, tint string) {
	best, bx, by := 0.0, 0, 0
	for ddy := -spiritHalfH; ddy <= spiritHalfH; ddy++ {
		for ddx := -spiritHalfW; ddx <= spiritHalfW; ddx++ {
			x0, y0 := cx-spiritCell/2+ddx*spiritCell, cy-spiritCell/2+ddy*spiritCell
			for jy := -1; jy <= 1; jy++ {
				for jx := -1; jx <= 1; jx++ {
					if s := spiritRingScore(at, r, x0+jx, y0+jy); s > best {
						best, dx, dy, bx, by = s, ddx, ddy, x0+jx, y0+jy
					}
				}
			}
		}
	}
	if best < spiritRingMin {
		return false, 0, 0, ""
	}
	return true, dx, dy, spiritTint(at, r, bx, by)
}

// spiritTint 커서 칸 속 색 (테두리 안쪽 평균) — 갈 수 있는 칸은 초록, 캐릭터 칸·벽은 빨강(실측)
func spiritTint(at func(x, y int) color.RGBA, r image.Rectangle, x0, y0 int) string {
	var sr, sg, sb, n int
	for y := y0 + 8; y <= y0+39; y += 3 {
		for x := x0 + 8; x <= x0+39; x += 3 {
			if x < r.Min.X || y < r.Min.Y || x >= r.Max.X || y >= r.Max.Y {
				continue
			}
			c := at(x, y)
			sr, sg, sb, n = sr+int(c.R), sg+int(c.G), sb+int(c.B), n+1
		}
	}
	if n == 0 {
		return ""
	}
	R, G, B := sr/n, sg/n, sb/n
	switch {
	case G >= R+30 && G >= B+30:
		return "초록"
	case R >= G+40 && R >= B+30:
		return "빨강"
	}
	return ""
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
	Wake    func()                                    // 백그라운드 활성 위장만(키 없이) — 커서가 안 그려졌을 때. nil 이면 안 씀
	// Activate 창을 앞으로 가져오기, TapFG 포그라운드 키(robotgo) — 백그라운드 키로 커서가 안 움직일 때만 쓴다.
	// 둘 다 있어야 포그라운드를 쓴다.
	Activate func()
	TapFG    func(key string)
	Sleep    func(d time.Duration) bool // 중지되면 false
	Log      func(msg string)
	// Save 진단(영술사 테스트) 때 화면을 남긴다 — nil 이면 안 남김
	Save func(name string, img *image.RGBA)
	// Mode 창마다 기억하는 입력 방식 — nil 이면 이번 호출에서만 판단
	Mode *SpiritMode
}

// SpiritMode 창마다 "백그라운드 키로는 커서가 안 움직인다"를 기억한다 — 한 번 확인되면 다음부터 바로 포그라운드.
// 이동이 안 됐으면 잠깐 쉰다(2초 점검마다 창을 끌어오지 않게).
type SpiritMode struct {
	fg        atomic.Bool
	failUntil atomic.Int64 // UnixNano
}

// ForegroundOnly 이 창은 포그라운드로만 커서가 움직이는지
func (m *SpiritMode) ForegroundOnly() bool { return m != nil && m.fg.Load() }

var spiritModes sync.Map // hwnd → *SpiritMode

// SpiritModeFor 창별 입력 방식 기억 (같은 창이면 같은 값)
func SpiritModeFor(hwnd uint64) *SpiritMode {
	m, _ := spiritModes.LoadOrStore(hwnd, &SpiritMode{})
	return m.(*SpiritMode)
}

// NewSpiritIO 실제 창용 입출력 — 키는 먼저 백그라운드로 누르고, 커서가 안 움직일 때만 창을 앞으로 가져와 누른다.
func NewSpiritIO(wm *WindowManager, om *OCRManager, hwnd uint64, sleep func(time.Duration) bool, logf func(string)) SpiritIO {
	bgErrLogged := false
	activate := func() {
		if wm.GetForegroundWindow() != hwnd {
			wm.ActivateWindow(hwnd)
			time.Sleep(350 * time.Millisecond)
		}
	}
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
		Wake:     func() { BgSpoofActive(hwnd) },
		Activate: activate,
		TapFG: func(k string) {
			activate() // 그새 다른 창을 눌렀으면 다시 앞으로
			robotgo.KeyTap(k)
		},
		Sleep: sleep,
		Log:   logf,
		Mode:  SpiritModeFor(hwnd),
	}
}

// SpiritMoveTo 영술사를 (tx,ty) 로 옮긴다(±1 이면 그대로). 빙의 중이 아니면 possessKey(빙의:도깨비불)부터 누른다.
// 커서는 누르고 → 화면에서 자리를 확인하고 → 모자라면 더 누르는 식으로 목표 칸에 맞춘 뒤 Q, 성공은 좌표로 본다.
// 백그라운드 키에 커서가 안 움직이면(화면으로 확인) 창을 앞으로 가져와 누르고, 그 창은 다음부터 바로 그렇게 한다.
// 목표 칸이 못 가는 칸이면 캐릭터 쪽으로 한 칸씩 당겨 다시. 끝나면 커서를 캐릭터 칸에 둔다. 키를 눌렀으면 true.
func SpiritMoveTo(io SpiritIO, tx, ty int, possessKey, who string) bool {
	mode := io.Mode
	if mode == nil {
		mode = &SpiritMode{}
	}
	if time.Now().UnixNano() < mode.failUntil.Load() {
		return false // 방금 이동이 안 됨 — 잠깐 쉼
	}
	canFG := io.Activate != nil && io.TapFG != nil
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
	if v := DetectSpiritView(img, io.Region(img)); !v.Grid {
		if _, ok, stopped := spiritPossess(io, canFG, possessKey, who); stopped {
			return true
		} else if !ok {
			io.Log(fmt.Sprintf("[영술사] %s 빙의 칸이 안 보여 이동 생략", who))
			mode.failUntil.Store(time.Now().Add(spiritFailWait).UnixNano())
			return true
		}
	}
	// 한 번에 갈 수 있는 건 네모 안(좌우 8칸, 위아래 7칸) — 더 멀면 다음 점검 때 이어서 간다
	mx, my := spiritClamp(dx, spiritHalfW), spiritClamp(dy, spiritHalfH)
	io.Log(fmt.Sprintf("[영술사] %s (%d,%d) → (%d,%d): 커서 %+d,%+d 칸 → Q", who, c.X, c.Y, tx, ty, mx, my))

	fg := canFG && mode.fg.Load()
	bgReached := false // 백그라운드로 커서를 목표 칸에 맞췄는데 Q 뒤 그대로였던 적이 있음
	targets := spiritTargets(mx, my)
	for i := 0; i < len(targets); i++ {
		t := targets[i]
		h := spiritHop(io, fg, c, t[0], t[1])
		switch {
		case h.stopped:
			return true
		case h.moved:
			if fg && !mode.fg.Load() {
				mode.fg.Store(true) // 백그라운드로는 안 되던 걸 포그라운드로 함 — 이 창은 다음부터 바로
			}
			mode.failUntil.Store(0)
			io.Log(fmt.Sprintf("[영술사] %s 이동 완료 → (%d,%d)%s", who, h.now.X, h.now.Y, spiritHowText(fg)))
			spiritCursorHome(io, fg, who)
			return true
		}
		if !fg && canFG && (h.stuck || !h.reached) {
			if h.stuck {
				io.Log(fmt.Sprintf("[영술사] %s 백그라운드 방향키에 커서가 안 움직임(%s) — 창을 앞으로 가져와 다시", who, spiritCursorText(h.view)))
				mode.fg.Store(true)
			} else {
				io.Log(fmt.Sprintf("[영술사] %s 백그라운드로는 그대로(커서 %s) — 창을 앞으로 가져와 다시", who, spiritCursorText(h.view)))
			}
			fg = true
			i-- // 같은 칸을 포그라운드로 다시
			continue
		}
		if h.reached {
			if !fg {
				bgReached = true
			}
			io.Log(fmt.Sprintf("[영술사] %s 커서는 %+d,%+d 칸(%s)인데 Q 뒤 그대로 — 못 가는 칸 같음", who, t[0], t[1], spiritTintText(h.view.Tint)))
		}
		if i+1 < len(targets) {
			n := targets[i+1]
			io.Log(fmt.Sprintf("[영술사] %s 캐릭터 쪽으로 당겨 %+d,%+d 칸으로 다시", who, n[0], n[1]))
		}
	}
	// 백그라운드로 커서는 맞췄는데 Q 만 안 먹었을 수도 — 마지막으로 포그라운드로 첫 칸 한 번
	if !fg && canFG && bgReached {
		io.Log(fmt.Sprintf("[영술사] %s 마지막으로 창을 앞으로 가져와 %+d,%+d 칸 다시", who, targets[0][0], targets[0][1]))
		if h := spiritHop(io, true, c, targets[0][0], targets[0][1]); h.stopped {
			return true
		} else if h.moved {
			mode.fg.Store(true)
			mode.failUntil.Store(0)
			io.Log(fmt.Sprintf("[영술사] %s 이동 완료 → (%d,%d)%s", who, h.now.X, h.now.Y, spiritHowText(true)))
			spiritCursorHome(io, true, who)
			return true
		}
		fg = true
	}
	io.Log(fmt.Sprintf("[영술사] %s Q를 눌러도 그대로 (%d,%d) — 커서 칸이 벽이거나 키가 안 먹음, %d초 뒤 다시", who, c.X, c.Y, int(spiritFailWait/time.Second)))
	mode.failUntil.Store(time.Now().Add(spiritFailWait).UnixNano())
	spiritCursorHome(io, fg, who)
	return true
}

// spiritPossess 빙의 칸이 안 보일 때 빙의 키(백그라운드 → 그래도 안 보이면 포그라운드)
func spiritPossess(io SpiritIO, canFG bool, possessKey, who string) (v SpiritView, ok, stopped bool) {
	key := SpiritKeyOr(possessKey)
	io.Log(fmt.Sprintf("[영술사] %s 빙의 칸(파란 네모)이 안 보임 — %s(빙의:도깨비불) 누르고 이동", who, key))
	press := func(fg bool) bool {
		if fg {
			io.Activate()
			io.TapFG(key)
		} else {
			io.Tap(key)
		}
		if !io.Sleep(1500 * time.Millisecond) {
			return false
		}
		v, ok = spiritLook(io)
		ok = ok && v.Grid
		return true
	}
	if !press(false) {
		return v, false, true
	}
	if !ok && canFG {
		io.Log(fmt.Sprintf("[영술사] %s 그래도 안 보여 창을 앞으로 가져와 %s 다시", who, key))
		if !press(true) {
			return v, false, true
		}
	}
	return v, ok, false
}

// spiritTargets 갈 칸 후보: 목표 칸, 못 가면 캐릭터 쪽으로 1칸·2칸 당긴 칸
func spiritTargets(mx, my int) [][2]int {
	pull := func(v, k int) int {
		switch {
		case v > k:
			return v - k
		case v < -k:
			return v + k
		}
		return 0
	}
	out := [][2]int{{mx, my}}
	for k := 1; k <= 2; k++ {
		n := [2]int{pull(mx, k), pull(my, k)}
		if n == [2]int{0, 0} {
			break
		}
		if n != out[len(out)-1] {
			out = append(out, n)
		}
	}
	return out
}

type spiritHopResult struct {
	moved   bool       // Q 뒤 좌표가 바뀜
	now     GameCoords // 바뀐 좌표
	reached bool       // Q 전에 커서가 목표 칸에 있는 걸 화면으로 확인함
	stuck   bool       // 커서가 보이는데 방향키를 눌러도 그대로 — 이 입력 방식은 커서를 못 움직임
	view    SpiritView // 마지막으로 본 빙의 화면
	stopped bool
}

// spiritHop 커서를 (mx,my) 칸에 맞추고(화면으로 확인하며) Q 한 번 — 백그라운드 또는 포그라운드
func spiritHop(io SpiritIO, fg bool, c0 GameCoords, mx, my int) spiritHopResult {
	tap := io.Tap
	if fg {
		tap = io.TapFG
		io.Activate()
	}
	cur, ok := spiritSee(io, !fg)
	if !ok || !cur.Grid {
		return spiritHopResult{view: cur}
	}
	st := spiritSteer(io, tap, !fg, cur, mx, my)
	h := spiritHopResult{reached: st.reached, stuck: st.stuck, view: st.view, stopped: st.stopped}
	if st.stopped || st.stuck {
		return h
	}
	tap("q")
	if !io.Sleep(900 * time.Millisecond) {
		h.stopped = true
		return h
	}
	h.now, h.moved = spiritMovedFrom(io, c0)
	return h
}

type spiritSteerResult struct {
	view    SpiritView // 마지막으로 본 화면
	reached bool       // 커서가 목표 칸에 있는 걸 확인함
	stuck   bool       // 커서가 보이는데 눌러도 그대로
	stopped bool
}

// spiritSteer 커서를 (tx,ty) 칸으로: 남은 만큼 누르고 → 화면으로 확인 → 모자라면 더 누르기를 3번까지.
// cur 는 지금 화면 — 커서가 안 보이면 캐릭터 칸에 있다고 보고 시작한다. 누른 뒤에도 안 보이면 누른 만큼 갔다고 본다.
func spiritSteer(io SpiritIO, tap func(string), wake bool, cur SpiritView, tx, ty int) spiritSteerResult {
	res := spiritSteerResult{view: cur}
	px, py, known := 0, 0, cur.Cursor
	if known {
		px, py = cur.DX, cur.DY
	}
	for round := 0; round < 3; round++ {
		if known && px == tx && py == ty {
			res.reached = true
			return res
		}
		if !spiritArrows(io, tap, tx-px, ty-py) || !io.Sleep(250*time.Millisecond) {
			res.stopped = true
			return res
		}
		v, ok := spiritSee(io, wake)
		if !ok || !v.Grid {
			return res
		}
		res.view = v
		if !v.Cursor {
			return res
		}
		if known && v.DX == px && v.DY == py {
			res.stuck = true
			return res
		}
		px, py, known = v.DX, v.DY, true
	}
	res.reached = known && px == tx && py == ty
	return res
}

// spiritLook 지금 화면의 빙의 상태
func spiritLook(io SpiritIO) (SpiritView, bool) {
	img, err := io.Capture()
	if err != nil {
		return SpiritView{}, false
	}
	return DetectSpiritView(img, io.Region(img)), true
}

// spiritSee 지금 화면 — wake 면 커서가 안 그려져 있을 때 활성 위장(백그라운드) 뒤 한 번 더 본다
func spiritSee(io SpiritIO, wake bool) (SpiritView, bool) {
	v, ok := spiritLook(io)
	if ok && v.Grid && !v.Cursor && wake && io.Wake != nil {
		io.Wake()
		if !io.Sleep(150 * time.Millisecond) {
			return v, ok
		}
		v, ok = spiritLook(io)
	}
	return v, ok
}

// spiritCursorHome 커서를 캐릭터 칸으로 되돌린다(스킬이 커서 자리에 나가므로). Q 로 간 뒤엔 커서가 새 캐릭터 칸에
// 있어 보통 할 일이 없다. 안 보이면 그대로 둔다.
func spiritCursorHome(io SpiritIO, fg bool, who string) {
	tap := io.Tap
	if fg {
		tap = io.TapFG
	}
	v, ok := spiritSee(io, !fg)
	if !ok || !v.Grid || !v.Cursor || (v.DX == 0 && v.DY == 0) {
		return
	}
	io.Log(fmt.Sprintf("[영술사] %s 커서가 캐릭터에서 %+d,%+d 칸 — 되돌림", who, v.DX, v.DY))
	if st := spiritSteer(io, tap, !fg, v, 0, 0); !st.reached && !st.stopped && st.view.Cursor {
		io.Log(fmt.Sprintf("[영술사] %s 커서를 캐릭터 칸으로 못 되돌림 (%s)", who, spiritCursorText(st.view)))
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

// spiritCoordsNow 지금 좌표
func spiritCoordsNow(io SpiritIO) (GameCoords, bool) {
	img, err := io.Capture()
	if err != nil {
		return GameCoords{}, false
	}
	c, err := io.Coords(img)
	if err != nil || (c.X == 0 && c.Y == 0) {
		return GameCoords{}, false
	}
	return c, true
}

// SpiritDiagnose '영술사 테스트' — ① 빙의 칸·커서가 보이는지 ② 백그라운드 방향키가 커서를 한 칸씩 움직이는지
// ③ 사냥 때와 같은 SpiritMoveTo 로 오른쪽 3칸 갔다가 제자리. 잡은 화면은 단계 이름을 붙여 전부 io.Save 로
// 남긴다(사용자 2026-10-07: 봇이 실제로 본 화면이 필요).
func SpiritDiagnose(io SpiritIO) []string {
	var out []string
	add := func(format string, a ...interface{}) { out = append(out, fmt.Sprintf(format, a...)) }
	step := "시작"
	if io.Save != nil {
		capture, save := io.Capture, io.Save
		io.Capture = func() (*image.RGBA, error) {
			img, err := capture()
			if err == nil {
				save(step, img)
			}
			return img, err
		}
	}
	img, err := io.Capture()
	if err != nil {
		add("화면을 못 읽음: %v (창이 최소화돼 있으면 안 됩니다)", err)
		return out
	}
	c, cerr := io.Coords(img)
	if cerr != nil || (c.X == 0 && c.Y == 0) {
		add("좌표를 못 읽음 (오른쪽 아래 X·Y 표시) — 좌표가 보이는 상태에서 다시 눌러 주세요")
		return out
	}
	add("좌표 (%d,%d)", c.X, c.Y)
	v := DetectSpiritView(img, io.Region(img))
	if !v.Grid {
		add("파란 네모가 안 보임 — 빙의 상태가 아니거나 화면에서 못 찾음. 빙의:도깨비불을 쓴 뒤 다시 눌러 주세요")
		return out
	}
	add("파란 네모 찾음 — 커서 %s", spiritCursorText(v))
	if !v.Cursor && io.Wake != nil {
		step = "활성위장"
		io.Wake()
		io.Sleep(300 * time.Millisecond)
		if w, ok := spiritLook(io); ok {
			v = w
			add("백그라운드 활성 위장 → 커서 %s", spiritCursorText(v))
		}
	}

	// ② 백그라운드 방향키 한 칸씩 (→ 한 번, ↓ 한 번) — 커서 자리가 바뀌는지
	moved, stuck := 0, 0
	prev := v
	for _, s := range []struct{ key, arrow, name string }{{"right", "→", "오른쪽"}, {"down", "↓", "아래"}} {
		step = "백그라운드_" + s.name
		io.Tap(s.key)
		io.Sleep(350 * time.Millisecond)
		cur, _ := spiritLook(io)
		add("백그라운드 %s 1칸 → 커서 %s", s.arrow, spiritCursorText(cur))
		if prev.Cursor && cur.Cursor {
			if cur.DX == prev.DX && cur.DY == prev.DY {
				stuck++
			} else {
				moved++
			}
		}
		prev = cur
	}
	bg := "확실하지 않음(커서가 안 보임)"
	switch {
	case moved == 2:
		bg = "커서가 움직임"
		io.Tap("left") // 눌렀던 만큼 되돌림
		io.Sleep(spiritKeyGap)
		io.Tap("up")
		io.Sleep(300 * time.Millisecond)
	case stuck == 2:
		bg = "커서가 그대로 — 게임이 백그라운드 방향키를 안 받음"
	}
	add("→ 백그라운드 방향키: %s", bg)

	// ③ 실제 이동 — 사냥 때와 같은 코드로 오른쪽 3칸 → 제자리. 입력 방식은 ②의 결과로 새로 정한다(창별 기억에도 남음).
	mode := io.Mode
	if mode == nil {
		mode = &SpiritMode{}
		io.Mode = mode
	}
	mode.fg.Store(stuck == 2 && io.Activate != nil && io.TapFG != nil)
	mode.failUntil.Store(0)
	var moveLog []string
	logf := io.Log
	io.Log = func(m string) {
		logf(m)
		moveLog = append(moveLog, strings.TrimPrefix(m, "[영술사] 테스트 "))
	}
	hop := func(from GameCoords, tx, ty int, name string) (GameCoords, bool) {
		step, moveLog = name, nil
		mode.failUntil.Store(0)
		SpiritMoveTo(io, tx, ty, "", "테스트")
		for _, m := range moveLog {
			add("  · %s", m)
		}
		now, ok := spiritCoordsNow(io)
		switch {
		case !ok:
			add("%s: 좌표를 못 읽음", name)
			return from, false
		case now.X == from.X && now.Y == from.Y:
			add("%s: 그대로 (%d,%d)", name, now.X, now.Y)
			return now, false
		}
		add("%s: (%d,%d) → (%d,%d)", name, from.X, from.Y, now.X, now.Y)
		return now, true
	}
	c2, ok := hop(c, c.X+3, c.Y, "오른쪽3칸")
	if !ok {
		add("결론: 이동이 안 됨 — 찍힌 화면 폴더를 올려 주세요")
		return out
	}
	hop(c2, c.X, c.Y, "제자리")
	if mode.ForegroundOnly() {
		add("결론: 백그라운드 키는 빙의 커서가 안 받음 — 영술사 이동 때만 창을 잠깐 앞으로 가져와 누릅니다(이 창은 기억)")
	} else {
		add("결론: 백그라운드로 이동됨 — 창을 띄우지 않습니다")
	}
	return out
}
func spiritCursorText(v SpiritView) string {
	if !v.Grid {
		return "(파란 네모 없음)"
	}
	if !v.Cursor {
		return "안 보임"
	}
	if v.DX == 0 && v.DY == 0 {
		return "캐릭터 칸" + spiritTintSuffix(v.Tint)
	}
	return fmt.Sprintf("캐릭터에서 %+d,%+d 칸", v.DX, v.DY) + spiritTintSuffix(v.Tint)
}

func spiritTintSuffix(t string) string {
	if t == "" {
		return ""
	}
	return " (" + t + ")"
}

func spiritTintText(t string) string {
	if t == "" {
		return "색 모름"
	}
	return t
}

func spiritHowText(fg bool) string {
	if fg {
		return " (창을 앞으로 가져와 누름)"
	}
	return ""
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
