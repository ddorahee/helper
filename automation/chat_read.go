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

// 채팅 영역 — 클라이언트 아래쪽 기준 (1600x900 에서 y 738~846, x 10~600)
const (
	chatX0, chatX1     = 10, 600
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
	best, bestBad := "", 1<<30
	for _, ink := range cands {
		bin := &GlyphBin{W: W, H: H, Px: make([]bool, W*H)}
		for i, c := range px {
			bin.Px[i] = c == ink
		}
		txt, bad := readChatWords(bin, d)
		if bad < bestBad || bad == bestBad && len(txt) > len(best) {
			best, bestBad = txt, bad
		}
		if bad == 0 {
			break
		}
	}
	return best
}

// readChatWords 빈 열로 단어를 나눠 단어마다 읽는다. 모르는 단어는 "?" 로 두고 개수를 센다.
func readChatWords(b *GlyphBin, d GlyphDict) (string, int) {
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
		w, ok := recognizeGlyphSpan(b, d, top, bot, cols, first, last)
		if !ok {
			w = "?"
			bad++
		}
		words = append(words, w)
	}
	return strings.Join(words, " "), bad
}
