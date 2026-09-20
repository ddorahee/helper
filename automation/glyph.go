package automation

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// 글리프 템플릿 매칭 ─ 게임 폰트가 안티에일리어싱 없는 고정 비트맵이라는 점을 이용해
// 맵 이름 / 닉네임을 픽셀 단위로 읽는다.
//
// dataset 714장(창 크기 3종) 검증 결과:
//   닉네임 714/714 = 100%,  맵 707/707 = 100%
//   (나머지 7장은 맵 전환 중 암전이거나 마우스 커서가 글자를 가려 글자 자체가 없다)
//   속도 0.44ms/회 — 기존 PowerShell WinRT OCR 은 1410ms, 맵 정확도 1.7% / 닉 71.2%
//
// 폰트 구조:
//   - 전각 12px / 반각 6px 고정 어드밴스
//   - 다만 잉크가 자기 칸을 1px 넘칠 수 있다 ('햐'의 ㅑ 가지는 오른쪽, '험'의 ㅓ 가지는 왼쪽)
//     → 사전 구축(glyph_fit.go)에서 글자별 넘침량을 맞춰 자른다
//   - 인식할 땐 칸 경계를 모르므로 DP 로 "잉크를 전부 소비하는 최소 글자수 분할"을 찾는다

//go:embed glyph_dict.json
var embeddedGlyphDict []byte

// glyphMaxW 한 글리프의 최대 폭 (전각 12 + 넘침 2)
const glyphMaxW = 14

// glyphLumDark 맵 이름 바 안쪽은 lum 0~9(글자) / 110~220(패널)로 완전히 갈린다.
const glyphLumDark = 70

// GlyphBin 이진화된 영역
type GlyphBin struct {
	W, H int
	Px   []bool
}

func (b *GlyphBin) At(x, y int) bool { return b.Px[y*b.W+x] }

// GlyphRegion 잘라낼 영역과 글자 극성
type GlyphRegion struct {
	X0, Y0, X1, Y1 int
	White          bool // true=흰 글자(닉네임), false=어두운 글자(맵 이름)
}

// BinarizeGlyph 영역을 1비트로 만든다
func BinarizeGlyph(img image.Image, r GlyphRegion) *GlyphBin {
	w, h := r.X1-r.X0, r.Y1-r.Y0
	if w <= 0 || h <= 0 {
		return &GlyphBin{}
	}
	b := &GlyphBin{W: w, H: h, Px: make([]bool, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cr, cg, cb, _ := img.At(r.X0+x, r.Y0+y).RGBA()
			r8, g8, b8 := int(cr>>8), int(cg>>8), int(cb>>8)
			var ink bool
			if r.White {
				ink = r8 > 200 && g8 > 200 && b8 > 200
			} else {
				ink = (r8*299+g8*587+b8*114)/1000 < glyphLumDark
			}
			b.Px[y*w+x] = ink
		}
	}
	return b
}

// Glyph 타이트하게 잘린 글리프 비트맵. Top 은 텍스트 밴드 상단 기준 세로 오프셋으로,
// 모양이 같고 위치만 다른 글자를 구분한다.
type Glyph struct {
	Top  int
	W, H int
	Bits []bool
}

// Key 사전 조회용 문자열. 비트를 6개씩 묶어 ASCII 한 글자로 인코딩한다.
func (g Glyph) Key() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d:%dx%d:", g.Top, g.W, g.H)
	acc, n := 0, 0
	for _, v := range g.Bits {
		acc <<= 1
		if v {
			acc |= 1
		}
		n++
		if n == 6 {
			sb.WriteByte(byte(48 + acc))
			acc, n = 0, 0
		}
	}
	if n > 0 {
		acc <<= uint(6 - n)
		sb.WriteByte(byte(48 + acc))
	}
	return sb.String()
}

// glyphBand 잉크가 있는 행 범위 (텍스트 줄)
func glyphBand(b *GlyphBin) (top, bot int, ok bool) {
	top, bot = -1, -1
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.At(x, y) {
				if top < 0 {
					top = y
				}
				bot = y
				break
			}
		}
	}
	return top, bot, top >= 0
}

// glyphInkCols 밴드 안에서 잉크가 있는 열
func glyphInkCols(b *GlyphBin, top, bot int) []bool {
	out := make([]bool, b.W)
	for x := 0; x < b.W; x++ {
		for y := top; y <= bot; y++ {
			if b.At(x, y) {
				out[x] = true
				break
			}
		}
	}
	return out
}

// glyphCut 열 [x0,x1) 을 잘라 상하좌우 타이트한 글리프를 만든다.
// 열까지 타이트하게 자르므로 칸 경계가 1px 어긋나도 같은 비트맵이 나온다.
func glyphCut(b *GlyphBin, bandTop, bandBot, x0, x1 int) (Glyph, bool) {
	if x0 < 0 {
		x0 = 0
	}
	if x1 > b.W {
		x1 = b.W
	}
	colInk := func(x int) bool {
		for y := bandTop; y <= bandBot; y++ {
			if b.At(x, y) {
				return true
			}
		}
		return false
	}
	for x0 < x1 && !colInk(x0) {
		x0++
	}
	for x1 > x0 && !colInk(x1-1) {
		x1--
	}
	if x1 <= x0 {
		return Glyph{}, false
	}
	minY, maxY := 1<<30, -1
	for y := bandTop; y <= bandBot; y++ {
		for x := x0; x < x1; x++ {
			if b.At(x, y) {
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
				break
			}
		}
	}
	if maxY < 0 {
		return Glyph{}, false
	}
	g := Glyph{Top: minY - bandTop, W: x1 - x0, H: maxY - minY + 1}
	g.Bits = make([]bool, g.W*g.H)
	i := 0
	for y := minY; y <= maxY; y++ {
		for x := x0; x < x1; x++ {
			g.Bits[i] = b.At(x, y)
			i++
		}
	}
	return g, true
}

// GlyphDict 비트맵키 → 글자
type GlyphDict map[string]string

// RecognizeGlyphs 잉크를 전부 소비하는 분할 중 글자수가 가장 적은 것을 고른다.
// 글리프 하나라도 사전에 없으면 실패(ok=false)로, 틀린 글자를 지어내지 않는다.
func RecognizeGlyphs(b *GlyphBin, d GlyphDict) (string, bool) {
	if b.W == 0 || len(d) == 0 {
		return "", false
	}
	top, bot, ok := glyphBand(b)
	if !ok {
		return "", false
	}
	cols := glyphInkCols(b, top, bot)
	first, last := -1, -1
	for x := 0; x < b.W; x++ {
		if cols[x] {
			if first < 0 {
				first = x
			}
			last = x
		}
	}

	type memo struct {
		n   int
		str string
		ok  bool
	}
	cache := make(map[int]memo, b.W)
	var solve func(i int) memo
	solve = func(i int) memo {
		for i < b.W && !cols[i] {
			i++
		}
		if i > last {
			return memo{ok: true}
		}
		if m, hit := cache[i]; hit {
			return m
		}
		cache[i] = memo{} // 재귀 가드
		best := memo{}
		for w := 1; w <= glyphMaxW && i+w <= b.W; w++ {
			if !cols[i+w-1] {
				continue // 글리프 오른쪽 끝은 잉크여야 한다
			}
			g, has := glyphCut(b, top, bot, i, i+w)
			if !has {
				continue
			}
			ch, found := d[g.Key()]
			if !found {
				continue
			}
			rest := solve(i + w)
			if !rest.ok {
				continue
			}
			if !best.ok || rest.n+1 < best.n {
				best = memo{rest.n + 1, ch + rest.str, true}
			}
		}
		cache[i] = best
		return best
	}
	res := solve(first)
	return res.str, res.ok
}

// ===== 영역 계산 =====

// GlyphNickRegion 우상단 닉네임. 패널 양옆 장식 못(2px)을 피하려고 기존
// NicknameCrop(0.875~0.945)보다 좁게 잡는다. 6글자(72px)까지 들어간다.
func GlyphNickRegion(offX, offY, cw, ch int) GlyphRegion {
	return GlyphRegion{
		X0:    offX + int(0.8875*float64(cw)),
		Y0:    offY + int(0.005*float64(ch)),
		X1:    offX + int(0.94*float64(cw)),
		Y1:    offY + int(0.06*float64(ch)),
		White: true,
	}
}

// glyphMapFallback 바 검출이 실패했을 때 쓰는 고정 위치 (클라이언트 1600x900 기준)
func glyphMapFallback(offX, offY int) GlyphRegion {
	return GlyphRegion{X0: offX + 622, Y0: offY + 4, X1: offX + 781, Y1: offY + 20}
}

// DetectMapBar 상단 중앙에서 황갈색 맵 이름 패널을 찾아, 테두리를 물린 텍스트 영역을 돌려준다.
// 창 크기가 달라도 따라가도록 고정 좌표 대신 검출을 쓴다.
func DetectMapBar(img image.Image, offX, offY, cw, ch int) (GlyphRegion, bool) {
	bounds := img.Bounds()
	sx0, sx1 := offX+cw*25/100, offX+cw*65/100
	sy0, sy1 := offY, offY+ch*7/100
	if sx1 > bounds.Max.X {
		sx1 = bounds.Max.X
	}
	if sy1 > bounds.Max.Y {
		sy1 = bounds.Max.Y
	}
	if sx1-sx0 < 120 || sy1-sy0 < 10 {
		return GlyphRegion{}, false
	}
	tan := func(x, y int) bool {
		cr, cg, cb, _ := img.At(x, y).RGBA()
		r8, g8, b8 := int(cr>>8), int(cg>>8), int(cb>>8)
		l := (r8*299 + g8*587 + b8*114) / 1000
		return l > 95 && l < 235 && r8 > b8+25 && r8 >= g8
	}
	ry0, ry1 := -1, -1
	for y := sy0; y < sy1; y++ {
		run, best := 0, 0
		for x := sx0; x < sx1; x++ {
			if tan(x, y) {
				run++
				if run > best {
					best = run
				}
			} else {
				run = 0
			}
		}
		if best >= 100 {
			if ry0 < 0 {
				ry0 = y
			}
			ry1 = y
		} else if ry0 >= 0 {
			break
		}
	}
	if ry0 < 0 || ry1-ry0 < 8 {
		return GlyphRegion{}, false
	}
	h := ry1 - ry0 + 1
	bestLen, bestStart, curStart := 0, 0, -1
	for x := sx0; x <= sx1; x++ {
		on := false
		if x < sx1 {
			n := 0
			for y := ry0; y <= ry1; y++ {
				if tan(x, y) {
					n++
				}
			}
			on = n*100 >= h*35
		}
		if on && curStart < 0 {
			curStart = x
		} else if !on && curStart >= 0 {
			if x-curStart > bestLen {
				bestLen, bestStart = x-curStart, curStart
			}
			curStart = -1
		}
	}
	if bestLen < 100 {
		return GlyphRegion{}, false
	}
	return GlyphRegion{X0: bestStart + 2, Y0: ry0 + 1, X1: bestStart + bestLen - 2, Y1: ry1}, true
}

// GlyphMapRegion 맵 이름 영역 (검출 실패 시 고정 좌표)
func GlyphMapRegion(img image.Image, offX, offY, cw, ch int) GlyphRegion {
	if r, ok := DetectMapBar(img, offX, offY, cw, ch); ok {
		return r
	}
	return glyphMapFallback(offX, offY)
}

// ===== 사전 보관 =====

var (
	glyphDictMu   sync.RWMutex
	glyphDict     GlyphDict
	glyphDictPath = filepath.Join("config", "glyphs.json")
)

// LoadGlyphDict 내장 사전 + 사용자가 학습시킨 사전(config/glyphs.json)을 합쳐 올린다.
func LoadGlyphDict() GlyphDict {
	glyphDictMu.RLock()
	d := glyphDict
	glyphDictMu.RUnlock()
	if d != nil {
		return d
	}

	glyphDictMu.Lock()
	defer glyphDictMu.Unlock()
	if glyphDict != nil {
		return glyphDict
	}
	d = GlyphDict{}
	if err := json.Unmarshal(embeddedGlyphDict, &d); err != nil {
		log.Printf("[글리프] 내장 사전 파싱 실패: %v", err)
		d = GlyphDict{}
	}
	base := len(d)
	if raw, err := os.ReadFile(glyphDictPath); err == nil {
		user := GlyphDict{}
		if err := json.Unmarshal(raw, &user); err == nil {
			for k, v := range user {
				d[k] = v
			}
		} else {
			log.Printf("[글리프] 학습 사전 파싱 실패: %v", err)
		}
	}
	log.Printf("[글리프] 사전 %d개 (내장 %d, 학습 %d)", len(d), base, len(d)-base)
	glyphDict = d
	return d
}

// MergeGlyphDict 새로 학습한 글리프를 사전에 합치고 config/glyphs.json 에 저장한다.
func MergeGlyphDict(add GlyphDict) (int, error) {
	cur := LoadGlyphDict()
	glyphDictMu.Lock()
	added := 0
	for k, v := range add {
		if old, ok := cur[k]; ok {
			if old == v {
				continue
			}
			log.Printf("[글리프] 덮어씀: '%s' → '%s'", old, v)
		}
		cur[k] = v
		added++
	}
	glyphDictMu.Unlock()

	user := GlyphDict{}
	if raw, err := os.ReadFile(glyphDictPath); err == nil {
		json.Unmarshal(raw, &user)
	}
	for k, v := range add {
		user[k] = v
	}
	if err := os.MkdirAll(filepath.Dir(glyphDictPath), 0755); err != nil {
		return added, err
	}
	raw, err := json.MarshalIndent(user, "", " ")
	if err != nil {
		return added, err
	}
	return added, os.WriteFile(glyphDictPath, raw, 0644)
}

// GlyphDictChars 사전이 아는 글자들 (UI 표시용)
func GlyphDictChars() string {
	d := LoadGlyphDict()
	glyphDictMu.RLock()
	defer glyphDictMu.RUnlock()
	seen := map[string]bool{}
	var out []string
	for _, v := range d {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return strings.Join(out, "")
}

// GlyphDictSize 현재 사전 글리프 수
func GlyphDictSize() int {
	d := LoadGlyphDict()
	glyphDictMu.RLock()
	defer glyphDictMu.RUnlock()
	return len(d)
}
