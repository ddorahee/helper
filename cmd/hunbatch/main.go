package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"example.com/m/automation"
)

// dataset 폴더의 모든 png를 OCR 자동 라벨링 → YOLO 데이터셋 생성
// 사용법: hunbatch <dataset폴더> <출력폴더>
//   출력: <출력>/images/train|val, labels/train|val, dataset.yaml
func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: hunbatch <datasetDir> <outDir>")
		return
	}
	srcDir, outDir := os.Args[1], os.Args[2]

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		fmt.Printf("폴더 읽기 실패: %v\n", err)
		return
	}
	var pngs []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			pngs = append(pngs, e.Name())
		}
	}
	sort.Strings(pngs)
	if len(pngs) == 0 {
		fmt.Println("png 없음")
		return
	}
	fmt.Printf("총 %d장 처리 시작 (OCR이라 장당 수초~수십초)\n", len(pngs))

	for _, d := range []string{"images/train", "images/val", "labels/train", "labels/val"} {
		os.MkdirAll(filepath.Join(outDir, d), 0755)
	}

	om := automation.NewOCRManager(nil)
	totalBoxes, labeled, empty := 0, 0, 0

	for i, name := range pngs {
		f, err := os.Open(filepath.Join(srcDir, name))
		if err != nil {
			continue
		}
		src, err := png.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		b := src.Bounds()
		W, H := b.Dx(), b.Dy()

		words, err := om.RecognizeYellowNames(src)
		if err != nil {
			fmt.Printf("[%d/%d] %s OCR실패: %v\n", i+1, len(pngs), name, err)
			continue
		}

		var lines []string
		for _, w := range words {
			cls := classify(w.Text) // 0=훈족(적), 1=인도정예병사(아군), -1=무관
			if cls < 0 {
				continue
			}
			// 좌측 UI 패널 제외 (게임영역 9%~72%)
			if w.X < float64(W)*0.09 || w.X > float64(W)*0.72 {
				continue
			}
			// 이름표 박스 + 아래 몬스터 몸체 (이름표 높이 + 38px)
			bx := w.X
			by := w.Y
			bw := w.Width
			bh := w.Height + 38
			cx := (bx + bw/2) / float64(W)
			cy := (by + bh/2) / float64(H)
			nw := bw / float64(W)
			nh := bh / float64(H)
			lines = append(lines, fmt.Sprintf("%d %.6f %.6f %.6f %.6f", cls, cx, cy, nw, nh))
		}

		split := "train"
		if i%5 == 4 {
			split = "val"
		}
		base := strings.TrimSuffix(name, ".png")
		copyFile(filepath.Join(srcDir, name), filepath.Join(outDir, "images", split, name))
		os.WriteFile(filepath.Join(outDir, "labels", split, base+".txt"), []byte(strings.Join(lines, "\n")), 0644)

		if len(lines) > 0 {
			labeled++
			totalBoxes += len(lines)
		} else {
			empty++
		}
		if (i+1)%10 == 0 || i == len(pngs)-1 {
			fmt.Printf("[%d/%d] 누적: 라벨 %d장 박스 %d개 빈 %d장\n", i+1, len(pngs), labeled, totalBoxes, empty)
		}
	}

	yaml := fmt.Sprintf("path: %s\ntrain: images/train\nval: images/val\nnames:\n  0: hunjok\n  1: indo\n", filepath.ToSlash(outDir))
	os.WriteFile(filepath.Join(outDir, "dataset.yaml"), []byte(yaml), 0644)

	fmt.Printf("\n=== 완료 ===\n라벨 %d장 / 빈 %d장 / 총 박스 %d개\n출력: %s\n", labeled, empty, totalBoxes, outDir)
}

// classify OCR 텍스트로 클래스 판정. 0=훈족(적), 1=인도정예병사(아군), -1=무관
// 인도정예병사를 먼저 검사 (오분류 방지)
func classify(text string) int {
	indoKeys := []string{"인도", "정예", "도정", "예병", "정몌", "인노", "도졍"}
	for _, k := range indoKeys {
		if strings.Contains(text, k) {
			return 1
		}
	}
	hunKeys := []string{"훈족", "족기", "기마", "주술", "주풀", "창범", "창병", "족주", "쪽기", "준족", "흔족"}
	for _, k := range hunKeys {
		if strings.Contains(text, k) {
			return 0
		}
	}
	return -1
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	os.WriteFile(dst, data, 0644)
}

var _ = image.Black
