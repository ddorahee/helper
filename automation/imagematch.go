package automation

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// resizeImage 단순 nearest-neighbor 리사이즈 (다중 스케일 매칭용)
func resizeImage(src image.Image, scale float64) *image.RGBA {
	bounds := src.Bounds()
	w := int(float64(bounds.Dx()) * scale)
	h := int(float64(bounds.Dy()) * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := int(float64(x)/scale) + bounds.Min.X
			sy := int(float64(y)/scale) + bounds.Min.Y
			if sx >= bounds.Max.X {
				sx = bounds.Max.X - 1
			}
			if sy >= bounds.Max.Y {
				sy = bounds.Max.Y - 1
			}
			r, g, b, a := src.At(sx, sy).RGBA()
			dst.Set(x, y, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)})
		}
	}
	return dst
}

// FindImageMultiScale 여러 스케일로 needle을 변환해 haystack에서 검색.
// 게임 해상도가 달라 다이얼로그가 스케일링된 경우에도 매칭 가능.
// 매칭되면 원본 haystack 좌표 + 사용된 스케일 반환.
// scales nil이면 [0.6, 0.7, 0.8, 0.9, 1.0, 1.1, 1.25, 1.5] 시도.
func FindImageMultiScale(haystack image.Image, needle image.Image, scales []float64,
	perPixelTolerance int, minMatchRatio float64) (x, y int, scale float64, found bool) {
	if scales == nil {
		scales = []float64{1.0, 0.9, 1.1, 0.8, 1.25, 0.7, 1.5, 0.6}
	}
	for _, s := range scales {
		var n image.Image = needle
		if s != 1.0 {
			n = resizeImage(needle, s)
		}
		nx, ny, ok := FindImageInRegion(haystack, n, nil, perPixelTolerance, minMatchRatio)
		if ok {
			return nx, ny, s, true
		}
	}
	return 0, 0, 0, false
}

// LoadPNG 디스크에서 PNG 이미지 로드
func LoadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// pixelDiff 두 픽셀의 RGB 차이 절대합 (0~765)
func pixelDiff(r1, g1, b1, r2, g2, b2 uint32) int {
	dr := int(r1>>8) - int(r2>>8)
	if dr < 0 {
		dr = -dr
	}
	dg := int(g1>>8) - int(g2>>8)
	if dg < 0 {
		dg = -dg
	}
	db := int(b1>>8) - int(b2>>8)
	if db < 0 {
		db = -db
	}
	return dr + dg + db
}

// FindImageInRegion haystack에서 needle 이미지를 찾아 좌상단 좌표 반환.
// region: haystack 내 검색 영역 (절대 좌표). nil이면 haystack 전체.
// 픽셀당 최대 RGB 차이 합을 perPixelTolerance, 전체 일치 비율을 minMatchRatio로 제어.
// 빠른 조기 탈출을 위해 픽셀 일부만 샘플링하는 stride 적용.
func FindImageInRegion(haystack image.Image, needle image.Image, region *image.Rectangle, perPixelTolerance int, minMatchRatio float64) (int, int, bool) {
	hb := haystack.Bounds()
	nb := needle.Bounds()
	nw := nb.Dx()
	nh := nb.Dy()
	if nw == 0 || nh == 0 {
		return 0, 0, false
	}

	searchRect := hb
	if region != nil {
		searchRect = hb.Intersect(*region)
	}
	if searchRect.Dx() < nw || searchRect.Dy() < nh {
		return 0, 0, false
	}

	// 샘플링 스트라이드: 너무 많은 픽셀은 비교 비용 큼
	stride := 2
	if nw < 16 || nh < 16 {
		stride = 1
	}

	totalSamples := 0
	for sy := 0; sy < nh; sy += stride {
		for sx := 0; sx < nw; sx += stride {
			totalSamples++
		}
	}
	if totalSamples == 0 {
		return 0, 0, false
	}
	requiredMatches := int(float64(totalSamples) * minMatchRatio)

	maxX := searchRect.Max.X - nw
	maxY := searchRect.Max.Y - nh

	for y := searchRect.Min.Y; y <= maxY; y++ {
		for x := searchRect.Min.X; x <= maxX; x++ {
			matches := 0
			misses := 0
			maxMisses := totalSamples - requiredMatches
			ok := true

			for sy := 0; sy < nh && ok; sy += stride {
				for sx := 0; sx < nw; sx += stride {
					hr, hg, hb_, _ := haystack.At(x+sx, y+sy).RGBA()
					nr, ng_, nb_, _ := needle.At(nb.Min.X+sx, nb.Min.Y+sy).RGBA()
					if pixelDiff(hr, hg, hb_, nr, ng_, nb_) <= perPixelTolerance {
						matches++
					} else {
						misses++
						if misses > maxMisses {
							ok = false
							break
						}
					}
				}
			}
			if ok && matches >= requiredMatches {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}
