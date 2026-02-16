package automation

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/image/draw"
)

// OCRConfig OCR 크롭 영역 설정
type OCRConfig struct {
	NameRegionX      int
	NameRegionY      int
	NameRegionWidth  int
	NameRegionHeight int
}

// OCRManager Windows 내장 OCR 관리자
type OCRManager struct {
	wm     *WindowManager
	config OCRConfig
}

// NewOCRManager 새로운 OCR 매니저 생성
func NewOCRManager(wm *WindowManager) *OCRManager {
	return &OCRManager{
		wm: wm,
		config: OCRConfig{
			NameRegionX:      0,
			NameRegionY:      5,
			NameRegionWidth:  180,
			NameRegionHeight: 25,
		},
	}
}

// SetConfig OCR 설정 업데이트
func (om *OCRManager) SetConfig(cfg OCRConfig) {
	om.config = cfg
}

// GetConfig OCR 설정 반환
func (om *OCRManager) GetConfig() OCRConfig {
	return om.config
}

// CaptureNameRegion 게임 창에서 캐릭터 이름 영역만 크롭하여 반환
func (om *OCRManager) CaptureNameRegion(hwnd uint64) (image.Image, error) {
	img, _, err := om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		return nil, fmt.Errorf("창 캡처 실패: %v", err)
	}

	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	cfg := om.config
	cropX := cfg.NameRegionX
	cropY := cfg.NameRegionY
	cropW := cfg.NameRegionWidth
	cropH := cfg.NameRegionHeight

	// X=0이면 자동 계산 (오른쪽 위)
	if cropX == 0 {
		cropX = width - cropW - 10
	}

	// 범위 보정
	if cropX < 0 {
		cropX = 0
	}
	if cropX+cropW > width {
		cropW = width - cropX
	}
	if cropY+cropH > height {
		cropH = height - cropY
	}
	if cropW <= 0 || cropH <= 0 {
		return nil, fmt.Errorf("유효하지 않은 크롭 영역: %dx%d", cropW, cropH)
	}

	cropped := img.SubImage(image.Rect(cropX, cropY, cropX+cropW, cropY+cropH))
	return cropped, nil
}

// scaleImage 이미지를 지정 배율로 확대
func scaleImage(src image.Image, scale int) *image.RGBA {
	bounds := src.Bounds()
	scaledW := bounds.Dx() * scale
	scaledH := bounds.Dy() * scale
	scaled := image.NewRGBA(image.Rect(0, 0, scaledW, scaledH))
	draw.NearestNeighbor.Scale(scaled, scaled.Bounds(), src, bounds, draw.Over, nil)
	return scaled
}

// makeOCRVariants 하나의 크롭 이미지로부터 전처리 변형을 생성
// 성공 가능성이 높은 순서대로 배치 (빠른 early return)
func makeOCRVariants(src image.Image) []*image.RGBA {
	var variants []*image.RGBA

	// 4배 확대 원본 (가장 성공률 높음)
	scaled4 := scaleImage(src, 4)
	variants = append(variants, scaled4)

	// 3배 확대 원본
	scaled3 := scaleImage(src, 3)
	variants = append(variants, scaled3)

	bounds4 := scaled4.Bounds()
	w4, h4 := bounds4.Dx(), bounds4.Dy()

	// 표준 이진화 (어두운 글자 → 검정, 밝은 배경 → 흰색)
	for _, th := range []int{160, 120} {
		result := image.NewRGBA(image.Rect(0, 0, w4, h4))
		for y := 0; y < h4; y++ {
			for x := 0; x < w4; x++ {
				r, g, b, _ := scaled4.At(x, y).RGBA()
				lum := (299*int(r>>8) + 587*int(g>>8) + 114*int(b>>8)) / 1000
				if lum < th {
					result.Set(x, y, color.Black)
				} else {
					result.Set(x, y, color.White)
				}
			}
		}
		variants = append(variants, result)
	}

	// 반전 이진화 (밝은 글자 → 검정, 어두운 배경 → 흰색)
	for _, th := range []int{160, 120} {
		result := image.NewRGBA(image.Rect(0, 0, w4, h4))
		for y := 0; y < h4; y++ {
			for x := 0; x < w4; x++ {
				r, g, b, _ := scaled4.At(x, y).RGBA()
				lum := (299*int(r>>8) + 587*int(g>>8) + 114*int(b>>8)) / 1000
				if lum > th {
					result.Set(x, y, color.Black)
				} else {
					result.Set(x, y, color.White)
				}
			}
		}
		variants = append(variants, result)
	}

	return variants
}

// PreprocessForOCR 외부에서 전처리된 이미지를 확인할 수 있도록 공개
func PreprocessForOCR(src image.Image) *image.RGBA {
	return scaleImage(src, 4)
}

// recognizeImage 이미지를 PowerShell OCR로 인식 (내부 헬퍼)
func (om *OCRManager) recognizeImage(processed image.Image) (string, error) {
	// 임시 PNG 파일 생성
	tmpFile, err := os.CreateTemp("", "baram-ocr-*.png")
	if err != nil {
		return "", fmt.Errorf("임시 파일 생성 실패: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if err := png.Encode(tmpFile, processed); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("PNG 인코딩 실패: %v", err)
	}
	tmpFile.Close()

	// PowerShell 스크립트를 임시 .ps1 파일로 저장하여 실행
	psScript := "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8\n" +
		"Add-Type -AssemblyName System.Runtime.WindowsRuntime\n" +
		"\n" +
		"$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' })[0]\n" +
		"\n" +
		"Function Await($WinRtTask, $ResultType) {\n" +
		"    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)\n" +
		"    $netTask = $asTask.Invoke($null, @($WinRtTask))\n" +
		"    $netTask.Wait(-1) | Out-Null\n" +
		"    $netTask.Result\n" +
		"}\n" +
		"\n" +
		"$null = [Windows.Media.Ocr.OcrEngine, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]\n" +
		"$null = [Windows.Graphics.Imaging.BitmapDecoder, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]\n" +
		"$null = [Windows.Storage.Streams.RandomAccessStream, Windows.Storage.Streams, ContentType=WindowsRuntime]\n" +
		"$null = [Windows.Storage.StorageFile, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]\n" +
		"$null = [Windows.Globalization.Language, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]\n" +
		"\n" +
		"$lang = New-Object Windows.Globalization.Language('ko')\n" +
		"$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($lang)\n" +
		"if ($null -eq $engine) {\n" +
		"    Write-Error 'Korean OCR not available'\n" +
		"    exit 1\n" +
		"}\n" +
		"\n" +
		"$path = '" + tmpPath + "'\n" +
		"$storageFile = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($path)) ([Windows.Storage.StorageFile])\n" +
		"$stream = Await ($storageFile.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])\n" +
		"$bitmapDecoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])\n" +
		"$bitmap = Await ($bitmapDecoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])\n" +
		"$ocrResult = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])\n" +
		"$stream.Dispose()\n" +
		"\n" +
		"Write-Output $ocrResult.Text\n"

	// PowerShell 스크립트를 임시 파일로 저장
	psFile, err := os.CreateTemp("", "baram-ocr-*.ps1")
	if err != nil {
		return "", fmt.Errorf("PS1 임시 파일 생성 실패: %v", err)
	}
	psPath := psFile.Name()
	defer os.Remove(psPath)

	// UTF-8 BOM 작성 (PowerShell이 UTF-8로 읽도록)
	psFile.Write([]byte{0xEF, 0xBB, 0xBF})
	psFile.WriteString(psScript)
	psFile.Close()

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", psPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("PowerShell OCR 실패: %v (stderr: %s)", err, stderr.String())
	}

	text := strings.TrimSpace(stdout.String())
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")
	return text, nil
}

// RecognizeText 이미지에서 한국어 텍스트를 OCR로 인식
// 여러 전처리 변형을 시도하여 첫 번째 한글 결과를 반환
func (om *OCRManager) RecognizeText(img image.Image) (string, error) {
	variants := makeOCRVariants(img)

	for _, variant := range variants {
		text, err := om.recognizeImage(variant)
		if err != nil {
			continue
		}

		// 한글만 추출
		korean := extractKorean(text)
		if len([]rune(korean)) >= 2 {
			return korean, nil
		}
	}

	return "", nil
}

// extractKorean 문자열에서 한글 완성형만 추출
func extractKorean(s string) string {
	var filtered []rune
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			filtered = append(filtered, r)
		}
	}
	return string(filtered)
}

// CheckKoreanOCRAvailable 한국어 OCR 언어 팩 설치 여부 확인
func (om *OCRManager) CheckKoreanOCRAvailable() (bool, error) {
	psScript := `
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$null = [Windows.Media.Ocr.OcrEngine, Windows.Foundation.UniversalApiContract, ContentType=WindowsRuntime]
$langs = [Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages
foreach ($l in $langs) {
    if ($l.LanguageTag -eq "ko") {
        Write-Output "true"
        exit 0
    }
}
Write-Output "false"
`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("OCR 확인 실패: %v", err)
	}
	result := strings.TrimSpace(string(out))
	result = strings.TrimPrefix(result, "\xef\xbb\xbf")
	return result == "true", nil
}

// DetectCharacterName 게임 창에서 캐릭터 이름을 OCR로 감지
func (om *OCRManager) DetectCharacterName(hwnd uint64) (string, error) {
	img, err := om.CaptureNameRegion(hwnd)
	if err != nil {
		return "", fmt.Errorf("영역 캡처 실패: %v", err)
	}

	text, err := om.RecognizeText(img)
	if err != nil {
		return "", fmt.Errorf("OCR 인식 실패: %v", err)
	}

	return text, nil
}
