package automation

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"
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
	spiritKeyGap     = 60 * time.Millisecond // 커서 방향키 사이 (화면 커서라 빨라도 됨)
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
	if bestN >= 250 {
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

// SpiritIO 영술사 이동에 쓰는 창 입출력 — 창마다 백그라운드/포그라운드가 달라 부르는 쪽이 채운다.
type SpiritIO struct {
	Capture func() (*image.RGBA, error)                // 창을 띄우지 않는 캡처
	Region  func(img *image.RGBA) image.Rectangle      // 클라이언트(게임 화면) 영역
	Coords  func(img *image.RGBA) (GameCoords, error)  // 우하단 HUD 좌표
	Tap     func(key string)                           // 키 1회
	Sleep   func(d time.Duration) bool                 // 중지되면 false
	Log     func(msg string)
}

// SpiritMoveTo 영술사를 (tx,ty) 로 옮긴다(±1 이면 그대로). 빙의 중이 아니면 possessKey(빙의:도깨비불)부터 누른다.
// 커서를 목표 칸까지 옮기고 Q → 커서가 캐릭터 칸으로 돌아왔는지 보고 아니면 되돌린다. 키를 눌렀으면 true.
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
		io.Log(fmt.Sprintf("[영술사] %s 빙의 안 됨 — %s(빙의:도깨비불) 누르고 이동", who, key))
		io.Tap(key)
		if !io.Sleep(1500 * time.Millisecond) {
			return true
		}
		if img, err = io.Capture(); err != nil {
			return true
		}
		if v = DetectSpiritView(img, io.Region(img)); !v.Grid {
			io.Log(fmt.Sprintf("[영술사] %s 빙의 칸(파란 네모)이 안 보여 이동 생략", who))
			return true
		}
	}
	// 한 번에 갈 수 있는 건 네모 안(좌우 8칸, 위아래 7칸) — 더 멀면 다음 점검 때 이어서 간다
	mx, my := spiritClamp(dx, spiritHalfW), spiritClamp(dy, spiritHalfH)
	io.Log(fmt.Sprintf("[영술사] %s (%d,%d) → (%d,%d): 커서 %+d,%+d 칸 → Q", who, c.X, c.Y, tx, ty, mx, my))
	if !spiritArrows(io, mx-v.DX, my-v.DY) {
		return true
	}
	io.Tap("q")
	if !io.Sleep(800 * time.Millisecond) {
		return true
	}
	// 옮긴 뒤 커서를 캐릭터 칸으로 — 스킬이 커서 자리에 나가므로 (두 번까지 확인)
	for try := 0; try < 2; try++ {
		img, err := io.Capture()
		if err != nil {
			break
		}
		v := DetectSpiritView(img, io.Region(img))
		if !v.Grid || !v.Cursor || (v.DX == 0 && v.DY == 0) {
			break
		}
		io.Log(fmt.Sprintf("[영술사] %s 커서가 캐릭터에서 %+d,%+d 칸 — 되돌림", who, v.DX, v.DY))
		if !spiritArrows(io, -v.DX, -v.DY) || !io.Sleep(300*time.Millisecond) {
			break
		}
	}
	return true
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
func spiritArrows(io SpiritIO, ax, ay int) bool {
	press := func(n int, pos, neg string) bool {
		key := pos
		if n < 0 {
			key, n = neg, -n
		}
		for i := 0; i < n; i++ {
			io.Tap(key)
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
