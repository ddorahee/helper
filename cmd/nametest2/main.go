package main

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"example.com/m/automation"
)

// 크롭된 이름표 이미지를 RecognizeText로 OCR (노란 글씨 읽히는지 검증)
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: nametest2 <png>")
		return
	}
	f, err := os.Open(os.Args[1])
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

	// image.Image → *image.RGBA 보장
	b := src.Bounds()
	rgba := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			rgba.Set(x, y, src.At(x, y))
		}
	}

	om := automation.NewOCRManager(nil)
	txt, err := om.RecognizeText(rgba)
	if err != nil {
		fmt.Printf("OCR 실패: %v\n", err)
		return
	}
	fmt.Printf("RecognizeText 결과: '%s'\n", txt)
}
