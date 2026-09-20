// glyphgen ─ 게임 폰트 사전을 통째로 생성한다.
//
// 게임은 윈도우 기본 비트맵 폰트 **굴림체 12px 굵게(FW_BOLD)** 를 그대로 쓴다.
// GDI 는 비트맵 폰트를 굵게 만들 때 1px 오른쪽으로 민 사본을 OR 하므로 1px 획이 2px 이 된다.
// 어드밴스도 12→13 이 되지만 사전은 타이트 비트맵이라 상관없다.
//
// 근거: dataset 714장에서 실측한 글리프 59자와 렌더 결과가 픽셀 단위로 전부 일치한다
// (cmd/fontprobe 로 재확인 가능). 보통 굵기(400)로는 한 글자도 안 맞는다.
//
// 이걸로 한글 11,172자를 미리 찍어 넣으므로 '글자 학습'이 필요 없다 —
// 어떤 캐릭터 이름이든 어느 PC 에서든 바로 읽힌다.
//
//	go run ./cmd/glyphgen              # automation/glyph_dict.json 생성
//	go run ./cmd/glyphgen -check       # 생성만 하고 기존 사전과 대조 (파일은 안 건드림)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"os"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"example.com/m/automation"
)

const (
	fontFace   = "굴림체"
	fontHeight = 12
	fontWeight = 700 // FW_BOLD — 이게 핵심이다
	dictPath   = "automation/glyph_dict.json"
	seedPath   = "automation/glyph_dict_seed.json"

	cellPitch = 24 // 렌더할 때 글자 사이 간격 (붙지 않게 넉넉히)
	perRow    = 48
)

var (
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	user32                 = syscall.NewLazyDLL("user32.dll")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procCreateFontW        = gdi32.NewProc("CreateFontW")
	procTextOutW           = gdi32.NewProc("TextOutW")
	procSetBkColor         = gdi32.NewProc("SetBkColor")
	procSetTextColor       = gdi32.NewProc("SetTextColor")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
)

type bmiHeader struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	XPPM, YPPM             int32
	ClrUsed, ClrImportant  uint32
}
type bitmapInfo struct {
	H      bmiHeader
	Colors [3]uint32
}

const (
	nonAntialiased = 3
	hangeulCharset = 129
	opaque         = 2
)

// renderRow 글자들을 cellPitch 간격으로 한 줄에 찍어 이진 격자로 돌려준다.
// 간격을 넉넉히 둬서 굵게 렌더로 넓어진 글리프끼리 맞닿지 않게 한다.
func renderRow(chars []rune) [][]bool {
	W, H := len(chars)*cellPitch+cellPitch, 32
	screenDC, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screenDC)
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	defer procDeleteDC.Call(memDC)

	var bmi bitmapInfo
	bmi.H.Size = uint32(unsafe.Sizeof(bmi.H))
	bmi.H.Width, bmi.H.Height = int32(W), -int32(H)
	bmi.H.Planes, bmi.H.BitCount = 1, 32
	var bits unsafe.Pointer
	hBmp, _, _ := procCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bmi)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hBmp == 0 {
		return nil
	}
	defer procDeleteObject.Call(hBmp)
	procSelectObject.Call(memDC, hBmp)
	data := unsafe.Slice((*byte)(bits), W*H*4)
	for i := range data {
		data[i] = 0xFF
	}

	fn, _ := syscall.UTF16PtrFromString(fontFace)
	hFont, _, _ := procCreateFontW.Call(uintptr(int32(fontHeight)), 0, 0, 0, fontWeight, 0, 0, 0,
		hangeulCharset, 0, 0, nonAntialiased, 0, uintptr(unsafe.Pointer(fn)))
	if hFont == 0 {
		return nil
	}
	defer procDeleteObject.Call(hFont)
	procSelectObject.Call(memDC, hFont)
	procSetBkMode.Call(memDC, opaque)
	procSetBkColor.Call(memDC, 0x00FFFFFF)
	procSetTextColor.Call(memDC, 0)

	for i, r := range chars {
		buf := utf16Of(r)
		procTextOutW.Call(memDC, uintptr(i*cellPitch+2), 0,
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	}

	grid := make([][]bool, H)
	for y := 0; y < H; y++ {
		grid[y] = make([]bool, W)
		for x := 0; x < W; x++ {
			i := (y*W + x) * 4
			grid[y][x] = (int(data[i+2])*299+int(data[i+1])*587+int(data[i])*114)/1000 < 128
		}
	}
	return grid
}

func utf16Of(r rune) []uint16 {
	s, _ := syscall.UTF16FromString(string(r))
	return s[:len(s)-1]
}

// cutCell 격자에서 [x0,x1) 칸의 잉크를 타이트하게 잘라 글리프로 만든다.
// Top 은 폰트의 고정 윗줄(fontTop)을 기준으로 잡는다 — 게임 텍스트 줄의 밴드 상단과 같다.
func cutCell(grid [][]bool, x0, x1, fontTop int) (automation.Glyph, bool) {
	minX, maxX, minY, maxY := 1<<30, -1, 1<<30, -1
	for y := range grid {
		for x := x0; x < x1 && x < len(grid[y]); x++ {
			if grid[y][x] {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < 0 {
		return automation.Glyph{}, false
	}
	g := automation.Glyph{Top: minY - fontTop, W: maxX - minX + 1, H: maxY - minY + 1}
	g.Bits = make([]bool, g.W*g.H)
	i := 0
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			g.Bits[i] = grid[y][x]
			i++
		}
	}
	return g, true
}

// findFontTop 폰트가 그리는 가장 윗줄. 한글 여러 자를 찍어 최소 잉크 행을 본다.
func findFontTop() int {
	var rs []rune
	for r := rune(0xAC00); r < 0xAC00+perRow; r++ {
		rs = append(rs, r)
	}
	grid := renderRow(rs)
	for y := range grid {
		for _, v := range grid[y] {
			if v {
				return y
			}
		}
	}
	return 0
}

func main() {
	check := flag.Bool("check", false, "파일을 쓰지 않고 기존 사전과 대조만 한다")
	flag.Parse()

	fontTop := findFontTop()
	fmt.Printf("폰트: %s %dpx weight=%d, 윗줄 y=%d\n", fontFace, fontHeight, fontWeight, fontTop)

	// 생성 대상: 한글 음절 전체 + 출력 가능한 ASCII
	var targets []rune
	for r := rune(0x20); r <= 0x7E; r++ {
		targets = append(targets, r)
	}
	for r := rune(0xAC00); r <= 0xD7A3; r++ {
		targets = append(targets, r)
	}

	out := map[string]string{}
	collide := 0
	var samples []string
	var blank []string
	for i := 0; i < len(targets); i += perRow {
		end := i + perRow
		if end > len(targets) {
			end = len(targets)
		}
		batch := targets[i:end]
		grid := renderRow(batch)
		if grid == nil {
			fmt.Println("렌더 실패")
			os.Exit(1)
		}
		for j, r := range batch {
			g, ok := cutCell(grid, j*cellPitch, (j+1)*cellPitch, fontTop)
			if !ok {
				blank = append(blank, string(r))
				continue
			}
			k := g.Key()
			if prev, dup := out[k]; dup {
				collide++
				if len(samples) < 12 {
					samples = append(samples, prev+"="+string(r))
				}
				// 같은 비트맵이면 코드포인트가 낮은 쪽을 남긴다.
				// 한글 음절은 (초성×중성×종성) 순이라 앞쪽이 실사용 빈도가 높다(재 > 쟤).
				if []rune(prev)[0] < r {
					continue
				}
			}
			out[k] = string(r)
		}
	}
	fmt.Printf("생성: %d글리프 (대상 %d자, 비트맵 겹침 %d, 빈 글자 %d)\n",
		len(out), len(targets), collide, len(blank))
	fmt.Printf("  겹침 예시: %s\n", strings.Join(samples, " "))
	// 실측 사전(dataset 714장에서 뽑은 것)과 대조 — 어긋나면 그게 버그 신호다
	if raw, err := os.ReadFile(seedPath); err == nil {
		seed := map[string]string{}
		json.Unmarshal(raw, &seed)
		hit, var_miss := 0, []string{}
		for k, v := range seed {
			if out[k] == v {
				hit++
			} else {
				var_miss = append(var_miss, v)
			}
		}
		sort.Strings(var_miss)
		fmt.Printf("실측 사전 대조: %d/%d 일치", hit, len(seed))
		if len(var_miss) > 0 {
			fmt.Printf("  불일치 [%s]", strings.Join(var_miss, ""))
		}
		fmt.Println()
	}

	if *check {
		return
	}
	b, _ := json.Marshal(out)
	if err := os.WriteFile(dictPath, b, 0644); err != nil {
		fmt.Println("저장 실패:", err)
		os.Exit(1)
	}
	fmt.Printf("저장: %s (%.0fKB)\n", dictPath, float64(len(b))/1024)
	_ = image.Rect
}
