package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"example.com/m/automation"
	"golang.org/x/image/draw"
)

// 노란 글씨 이름표 이진화 후 OCR
// 노란 픽셀 → 흰색, 배경 → 검정 (helper의 흰색필터와 호환)
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: nametest3 <png>")
		return
	}
	f, _ := os.Open(os.Args[1])
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		fmt.Printf("디코드 실패: %v\n", err)
		return
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	// 노란 글씨 이진화
	bin := image.NewRGBA(image.Rect(0, 0, w, h))
	yellowCount := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			r8, g8, b8 := int(r>>8), int(g>>8), int(bl>>8)
			isYellow := r8 > 140 && g8 > 110 && b8 < 130 && (r8+g8)/2-b8 > 35
			if isYellow {
				bin.Set(x, y, color.White)
				yellowCount++
			} else {
				bin.Set(x, y, color.Black)
			}
		}
	}
	fmt.Printf("노란 픽셀 %d개\n", yellowCount)

	// 4배 확대
	big := image.NewRGBA(image.Rect(0, 0, w*4, h*4))
	draw.NearestNeighbor.Scale(big, big.Bounds(), bin, bin.Bounds(), draw.Over, nil)

	// 이진화 결과 저장 (확인용)
	out, _ := os.Create(os.Getenv("USERPROFILE") + "\\Desktop\\nametag_bin.png")
	png.Encode(out, big)
	out.Close()

	om := automation.NewOCRManager(nil)
	txt, err := om.RecognizeText(big)
	if err != nil {
		fmt.Printf("OCR 실패: %v\n", err)
		return
	}
	fmt.Printf("이진화 후 OCR 결과: '%s'\n", txt)
}
