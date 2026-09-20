package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"

	"example.com/m/automation"
)

// 대표 프레임들의 우하단 현재 좌표(X,Y)를 읽어 방향-좌표 관계 분석
func main() {
	dir := `C:\Users\Home\Desktop\dataset`
	// [프레임설명] 파일명
	targets := []struct{ desc, file string }{
		{"입장", "baram_20260607_165151.429_61554.png"},
		{"남쪽몹", "baram_20260607_165215.430_61554.png"},
		{"서쪽몹", "baram_20260607_165233.432_61554.png"},
		{"북쪽몹", "baram_20260607_165239.420_61554.png"},
		{"동쪽몹", "baram_20260607_165245.425_61554.png"},
		{"쿠라칸등장", "baram_20260607_165303.426_61554.png"},
		{"쿠라칸처치", "baram_20260607_165309.430_61554.png"},
		{"2회입장", "baram_20260607_165448.434_61554.png"},
		{"끝부분", "baram_20260607_170145.432_61554.png"},
	}

	om := automation.NewOCRManager(nil)
	for _, t := range targets {
		f, err := os.Open(filepath.Join(dir, t.file))
		if err != nil {
			fmt.Printf("%s: 열기 실패\n", t.desc)
			continue
		}
		src, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		rgba := image.NewRGBA(src.Bounds())
		b := src.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				rgba.Set(x, y, src.At(x, y))
			}
		}
		coords, dbg, err := om.ReadCoordinatesFromImage(rgba)
		if err != nil {
			fmt.Printf("[%s] 좌표 읽기 실패: %v (dbg=%v)\n", t.desc, err, dbg)
		} else {
			fmt.Printf("[%s] 현재좌표 = (%d, %d)\n", t.desc, coords.X, coords.Y)
		}
	}
}
