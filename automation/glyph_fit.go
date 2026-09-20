package automation

import (
	"fmt"
	"strings"
)

// 글리프 학습 ─ "이 화면의 맵 이름은 OO 이다" 를 알려주면 새 글자의 비트맵을 뽑아 사전에 넣는다.
//
// 어려운 점은 글자 경계다. 폰트는 전각 12px / 반각 6px 고정 어드밴스지만 잉크가 자기 칸을
// 1px 넘칠 수 있고(예: '햐'의 ㅑ 가지, '험'의 ㅓ 가지), 이웃 글자와 맞닿으면 빈 열이 없어
// 단순 분할로는 어디서 잘라야 할지 알 수 없다.
//
// 그래서 이미 아는 글자를 닻으로 쓴다: 잘라낸 비트맵이 사전의 그 글자와 정확히 일치하는
// 경로만 인정하고, 모르는 글자에는 어드밴스 ±2 폭만 허용하는 DP 로 남은 자리를 채운다.
// 학습 후에는 같은 화면을 다시 읽어 입력한 문자열이 그대로 나오는지 검증한다.

// glyphAdvance 전각 12 / 반각 6
func glyphAdvance(r rune) int {
	if r < 0x80 {
		return 6
	}
	return 12
}

// glyphGridSlack 격자에서 허용하는 최대 어긋남 (px)
const glyphGridSlack = 3

// FitGlyphs 이진화된 영역과 정답 문자열로부터 새 글리프를 뽑는다.
// 반환은 "새로 배운 글리프"만 담긴 사전. 이미 아는 글자는 포함하지 않는다.
func FitGlyphs(b *GlyphBin, text string, known GlyphDict) (GlyphDict, error) {
	rs := []rune(strings.TrimSpace(text))
	if len(rs) == 0 {
		return nil, fmt.Errorf("문자열이 비었습니다")
	}
	if b.W == 0 {
		return nil, fmt.Errorf("영역이 비었습니다")
	}
	top, bot, ok := glyphBand(b)
	if !ok {
		return nil, fmt.Errorf("글자가 보이지 않습니다 (창이 가려졌거나 맵 전환 중일 수 있음)")
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

	// 글자 → 이미 아는 비트맵키 (역방향 조회)
	keyOf := map[string]string{}
	for k, v := range known {
		keyOf[v] = k
	}

	// 격자 기준 위치
	cum := make([]int, len(rs)+1)
	for i, r := range rs {
		cum[i+1] = cum[i] + glyphAdvance(r)
	}
	if d := cum[len(rs)] - (last - first + 1); d < -4 || d > 5 {
		return nil, fmt.Errorf("글자 수가 맞지 않습니다 (예상 폭 %dpx, 실제 %dpx)",
			cum[len(rs)], last-first+1)
	}

	type state struct{ i, x int }
	type res struct {
		score int
		cuts  []int // 각 글자의 [시작,끝) 을 순서대로
		ok    bool
	}
	memo := map[state]res{}

	var solve func(i, x int) res
	solve = func(i, x int) res {
		for x < b.W && !cols[x] {
			x++
		}
		if i == len(rs) {
			return res{ok: x > last}
		}
		if x > last {
			return res{}
		}
		st := state{i, x}
		if r, hit := memo[st]; hit {
			return r
		}
		memo[st] = res{} // 재귀 가드
		adv := glyphAdvance(rs[i])
		if d := x - (first + cum[i]); d < -glyphGridSlack || d > glyphGridSlack {
			memo[st] = res{}
			return res{}
		}
		best := res{}
		for w := 1; w <= glyphMaxW && x+w <= b.W; w++ {
			if !cols[x+w-1] {
				continue
			}
			g, has := glyphCut(b, top, bot, x, x+w)
			if !has {
				continue
			}
			key := g.Key()
			var gain int
			if want, isKnown := keyOf[string(rs[i])]; isKnown {
				if key != want {
					continue // 아는 글자인데 모양이 다르다 → 잘못 자른 것
				}
				gain = 100
			} else if other, taken := known[key]; taken && other != string(rs[i]) {
				continue // 다른 글자의 비트맵과 겹친다
			} else {
				// 모르는 글자: 어드밴스에 가까운 폭만 인정
				if w < adv-2 || w > adv+2 {
					continue
				}
				gain = 10 - abs(w-adv)
			}
			rest := solve(i+1, x+w)
			if !rest.ok {
				continue
			}
			if !best.ok || rest.score+gain > best.score {
				cuts := append([]int{x, x + w}, rest.cuts...)
				best = res{rest.score + gain, cuts, true}
			}
		}
		memo[st] = best
		return best
	}

	r := solve(0, first)
	if !r.ok {
		return nil, fmt.Errorf("글자 경계를 찾지 못했습니다 — 입력한 이름이 화면과 다르거나 글자가 가려졌을 수 있습니다")
	}

	out := GlyphDict{}
	for i, ch := range rs {
		if _, isKnown := keyOf[string(ch)]; isKnown {
			continue
		}
		g, has := glyphCut(b, top, bot, r.cuts[i*2], r.cuts[i*2+1])
		if !has {
			continue
		}
		out[g.Key()] = string(ch)
	}

	// 검증: 배운 걸 합쳐 다시 읽었을 때 입력한 문자열이 그대로 나와야 한다
	merged := GlyphDict{}
	for k, v := range known {
		merged[k] = v
	}
	for k, v := range out {
		merged[k] = v
	}
	got, okRead := RecognizeGlyphs(b, merged)
	if !okRead || got != string(rs) {
		return nil, fmt.Errorf("검증 실패: 다시 읽으니 '%s' 로 나옵니다", got)
	}
	return out, nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
