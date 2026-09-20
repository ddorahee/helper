// glyphteach ─ 크롭 이미지 파일로 글자를 가르쳐 내장 사전(automation/glyph_dict.json)에 넣는다.
//
// UI 의 '글자 학습' 은 배운 걸 config/glyphs.json 에 넣는다(그 PC에만 남는다).
// 이 도구는 exe 에 박히는 내장 사전을 직접 키우는 용도다 — 사용자가 보내준
// 맵/닉네임 크롭 이미지로 글자를 추가할 때 쓴다.
//
//	go run ./cmd/glyphteach map  11.png  2 대야산기슭
//	go run ./cmd/glyphteach nick 111.png 2 명멸멸
//
// 인자: <map|nick> <png> <배율> <정답 문자열>
// 배율은 크롭을 몇 배로 확대해 저장했는지 (UI 미리보기는 2배, 창 목록 닉네임은 3배).
// 이미지는 GlyphMapRegion / GlyphNickRegion 을 그대로 자른 것이어야 한다.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"sort"
	"strconv"
	"strings"

	"example.com/m/automation"
)

const dictPath = "automation/glyph_dict.json"

// shrink 확대 저장된 크롭을 원본 픽셀로 되돌린다 (확대는 최근접 복제라 정확히 복원된다)
func shrink(src image.Image, scale int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx()/scale, b.Dy()/scale
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+x*scale, b.Min.Y+y*scale))
		}
	}
	return dst
}

func main() {
	if len(os.Args) < 5 {
		fmt.Println("사용법: glyphteach <map|nick> <png> <배율> <정답 문자열>")
		os.Exit(2)
	}
	kind, path, text := os.Args[1], os.Args[2], strings.Join(os.Args[4:], " ")
	scale, err := strconv.Atoi(os.Args[3])
	if err != nil || scale < 1 {
		fmt.Println("배율이 잘못됐다:", os.Args[3])
		os.Exit(2)
	}

	fh, err := os.Open(path)
	if err != nil {
		fmt.Println("읽기 실패:", err)
		os.Exit(1)
	}
	src, err := png.Decode(fh)
	fh.Close()
	if err != nil {
		fmt.Println("PNG 디코드 실패:", err)
		os.Exit(1)
	}
	img := shrink(src, scale)
	b := img.Bounds()
	reg := automation.GlyphRegion{X0: 0, Y0: 0, X1: b.Dx(), Y1: b.Dy(), White: kind == "nick"}
	bin := automation.BinarizeGlyph(img, reg)
	fmt.Printf("크롭 %dx%d (원본 %dx%d), 극성=%s\n", src.Bounds().Dx(), src.Bounds().Dy(), b.Dx(), b.Dy(),
		map[bool]string{true: "흰 글자", false: "어두운 글자"}[reg.White])

	dict := automation.LoadGlyphDict()
	if got, ok := automation.RecognizeGlyphs(bin, dict); ok {
		fmt.Printf("이미 읽힌다: '%s'\n", got)
		if got != text {
			fmt.Printf("  ! 입력한 '%s' 와 다르다 — 확인 필요\n", text)
			os.Exit(1)
		}
		return
	}

	learned, err := automation.FitGlyphs(bin, text, dict)
	if err != nil {
		fmt.Println("학습 실패:", err)
		os.Exit(1)
	}
	fmt.Printf("새 글리프 %d개\n", len(learned))

	raw, err := os.ReadFile(dictPath)
	if err != nil {
		fmt.Println("사전 읽기 실패:", err)
		os.Exit(1)
	}
	cur := map[string]string{}
	if err := json.Unmarshal(raw, &cur); err != nil {
		fmt.Println("사전 파싱 실패:", err)
		os.Exit(1)
	}
	added := 0
	for k, v := range learned {
		if old, ok := cur[k]; ok && old != v {
			fmt.Printf("  !! 비트맵 충돌: 기존 '%s' vs 새 '%s' — 저장 안 함\n", old, v)
			os.Exit(1)
		}
		if _, ok := cur[k]; !ok {
			added++
		}
		cur[k] = v
	}
	out, _ := json.MarshalIndent(cur, "", " ")
	if err := os.WriteFile(dictPath, out, 0644); err != nil {
		fmt.Println("사전 저장 실패:", err)
		os.Exit(1)
	}
	seen := map[string]bool{}
	var cs []string
	for _, v := range cur {
		if !seen[v] {
			seen[v] = true
			cs = append(cs, v)
		}
	}
	sort.Strings(cs)
	fmt.Printf("저장: %s — %d글리프 추가, 총 %d글리프 / %d글자\n  %s\n",
		dictPath, added, len(cur), len(cs), strings.Join(cs, ""))
}
