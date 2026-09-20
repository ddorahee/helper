// fontprobe ─ 게임 폰트가 윈도우 기본 비트맵 폰트와 같은지 확인한다.
//
// 결론: 다르다. 윈도우 폰트로 한글 2350자를 찍어내는 길은 막혔다.
//
//	게임:   #######.##...#####..##.....##...##..#######   (대야산기슭)
//	굴림체: .####.#..#....####...#......#....#...######
//
// 어드밴스는 둘 다 12px 로 같지만, 게임 폰트는 세로획이 2px 이고 굴림체·돋움체·
// 바탕체·궁서체는 1px 이다. 사전의 한글 68자 중 한 글자도 일치하지 않았다.
// 바람의나라가 자체 비트맵 폰트를 쓴다는 뜻이다.
//
// 이 도구는 그 판단 근거를 다시 확인할 수 있게 남겨둔다.
//
//	go run ./cmd/fontprobe                    # 후보 폰트들을 현재 사전과 대조
//	go run ./cmd/fontprobe 굴림체 12           # 사전 글자 전체를 그 폰트로 렌더
//	go run ./cmd/fontprobe text 굴림체 12 대야산기슭  # 특정 문자열만 렌더
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"example.com/m/automation"
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
	procGetTextExtent      = gdi32.NewProc("GetTextExtentPoint32W")
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

var fontWeight = 700

const (
	nonAntialiased = 3
	hangeulCharset = 129
	opaque         = 2
)

// renderText 흰 배경에 검은 글자로 렌더해 이미지와 실제 글자 영역 크기를 돌려준다
func renderText(face string, height int, text string) (*image.RGBA, int, int) {
	const W, H = 1400, 48
	screenDC, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screenDC)
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	defer procDeleteDC.Call(memDC)

	var bmi bitmapInfo
	bmi.H.Size = uint32(unsafe.Sizeof(bmi.H))
	bmi.H.Width, bmi.H.Height = W, -H
	bmi.H.Planes, bmi.H.BitCount = 1, 32
	var bits unsafe.Pointer
	hBmp, _, _ := procCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bmi)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hBmp == 0 {
		return nil, 0, 0
	}
	defer procDeleteObject.Call(hBmp)
	procSelectObject.Call(memDC, hBmp)

	// DIB 는 검정으로 시작하므로 먼저 전체를 흰색으로 채운다
	data := unsafe.Slice((*byte)(bits), W*H*4)
	for i := range data {
		data[i] = 0xFF
	}

	fn, _ := syscall.UTF16PtrFromString(face)
	hFont, _, _ := procCreateFontW.Call(uintptr(int32(height)), 0, 0, 0, uintptr(fontWeight), 0, 0, 0,
		hangeulCharset, 0, 0, nonAntialiased, 0, uintptr(unsafe.Pointer(fn)))
	if hFont == 0 {
		return nil, 0, 0
	}
	defer procDeleteObject.Call(hFont)
	procSelectObject.Call(memDC, hFont)
	procSetBkMode.Call(memDC, opaque)
	procSetBkColor.Call(memDC, 0x00FFFFFF)
	procSetTextColor.Call(memDC, 0)

	tp, _ := syscall.UTF16FromString(text)
	tp = tp[:len(tp)-1]
	if len(tp) == 0 {
		return nil, 0, 0
	}
	procTextOutW.Call(memDC, 0, 0, uintptr(unsafe.Pointer(&tp[0])), uintptr(len(tp)))
	var sz struct{ X, Y int32 }
	procGetTextExtent.Call(memDC, uintptr(unsafe.Pointer(&tp[0])), uintptr(len(tp)),
		uintptr(unsafe.Pointer(&sz)))

	img := image.NewRGBA(image.Rect(0, 0, W, H))
	for i := 0; i < W*H; i++ {
		img.Pix[i*4+0] = data[i*4+2]
		img.Pix[i*4+1] = data[i*4+1]
		img.Pix[i*4+2] = data[i*4+0]
		img.Pix[i*4+3] = 255
	}
	return img, int(sz.X), int(sz.Y)
}

// dictChars 현재 사전이 아는 글자들 (한글만)
func dictChars() []string {
	raw, err := os.ReadFile("automation/glyph_dict.json")
	if err != nil {
		return nil
	}
	m := map[string]string{}
	json.Unmarshal(raw, &m)
	seen := map[string]bool{}
	var out []string
	for _, v := range m {
		r := []rune(v)
		if len(r) == 1 && r[0] >= 0xAC00 && r[0] <= 0xD7A3 && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func main() {
	chars := dictChars()
	if len(os.Args) > 1 && os.Args[1] == "text" {
		img, w, hh := renderText(os.Args[2], atoi(os.Args[3]), os.Args[4])
		if img != nil {
			showAscii(img, w, hh)
		}
		return
	}
	if len(chars) == 0 {
		fmt.Println("사전을 읽지 못했다 (helper 루트에서 실행해야 한다)")
		return
	}
	text := strings.Join(chars, "")
	dict := automation.LoadGlyphDict()

	if len(os.Args) > 2 {
		face := os.Args[1]
		h, _ := strconv.Atoi(os.Args[2])
		img, w, hh := renderText(face, h, text)
		if img == nil {
			fmt.Println("렌더 실패")
			return
		}
		showAscii(img, w, hh)
		return
	}

	fmt.Printf("대조 대상: 사전의 한글 %d자\n\n", len(chars))
	faces := []string{"굴림체", "굴림", "돋움체", "돋움", "바탕체", "궁서체", "GulimChe", "Gulim", "DotumChe", "Dotum"}
	type result struct {
		name       string
		hit, total int
		topOff     map[int]int
		miss       []string
	}
	var best result
	for _, face := range faces {
		for _, h := range []int{11, 12, 13, 14, -11, -12} {
			for _, wgt := range []int{400, 700} {
				fontWeight = wgt
				r := compareLine(face, h, wgt, chars, dict)
				if r.total == 0 {
					continue
				}
				mark := " "
				if r.hit == r.total {
					mark = "★"
				}
				fmt.Printf("%s %-9s h=%-4d w=%d  hit %d/%d  topdiff %v  miss[%s]\n",
					mark, face, h, wgt, r.hit, r.total, r.topOff, strings.Join(r.miss, ""))
				if r.hit > best.hit {
					best = r
				}
			}
		}
	}
	fmt.Printf("\n가장 근접: %s (%d/%d자)\n", best.name, best.hit, best.total)
	if best.total > 0 && best.hit == best.total {
		fmt.Println("→ 게임 폰트와 동일하다. 한글 전 음절을 렌더해 사전을 통째로 만들 수 있다.")
	}
}

// compareLine 글자들을 한 줄로 렌더한 뒤, 격자로 잘라 사전의 비트맵과 하나씩 대조한다.
// 굵게 렌더하면 GDI 가 1px 민 사본을 OR 하므로 어드밴스가 12→13 이 된다. 모양은 그대로다.
// 사전은 타이트 비트맵이라 어드밴스 차이는 상관없다.
func compareLine(face string, h, wgt int, chars []string, dict automation.GlyphDict) (res struct {
	name       string
	hit, total int
	topOff     map[int]int
	miss       []string
}) {
	res.name = fmt.Sprintf("%s h=%d w=%d", face, h, wgt)
	res.topOff = map[int]int{}
	text := strings.Join(chars, "")
	img, w, hh := renderText(face, h, text)
	if img == nil || w == 0 {
		return res
	}
	adv := w / len(chars)
	if adv < 8 {
		return res
	}
	reg := automation.GlyphRegion{X0: 0, Y0: 0, X1: w, Y1: hh, White: false}
	bin := automation.BinarizeGlyph(img, reg)

	// 글자 → 사전 비트맵 (Top 제외한 모양만 비교하기 위해 키를 쪼갠다)
	want := map[string]string{}
	for k, v := range dict {
		if i := strings.Index(k, ":"); i >= 0 {
			want[v] = k[i+1:] // "WxH:비트"
		}
	}
	for i, c := range chars {
		g, ok := automation.CutGlyph(bin, i*adv, (i+1)*adv)
		if !ok {
			continue
		}
		res.total++
		full := g.Key()
		shape := full
		if j := strings.Index(full, ":"); j >= 0 {
			shape = full[j+1:]
		}
		if want[c] == shape {
			res.hit++
			res.topOff[g.Top-dictTop(dict, c)]++
		} else {
			res.miss = append(res.miss, c)
		}
	}
	return res
}

func dictTop(dict automation.GlyphDict, ch string) int {
	for k, v := range dict {
		if v != ch {
			continue
		}
		if i := strings.Index(k, ":"); i > 0 {
			t, _ := strconv.Atoi(k[:i])
			return t
		}
	}
	return 0
}

func commonPrefix(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	n := 0
	for n < len(ra) && n < len(rb) && ra[n] == rb[n] {
		n++
	}
	return n
}

func showAscii(img *image.RGBA, w, h int) {
	if w > 150 {
		w = 150
	}
	for y := 0; y < h; y++ {
		line := ""
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if (int(r>>8)*299+int(g>>8)*587+int(b>>8)*114)/1000 < 128 {
				line += "#"
			} else {
				line += "."
			}
		}
		fmt.Println(line)
	}
}

func atoi(s string) int { v, _ := strconv.Atoi(s); return v }
