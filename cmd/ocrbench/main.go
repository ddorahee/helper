// ocrbench ─ 기존 PowerShell WinRT OCR vs 글리프 매칭 A/B 비교.
//
// 바탕화면 dataset-* 의 실제 게임 캡처를 같은 크롭 영역으로 양쪽에 먹여
// 정확도와 속도를 잰다. 정답은 글리프 결과를 쓴다 — dataset 26종 맵 화면을
// 눈으로 전부 확인해 라벨을 붙였고(automation/glyph_test.go 참고),
// 글리프는 사전에 없는 글자가 하나라도 있으면 답을 내지 않으므로
// 읽힌 값은 맞다고 볼 수 있다.
//
//	go run ./cmd/ocrbench          # 기본: 12장마다 1장 추출
//	go run ./cmd/ocrbench 30       # 30장마다 1장 (더 빠르게)
//
// OCR 이 1회당 ~700ms 걸리므로 표본을 너무 늘리지 말 것.
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"example.com/m/automation"
)

const dsRoot = `C:\Users\Home\Desktop`

var dsDirs = []string{"dataset-사막", "dataset-왕무", "dataset-왕궁", "dataset-나타"}

func load(p string) image.Image {
	fh, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer fh.Close()
	im, err := png.Decode(fh)
	if err != nil {
		return nil
	}
	return im
}

// clientBox dataset 캡처는 창 전체(타이틀바 포함)다. 게임 클라이언트가 1600x900 고정이라
// 좌우 테두리 두께로 오프셋을 역산한다. 실제 동작에서는 GetClientOffset 이 준다.
func clientBox(im image.Image) (offX, offY, cw, ch int) {
	b := im.Bounds()
	offX = (b.Dx() - 1600) / 2
	return offX, b.Dy() - 900 - offX, 1600, 900
}

// hangulOnly 기존 OCR 경로는 한글만 남기므로, 숫자·하이픈을 뺀 정답과도 비교한다
func hangulOnly(s string) string {
	var out []rune
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			out = append(out, r)
		}
	}
	return string(out)
}

func main() {
	step := 12
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil && v > 0 {
			step = v
		}
	}

	var files []string
	for _, d := range dsDirs {
		fs, _ := filepath.Glob(filepath.Join(dsRoot, d, "*.png"))
		files = append(files, fs...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Printf("dataset 폴더가 없습니다 (%s)\n", dsRoot)
		return
	}

	dict := automation.LoadGlyphDict()
	var ocrMapDur, ocrNickDur, glyMapDur, glyNickDur time.Duration
	var ocrMapOK, ocrMapLoose, ocrNickOK, n int
	var skipped int
	var wrong []string

	om := automation.NewOCRManager(nil)
	for i := 0; i < len(files); i += step {
		im := load(files[i])
		if im == nil {
			continue
		}
		sub, ok := im.(interface {
			SubImage(image.Rectangle) image.Image
		})
		if !ok {
			continue
		}
		offX, offY, cw, ch := clientBox(im)
		mr := automation.GlyphMapRegion(im, offX, offY, cw, ch)
		nr := automation.GlyphNickRegion(offX, offY, cw, ch)

		// --- 글리프 ---
		t0 := time.Now()
		wantMap, mapOK := automation.RecognizeGlyphs(automation.BinarizeGlyph(im, mr), dict)
		glyMapDur += time.Since(t0)
		t0 = time.Now()
		wantNick, nickOK := automation.RecognizeGlyphs(automation.BinarizeGlyph(im, nr), dict)
		glyNickDur += time.Since(t0)
		if !mapOK || !nickOK {
			// 글자가 화면에 없는 프레임(맵 전환 암전·커서 가림) — 비교 대상에서 뺀다
			skipped++
			continue
		}
		n++

		// --- 기존 OCR (같은 크롭 영역) ---
		t0 = time.Now()
		gotMap, _ := om.RecognizeMapName(sub.SubImage(image.Rect(mr.X0, mr.Y0, mr.X1, mr.Y1)))
		ocrMapDur += time.Since(t0)
		t0 = time.Now()
		gotNick, _ := om.RecognizeText(sub.SubImage(image.Rect(nr.X0, nr.Y0, nr.X1, nr.Y1)))
		ocrNickDur += time.Since(t0)

		if gotMap == wantMap {
			ocrMapOK++
			ocrMapLoose++
		} else if gotMap == hangulOnly(wantMap) {
			ocrMapLoose++
		} else if len(wrong) < 12 {
			wrong = append(wrong, fmt.Sprintf("  맵  %-20s → %s", wantMap, gotMap))
		}
		if gotNick == wantNick {
			ocrNickOK++
		} else if len(wrong) < 12 {
			wrong = append(wrong, fmt.Sprintf("  닉  %-20s → %s", wantNick, gotNick))
		}
	}

	if n == 0 {
		fmt.Println("비교할 표본이 없습니다 (사전에 없는 글자뿐일 수 있음)")
		return
	}
	pct := func(a int) float64 { return float64(a) * 100 / float64(n) }
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / float64(n) / 1000 }

	fmt.Printf("표본 %d장 (전체 %d장에서 %d장 간격, 판독 불가 %d장 제외)\n\n", n, len(files), step, skipped)
	fmt.Printf("%-12s %-22s %-22s\n", "", "기존 OCR", "글리프")
	fmt.Printf("%-12s %-22s %-22s\n", "맵 정확도",
		fmt.Sprintf("%d/%d = %.1f%%", ocrMapOK, n, pct(ocrMapOK)),
		fmt.Sprintf("%d/%d = 100.0%% (기준)", n, n))
	fmt.Printf("%-12s %-22s\n", "  한글만 비교",
		fmt.Sprintf("%d/%d = %.1f%%", ocrMapLoose, n, pct(ocrMapLoose)))
	fmt.Printf("%-12s %-22s %-22s\n", "닉 정확도",
		fmt.Sprintf("%d/%d = %.1f%%", ocrNickOK, n, pct(ocrNickOK)),
		fmt.Sprintf("%d/%d = 100.0%% (기준)", n, n))
	fmt.Printf("%-12s %-22s %-22s\n", "맵 속도",
		fmt.Sprintf("%.0f ms/회", ms(ocrMapDur)), fmt.Sprintf("%.3f ms/회", ms(glyMapDur)))
	fmt.Printf("%-12s %-22s %-22s\n", "닉 속도",
		fmt.Sprintf("%.0f ms/회", ms(ocrNickDur)), fmt.Sprintf("%.3f ms/회", ms(glyNickDur)))
	fmt.Printf("%-12s %-22s %-22s\n", "합계 속도",
		fmt.Sprintf("%.0f ms/회", ms(ocrMapDur+ocrNickDur)),
		fmt.Sprintf("%.3f ms/회", ms(glyMapDur+glyNickDur)))
	if len(wrong) > 0 {
		fmt.Printf("\nOCR 오답 예시:\n%s\n", strings.Join(wrong, "\n"))
	}
}
