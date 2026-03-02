package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	_ "image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/image/draw"
)

// Production-identical makeOCRVariants simulation test
// Variant order (same as production):
// 1) 4x wsat (white+low saturation) - best for "바이" type chars
// 2) 4x w230 (strict white) - cleanest results
// 3) 4x raw (no binarization) - full color context for OCR
// 4) 6x wsat - fallback for wider text
// KEY: NO extra sub-crop. Full CaptureNameRegion image passed directly.

func main() {
	srcDir := `C:\Users\alsrl\OneDrive\바탕 화면\새 폴더 (5)`
	outDir := `C:\Users\alsrl\OneDrive\바탕 화면\baram\cmd\ocrtest\output`
	os.MkdirAll(outDir, 0o755)

	imageFiles := []string{"1.png", "2.png", "3.png"}
	expectedNames := map[string]string{
		"1.png": "타라폼",
		"2.png": "재모바이",
		"3.png": "데브섹옵스",
	}

	fmt.Println("=== Production makeOCRVariants Simulation (v2) ===")
	fmt.Println("Variants: 4x_wsat → 4x_w230 → 4x_raw → 6x_wsat")
	fmt.Println("Early return: ≥3 Korean chars")
	fmt.Println()

	allMatch := true

	for _, fname := range imageFiles {
		srcPath := filepath.Join(srcDir, fname)
		expected := expectedNames[fname]
		fmt.Printf("=== %s (expected: %s) ===\n", fname, expected)

		img, err := loadImage(srcPath)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			allMatch = false
			continue
		}

		// NO extra sub-cropping — pass full image (simulates CaptureNameRegion output)
		type variantSpec struct {
			name    string
			process func(image.Image) *image.RGBA
		}

		variantSpecs := []variantSpec{
			{"4x_wsat", func(src image.Image) *image.RGBA {
				scaled := scaleNN(src, 4)
				return applyWsat(scaled)
			}},
			{"4x_w230", func(src image.Image) *image.RGBA {
				scaled := scaleNN(src, 4)
				return applyW230(scaled)
			}},
			{"4x_raw", func(src image.Image) *image.RGBA {
				return scaleNN(src, 4)
			}},
			{"6x_wsat", func(src image.Image) *image.RGBA {
				scaled := scaleNN(src, 6)
				return applyWsat(scaled)
			}},
		}

		bestKorean := ""
		bestVariant := ""

		for i, v := range variantSpecs {
			processed := v.process(img)
			padded := addPad(processed, 20)

			outPath := filepath.Join(outDir, fmt.Sprintf("%s_v2_%s.png",
				strings.TrimSuffix(fname, filepath.Ext(fname)), v.name))
			savePNG(outPath, padded)

			result, err := runOCR(outPath, "ko")
			if err != nil {
				fmt.Printf("  [%d] %-10s -> ERROR: %v\n", i, v.name, err)
				continue
			}

			korean := extractKorean(result)
			dist := levenshteinStr(korean, expected)

			marker := ""
			if korean == expected {
				marker = " *** EXACT ***"
			} else if dist <= 1 {
				marker = " ** CLOSE **"
			} else if dist <= 2 {
				marker = " * NEAR *"
			}

			fmt.Printf("  [%d] %-10s -> raw=%q korean=%q dist=%d%s\n",
				i, v.name, result, korean, dist, marker)

			// RecognizeText logic: keep longest Korean ≥2 chars, early return at ≥3
			if len([]rune(korean)) >= 2 && len([]rune(korean)) > len([]rune(bestKorean)) {
				bestKorean = korean
				bestVariant = v.name
			}
			if len([]rune(bestKorean)) >= 3 {
				fmt.Printf("  → Early return: %q from %s\n", bestKorean, bestVariant)
				break
			}
		}

		if bestKorean == "" {
			fmt.Printf("  → FINAL: no Korean detected\n")
			allMatch = false
			fmt.Println()
			continue
		}

		// Simulate main.go matching logic
		bestDist := levenshteinStr(bestKorean, expected)
		matched := bestDist <= 2

		// Sliding window matching (if OCR result is longer)
		bestRunes := []rune(bestKorean)
		expectedRunes := []rune(expected)
		if len(bestRunes) > len(expectedRunes) {
			for start := 0; start <= len(bestRunes)-len(expectedRunes); start++ {
				sub := bestRunes[start : start+len(expectedRunes)]
				d := levenshteinRunes(sub, expectedRunes)
				if d <= 1 {
					matched = true
					fmt.Printf("  → SLIDING WINDOW: %q[%d:%d]=%q vs %q dist=%d\n",
						bestKorean, start, start+len(expectedRunes), string(sub), expected, d)
					break
				}
			}
		}

		if matched {
			fmt.Printf("  → ✓ MATCH (best=%q dist=%d from %s)\n", bestKorean, bestDist, bestVariant)
		} else {
			fmt.Printf("  → ✗ NO MATCH (best=%q dist=%d from %s)\n", bestKorean, bestDist, bestVariant)
			allMatch = false
		}
		fmt.Println()
	}

	fmt.Println("=== RESULT ===")
	if allMatch {
		fmt.Println("  ✓ ALL 3/3 MATCH — 100% accuracy!")
	} else {
		fmt.Println("  ✗ Some images failed to match")
	}
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func scaleNN(src image.Image, scale int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*scale, b.Dy()*scale))
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func applyWsat(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rv, gv, bv, _ := src.At(x, y).RGBA()
			r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
			minC := r8
			if g8 < minC { minC = g8 }
			if b8 < minC { minC = b8 }
			maxC := r8
			if g8 > maxC { maxC = g8 }
			if b8 > maxC { maxC = b8 }
			if maxC > 200 && (maxC-minC) < 40 {
				dst.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				dst.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	return dst
}

func applyW230(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rv, gv, bv, _ := src.At(x, y).RGBA()
			r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
			if r8 > 230 && g8 > 230 && b8 > 230 {
				dst.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				dst.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	return dst
}

func addPad(src *image.RGBA, pad int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w+2*pad, h+2*pad))
	for y := 0; y < h+2*pad; y++ {
		for x := 0; x < w+2*pad; x++ {
			dst.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x+pad, y+pad, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil { return err }
	defer f.Close()
	return png.Encode(f, img)
}

func extractKorean(s string) string {
	var filtered []rune
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 { filtered = append(filtered, r) }
	}
	return string(filtered)
}

func levenshteinStr(a, b string) int { return levenshteinRunes([]rune(a), []rune(b)) }

func levenshteinRunes(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 { return lb }
	if lb == 0 { return la }
	m := make([][]int, la+1)
	for i := range m { m[i] = make([]int, lb+1); m[i][0] = i }
	for j := 0; j <= lb; j++ { m[0][j] = j }
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			c := 1; if a[i-1] == b[j-1] { c = 0 }
			v := m[i-1][j] + 1
			if m[i][j-1]+1 < v { v = m[i][j-1] + 1 }
			if m[i-1][j-1]+c < v { v = m[i-1][j-1] + c }
			m[i][j] = v
		}
	}
	return m[la][lb]
}

func runOCR(imagePath string, lang string) (string, error) {
	absPath, _ := filepath.Abs(imagePath)
	psScript := "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8\n" +
		"Add-Type -AssemblyName System.Runtime.WindowsRuntime\n" +
		"$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation" + "`" + "1' })[0]\n" +
		"Function Await($WinRtTask, $ResultType) { $asTask = $asTaskGeneric.MakeGenericMethod($ResultType); $netTask = $asTask.Invoke($null, @($WinRtTask)); $netTask.Wait(-1) | Out-Null; $netTask.Result }\n" +
		"$null = [Windows.Media.Ocr.OcrEngine, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]\n" +
		"$null = [Windows.Graphics.Imaging.BitmapDecoder, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]\n" +
		"$null = [Windows.Storage.Streams.RandomAccessStream, Windows.Storage.Streams, ContentType=WindowsRuntime]\n" +
		"$null = [Windows.Storage.StorageFile, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]\n" +
		"$null = [Windows.Globalization.Language, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]\n" +
		"$lang = New-Object Windows.Globalization.Language('" + lang + "')\n" +
		"$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($lang)\n" +
		"if ($null -eq $engine) { Write-Error '" + lang + " OCR not available'; exit 1 }\n" +
		"$path = '" + strings.ReplaceAll(absPath, "'", "''") + "'\n" +
		"$sf = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($path)) ([Windows.Storage.StorageFile])\n" +
		"$stream = Await ($sf.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])\n" +
		"$bd = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])\n" +
		"$bm = Await ($bd.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])\n" +
		"$r = Await ($engine.RecognizeAsync($bm)) ([Windows.Media.Ocr.OcrResult])\n" +
		"$stream.Dispose()\n" +
		"Write-Output $r.Text\n"

	psFile, _ := os.CreateTemp("", "ocrtest-*.ps1")
	psPath := psFile.Name()
	defer os.Remove(psPath)
	psFile.Write([]byte{0xEF, 0xBB, 0xBF})
	psFile.WriteString(psScript)
	psFile.Close()

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", psPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout; cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ps failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
	text := strings.TrimSpace(stdout.String())
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")
	return text, nil
}
