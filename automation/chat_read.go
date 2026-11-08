package automation

import (
	_ "embed"
	"encoding/json"
	"image"
	"log"
	"sort"
	"strings"
	"sync"
)

// 좌하단 채팅창 읽기 ─ 채팅은 굴림체 12px **보통 굵기**라 굵게 사전과 따로
// glyph_dict_chat.json(cmd/glyphgen -chat 으로 생성)을 쓴다.
//
// 녹화 PNG 4세션 6773줄 실측: 99% 줄이 통째로 읽힌다. 실패는 한 줄에 색이 섞인
// 시스템 공지(아이템 이름만 다른 색)와 드문 기호 정도라, 모르는 단어만 '?' 로 두고 나머지는 읽는다.
//
// 줄 구조(1600x900 클라이언트): 줄 간격 15px, 글자 높이 11~12px, 한 줄 한 색.
// 테두리 있는 세계후는 테두리 색이 더 많고 글자는 그 안쪽 색이라, 많이 쓰인 색 몇 개를 차례로 넣어 본다.

//go:embed glyph_dict_chat.json
var embeddedChatGlyphDict []byte

var (
	chatDictOnce sync.Once
	chatDict     GlyphDict
)

// LoadChatGlyphDict 채팅용 보통 굵기 사전
func LoadChatGlyphDict() GlyphDict {
	chatDictOnce.Do(func() {
		chatDict = GlyphDict{}
		if err := json.Unmarshal(embeddedChatGlyphDict, &chatDict); err != nil {
			log.Printf("[글리프] 채팅 사전 파싱 실패: %v", err)
		}
	})
	return chatDict
}

// 채팅 영역 — 클라이언트 아래쪽 기준 (1600x900 에서 y 738~846, x 16~600).
// x 12 에 채팅창 검은 세로 테두리가 있어서, 검은 글자 세계후를 읽을 때 같이 잡히지 않게 그 안쪽부터 본다.
const (
	chatX0, chatX1     = 16, 600
	chatTopFromBottom  = 162
	chatBotFromBottom  = 54
	chatBright         = 170 // 채팅 배경은 어둡게 깔려서 글자만 이 밝기를 넘는다
	chatWordGap        = 4   // 이 폭 이상 비면 단어 경계(띄어쓰기 6px)
	chatMinH, chatMaxH = 8, 16
)

type chatRGB [3]uint8

// ReadChatLines 창 이미지에서 채팅 줄들을 위→아래 순서로 읽는다. 띄어쓰기는 단어 경계로 복원한다.
func ReadChatLines(img image.Image, offX, offY, cw, ch int) []string {
	x0, x1 := offX+chatX0, offX+min(chatX1, cw)
	y0, y1 := offY+ch-chatTopFromBottom, offY+ch-chatBotFromBottom
	b := img.Bounds()
	if x0 < b.Min.X || y0 < b.Min.Y || x1 > b.Max.X || y1 > b.Max.Y || x1 <= x0 || y1 <= y0 {
		return nil
	}
	W, H := x1-x0, y1-y0
	px := make([]chatRGB, W*H)
	lit := make([]bool, H) // 밝은 픽셀이 있는 행
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			r, g, bb, _ := img.At(x0+x, y0+y).RGBA()
			c := chatRGB{uint8(r >> 8), uint8(g >> 8), uint8(bb >> 8)}
			px[y*W+x] = c
			if max(c[0], c[1], c[2]) >= chatBright {
				lit[y] = true
			}
		}
	}

	d := LoadChatGlyphDict()
	var out []string
	for y := 0; y < H; {
		if !lit[y] {
			y++
			continue
		}
		s := y
		for y < H && lit[y] {
			y++
		}
		if h := y - s; h < chatMinH || h > chatMaxH {
			continue // 채팅창 테두리선·위로 걸친 그림 등
		}
		if txt := readChatLine(px[s*W:y*W], W, y-s, d); txt != "" {
			out = append(out, txt)
		}
	}
	return out
}

// readChatLine 한 줄. 후보 색마다 읽어 보고 '?' 가 가장 적은 결과를 쓴다.
func readChatLine(px []chatRGB, W, H int, d GlyphDict) string {
	cnt := map[chatRGB]int{}
	for _, c := range px {
		if max(c[0], c[1], c[2]) >= chatBright {
			cnt[c]++
		}
	}
	cands := make([]chatRGB, 0, len(cnt))
	for c := range cnt {
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool { return cnt[cands[i]] > cnt[cands[j]] })
	if len(cands) > 3 {
		cands = cands[:3]
	}
	// 테두리 세계후 중 속색이 어두운 것(갈색 테두리/탁회색 등)은 속색이 밝기 기준에 못 미친다.
	// 가장 많은 밝은 색(=테두리)에 좌우나 위아래로 끼인 픽셀의 색도 후보로 넣는다.
	if len(cands) > 0 {
		edge := cands[0]
		inner := map[chatRGB]int{}
		for i, c := range px {
			x, y := i%W, i/W
			if c == edge || x == 0 || x == W-1 || y == 0 || y == H-1 {
				continue
			}
			if px[i-1] == edge && px[i+1] == edge || px[i-W] == edge && px[i+W] == edge {
				inner[c]++
			}
		}
		var ins []chatRGB
		for c := range inner {
			ins = append(ins, c)
		}
		sort.Slice(ins, func(i, j int) bool { return inner[ins[i]] > inner[ins[j]] })
		for _, c := range ins[:min(2, len(ins))] {
			if cnt[c] == 0 { // 이미 밝은 후보에 있으면 건너뜀
				cands = append(cands, c)
			}
		}
	}
	nBright := min(3, len(cnt))
	var outside []bool // 테두리를 넘지 않고 줄 가장자리에서 닿는 픽셀 = 배경
	best, bestBad := "", 1<<30
	for ci, ink := range cands {
		bin := &GlyphBin{W: W, H: H, Px: make([]bool, W*H)}
		if ci >= nBright {
			// 테두리 안쪽 색: 배경이 같은 색(검은 글자 + 검은 배경)일 수 있어 둘러싸인 픽셀만 글자로 본다
			if outside == nil {
				outside = chatOutside(px, W, H, cands[0])
			}
			for i, c := range px {
				bin.Px[i] = c == ink && !outside[i]
			}
		} else {
			for i, c := range px {
				bin.Px[i] = c == ink
			}
		}
		var alt *GlyphBin
		if ci >= nBright {
			alt = &GlyphBin{W: W, H: H, Px: append([]bool(nil), bin.Px...)}
			dropChatHoles(alt, px, cands[0])
		}
		txt, bad := readChatWords(bin, alt, d)
		if bad < bestBad || bad == bestBad && len(txt) > len(best) {
			best, bestBad = txt, bad
		}
		if bad == 0 {
			break
		}
	}
	return best
}

// chatOutside 줄 가장자리에서 테두리 색(edge)을 건너지 않고 4방향으로 닿는 픽셀.
// 테두리는 글자를 1px 둘러싸므로 글자 속 픽셀은 여기 안 들어간다.
func chatOutside(px []chatRGB, W, H int, edge chatRGB) []bool {
	out := make([]bool, W*H)
	var q []int
	push := func(i int) {
		if !out[i] && px[i] != edge {
			out[i] = true
			q = append(q, i)
		}
	}
	for x := 0; x < W; x++ {
		push(x)
		push((H-1)*W + x)
	}
	for y := 0; y < H; y++ {
		push(y * W)
		push(y*W + W - 1)
	}
	for len(q) > 0 {
		i := q[len(q)-1]
		q = q[:len(q)-1]
		x, y := i%W, i/W
		if x > 0 {
			push(i - 1)
		}
		if x < W-1 {
			push(i + 1)
		}
		if y > 0 {
			push(i - W)
		}
		if y < H-1 {
			push(i + W)
		}
	}
	return out
}

// dropChatHoles 테두리 안쪽 픽셀 중 글자 속·글자 사이 틈(구멍)을 지운다.
// 배경이 글자와 같은 색이면 구멍도 "둘러싸인 픽셀"로 잡힌다. 테두리는 글자를 4방향으로 1px 둘러싸므로
// 진짜 글자 덩어리는 끝 픽셀 바깥에 "그 픽셀 하나에만 붙은 테두리 픽셀"이 꼭 있고,
// 구멍 쪽 테두리 픽셀은 언제나 글자에도 붙어 있어 이웃이 2개 이상이다. 그런 테두리가 하나도 없는 덩어리를 지운다.
func dropChatHoles(b *GlyphBin, px []chatRGB, edge chatRGB) {
	W, H := b.W, b.H
	nb := func(i int) []int { // 4방향 이웃
		x, y := i%W, i/W
		var out []int
		if x > 0 {
			out = append(out, i-1)
		}
		if x < W-1 {
			out = append(out, i+1)
		}
		if y > 0 {
			out = append(out, i-W)
		}
		if y < H-1 {
			out = append(out, i+W)
		}
		return out
	}
	need := make([]bool, W*H)
	for i, c := range px {
		if c != edge {
			continue
		}
		only, n := -1, 0
		for _, j := range nb(i) {
			if b.Px[j] {
				only, n = j, n+1
			}
		}
		if n == 1 {
			need[only] = true
		}
	}
	// 확실한 글자 픽셀(need)에서 4방향으로 이어진 것만 남긴다. 구멍은 테두리 정의상 글자와 4방향으로 닿지 않는다.
	// 대각선으로만 이어진 획 가운데 픽셀도 빠지는데, 이건 구멍과 모양상 구별이 안 돼서 원래 판과 글자마다 골라 쓴다.
	keep := make([]bool, W*H)
	var q []int
	for i, v := range need {
		if v {
			keep[i] = true
			q = append(q, i)
		}
	}
	for len(q) > 0 {
		i := q[len(q)-1]
		q = q[:len(q)-1]
		for _, j := range nb(i) {
			if b.Px[j] && !keep[j] {
				keep[j] = true
				q = append(q, j)
			}
		}
	}
	copy(b.Px, keep)
}
// readChatWords 빈 열로 단어를 나눠 단어마다 읽는다. 모르는 단어는 "?" 로 두고 개수를 센다.
// alt(구멍 지운 판)가 있으면 글자마다 b 와 alt 중 사전에 있는 쪽을 쓴다.
func readChatWords(b, alt *GlyphBin, d GlyphDict) (string, int) {
	top, bot, ok := glyphBand(b)
	if !ok {
		return "", 1 << 20
	}
	cols := glyphInkCols(b, top, bot)
	var words []string
	bad := 0
	for x := 0; x < b.W; {
		if !cols[x] {
			x++
			continue
		}
		first, last := x, x
		for gap := 0; x < b.W && gap < chatWordGap; x++ {
			if cols[x] {
				last, gap = x, 0
			} else {
				gap++
			}
		}
		var w string
		if alt != nil {
			w, ok = recognizeGlyphSpan(b, d, top, bot, cols, first, last, alt)
		} else {
			w, ok = recognizeGlyphSpan(b, d, top, bot, cols, first, last)
		}
		if !ok {
			w = "?"
			bad++
		}
		words = append(words, w)
	}
	return strings.Join(words, " "), bad
}
