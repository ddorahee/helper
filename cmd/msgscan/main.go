package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"example.com/m/automation"
)

// dataset의 모든 png를 OCR하여 방향/보스/포탈 메시지가 있는 프레임을 찾는다.
// usage: msgscan <datasetDir>
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: msgscan <datasetDir>")
		return
	}
	dir := os.Args[1]
	entries, _ := os.ReadDir(dir)
	var pngs []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			pngs = append(pngs, e.Name())
		}
	}
	sort.Strings(pngs)

	keywords := []string{"동쪽", "서쪽", "남쪽", "북쪽", "쿠라칸", "등장", "포탈", "입장", "나가", "몰려", "처치", "획득", "출현", "방향", "보스"}
	om := automation.NewOCRManager(nil)

	fmt.Printf("총 %d장 OCR 스캔 시작\n\n", len(pngs))
	for i, name := range pngs {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		src, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		// 화면 중앙 영역 텍스트 위치 인식 (시스템 메시지)
		words, err := om.RecognizeWithPositions(src)
		if err != nil {
			continue
		}
		var hits []string
		for _, w := range words {
			for _, k := range keywords {
				if strings.Contains(w.Text, k) {
					hits = append(hits, fmt.Sprintf("'%s'@(%.0f,%.0f)", w.Text, w.X, w.Y))
					break
				}
			}
		}
		if len(hits) > 0 {
			fmt.Printf("[%d] %s → %s\n", i, name, strings.Join(hits, " "))
		}
		if (i+1)%20 == 0 {
			fmt.Printf("  ...(%d/%d 진행)\n", i+1, len(pngs))
		}
	}
	fmt.Println("\n=== 스캔 완료 ===")
}
