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

// dataset 이미지에서 노란 이름표 OCR → "훈족" 위치 검출 → 박스 시각화
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: hunlabel <png>")
		return
	}
	f, _ := os.Open(os.Args[1])
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		fmt.Printf("디코드 실패: %v\n", err)
		return
	}

	om := automation.NewOCRManager(nil)
	words, err := om.RecognizeYellowNames(src)
	if err != nil {
		fmt.Printf("OCR 실패: %v\n", err)
		return
	}

	fmt.Printf("=== 인식 단어 %d개 ===\n", len(words))
	hunCount := 0
	b := src.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, src, b.Min, draw.Src)

	for _, w := range words {
		isHun := strings.Contains(w.Text, "훈족") || strings.Contains(w.Text, "훈") || strings.Contains(w.Text, "족기") || strings.Contains(w.Text, "주술")
		mark := ""
		col := color.RGBA{120, 120, 255, 255}
		if isHun {
			mark = " <<< 훈족(적)"
			hunCount++
			col = color.RGBA{255, 30, 30, 255}
			// 이름표 아래 몬스터 몸체까지 박스
			drawRect(rgba, int(w.X), int(w.Y), int(w.Width), int(w.Height)+40, col)
		}
		fmt.Printf("  '%s' @(%.0f,%.0f) %.0fx%.0f%s\n", w.Text, w.X, w.Y, w.Width, w.Height, mark)
	}
	fmt.Printf("\n훈족(적) 매칭: %d개\n", hunCount)

	out, _ := os.Create(os.Getenv("USERPROFILE") + "\\Desktop\\hunlabel_out.png")
	png.Encode(out, rgba)
	out.Close()
	fmt.Println("박스 이미지: Desktop\\hunlabel_out.png")
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
