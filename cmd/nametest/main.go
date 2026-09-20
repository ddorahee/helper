package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"example.com/m/automation"
)

// dataset 이미지에 OCR을 돌려 적 이름(훈족기마병사/쿠라칸/인도정예병사)이 읽히는지 검증
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: nametest <png경로>")
		return
	}
	path := os.Args[1]

	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("열기 실패: %v\n", err)
		return
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		fmt.Printf("디코드 실패: %v\n", err)
		return
	}

	om := automation.NewOCRManager(nil)
	words, err := om.RecognizeWithPositions(src)
	if err != nil {
		fmt.Printf("OCR 실패: %v\n", err)
		return
	}

	fmt.Printf("=== OCR 인식 단어 %d개 ===\n", len(words))
	keywords := []string{"훈족", "기마", "병사", "쿠라칸", "쿠라", "인도", "정예"}
	for _, w := range words {
		hit := ""
		for _, k := range keywords {
			if strings.Contains(w.Text, k) {
				hit = " <<< 매칭"
				break
			}
		}
		fmt.Printf("  '%s' @(%.0f,%.0f) %.0fx%.0f%s\n", w.Text, w.X, w.Y, w.Width, w.Height, hit)
	}

	// 박스 시각화
	b := src.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, src, b.Min, draw.Src)
	for _, w := range words {
		drawRect(rgba, int(w.X), int(w.Y), int(w.Width), int(w.Height), color.RGBA{0, 255, 0, 255})
	}
	out, _ := os.Create(os.Getenv("USERPROFILE") + "\\Desktop\\nametest_out.png")
	png.Encode(out, rgba)
	out.Close()
	fmt.Println("\n박스 이미지 저장: Desktop\\nametest_out.png")
}

func drawRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for i := 0; i < 2; i++ {
		for px := x; px < x+w; px++ {
			img.Set(px, y+i, c)
			img.Set(px, y+h-1-i, c)
		}
		for py := y; py < y+h; py++ {
			img.Set(x+i, py, c)
			img.Set(x+w-1-i, py, c)
		}
	}
}
