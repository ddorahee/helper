package automation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/image/draw"
)

// OCRWord OCR 인식된 단어와 위치 정보
type OCRWord struct {
	Text   string  `json:"Text"`
	X      float64 `json:"X"`
	Y      float64 `json:"Y"`
	Width  float64 `json:"W"`
	Height float64 `json:"H"`
}

// batchOCRResult 배치 OCR 결과 (PowerShell 1회 호출로 여러 이미지 처리)
// Words는 json.RawMessage로 받아서 배열/단일객체 모두 처리
// Text는 맵 OCR용 (전체 텍스트)
type batchOCRResult struct {
	Index int              `json:"Index"`
	Text  string           `json:"Text"`
	Words json.RawMessage  `json:"Words"`
	Error string           `json:"Error"`
}

// parseBatchWords Words JSON을 []OCRWord로 파싱 (배열 또는 단일 객체 처리)
func parseBatchWords(raw json.RawMessage) []OCRWord {
	if len(raw) == 0 {
		return nil
	}
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "[]" {
		return nil
	}
	// 배열인 경우
	if strings.HasPrefix(s, "[") {
		var words []OCRWord
		if err := json.Unmarshal(raw, &words); err != nil {
			return nil
		}
		return words
	}
	// 단일 객체인 경우
	var word OCRWord
	if err := json.Unmarshal(raw, &word); err != nil {
		return nil
	}
	return []OCRWord{word}
}

// OCRConfig OCR 크롭 영역 설정
type OCRConfig struct {
	NameRegionX      int
	NameRegionY      int
	NameRegionWidth  int
	NameRegionHeight int
}

// persistentPowerShell 상주 PowerShell 프로세스 (WinRT + OcrEngine 1회 로드)
type persistentPowerShell struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mutex  sync.Mutex
	ready  bool
}

// OCRManager Windows 내장 OCR 관리자
type OCRManager struct {
	wm     *WindowManager
	config OCRConfig
	ps     *persistentPowerShell
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

// persistentPSScript 상주 PowerShell 스크립트 (초기화 1회 + stdin 루프)
func persistentPSScript() string {
	return "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8\n" +
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
		"    Write-Output 'ERROR:Korean OCR not available'\n" +
		"    exit 1\n" +
		"}\n" +
		"Write-Output 'READY'\n" +
		"[Console]::Out.Flush()\n" +
		"\n" +
		"while ($true) {\n" +
		"    $line = [Console]::In.ReadLine()\n" +
		"    if ($null -eq $line -or $line -eq 'EXIT') { break }\n" +
		"    # BATCH|mapCount|path1|path2|...\n" +
		"    $parts = $line.Split('|')\n" +
		"    if ($parts[0] -ne 'BATCH') { continue }\n" +
		"    $mapCount = [int]$parts[1]\n" +
		"    $paths = $parts[2..($parts.Length-1)]\n" +
		"    $idx = 0\n" +
		"    foreach ($path in $paths) {\n" +
		"        try {\n" +
		"            $storageFile = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($path)) ([Windows.Storage.StorageFile])\n" +
		"            $stream = Await ($storageFile.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])\n" +
		"            $bitmapDecoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])\n" +
		"            $bitmap = Await ($bitmapDecoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])\n" +
		"            $ocrResult = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])\n" +
		"            $stream.Dispose()\n" +
		"            if ($idx -lt $mapCount) {\n" +
		"                $txt = $ocrResult.Text -replace '\"', '\\\"'\n" +
		"                Write-Output ('{\"Index\":' + $idx + ',\"Text\":\"' + $txt + '\",\"Words\":[],\"Error\":\"\"}')\n" +
		"            } else {\n" +
		"                $words = @()\n" +
		"                foreach ($ocrLine in $ocrResult.Lines) {\n" +
		"                    foreach ($word in $ocrLine.Words) {\n" +
		"                        $rect = $word.BoundingRect\n" +
		"                        $words += @{ Text = $word.Text; X = $rect.X; Y = $rect.Y; W = $rect.Width; H = $rect.Height }\n" +
		"                    }\n" +
		"                }\n" +
		"                $wordsJson = if ($words.Count -eq 0) { '[]' } else { $words | ConvertTo-Json -Compress }\n" +
		"                if ($words.Count -eq 1) { $wordsJson = '[' + $wordsJson + ']' }\n" +
		"                Write-Output ('{\"Index\":' + $idx + ',\"Text\":\"\",\"Words\":' + $wordsJson + ',\"Error\":\"\"}')\n" +
		"            }\n" +
		"        } catch {\n" +
		"            $inner = $_.Exception.InnerException\n" +
		"            $detail = $_.Exception.Message\n" +
		"            if ($null -ne $inner) { $detail = $detail + ' >> ' + $inner.Message }\n" +
		"            $detail = $detail + ' [path=' + $path + ']'\n" +
		"            $detail = $detail -replace '\"', '\\\"'\n" +
		"            Write-Output ('{\"Index\":' + $idx + ',\"Text\":\"\",\"Words\":[],\"Error\":\"' + $detail + '\"}')\n" +
		"        }\n" +
		"        $idx++\n" +
		"    }\n" +
		"    Write-Output '---END---'\n" +
		"    [Console]::Out.Flush()\n" +
		"}\n"
}

// StartPersistentOCR 상주 PowerShell 프로세스 시작 (앱 시작 시 1회 호출)
func (om *OCRManager) StartPersistentOCR() error {
	if om.ps != nil && om.ps.ready {
		return nil // 이미 실행 중
	}
	return om.startPersistentPS()
}

func (om *OCRManager) startPersistentPS() error {
	// PS 스크립트를 임시 파일로 저장
	psFile, err := os.CreateTemp("", "baram-ocr-persistent-*.ps1")
	if err != nil {
		return fmt.Errorf("PS1 임시 파일 생성 실패: %v", err)
	}
	psPath := psFile.Name()
	psFile.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	psFile.WriteString(persistentPSScript())
	psFile.Close()
	// 주의: 상주 프로세스이므로 PS1 파일은 프로세스 종료 시 삭제

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", psPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW

	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.Remove(psPath)
		return fmt.Errorf("stdin pipe 실패: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.Remove(psPath)
		return fmt.Errorf("stdout pipe 실패: %v", err)
	}

	if err := cmd.Start(); err != nil {
		os.Remove(psPath)
		return fmt.Errorf("PowerShell 시작 실패: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB 버퍼

	// READY 메시지 대기
	ready := false
	if scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "\xef\xbb\xbf")
		if line == "READY" {
			ready = true
		} else if strings.HasPrefix(line, "ERROR:") {
			cmd.Process.Kill()
			os.Remove(psPath)
			return fmt.Errorf("PowerShell OCR 초기화 실패: %s", line)
		}
	}

	if !ready {
		cmd.Process.Kill()
		os.Remove(psPath)
		return fmt.Errorf("PowerShell READY 메시지 수신 실패")
	}

	om.ps = &persistentPowerShell{
		cmd:    cmd,
		stdin:  stdin,
		stdout: scanner,
		ready:  true,
	}

	log.Println("[OCR] 상주 PowerShell 프로세스 시작 완료")

	// 백그라운드에서 프로세스 종료 감시 + PS1 파일 정리
	go func() {
		cmd.Wait()
		os.Remove(psPath)
		if om.ps != nil {
			om.ps.mutex.Lock()
			om.ps.ready = false
			om.ps.mutex.Unlock()
		}
		log.Println("[OCR] 상주 PowerShell 프로세스 종료됨")
	}()

	return nil
}

// StopPersistentOCR 상주 PowerShell 프로세스 종료 (앱 종료 시 호출)
func (om *OCRManager) StopPersistentOCR() {
	if om.ps == nil {
		return
	}
	om.ps.mutex.Lock()
	defer om.ps.mutex.Unlock()
	if om.ps.ready {
		om.ps.stdin.Write([]byte("EXIT\n"))
		om.ps.stdin.Close()
		om.ps.ready = false
	}
	log.Println("[OCR] 상주 PowerShell 프로세스 종료 요청")
}

// ensurePersistentPS 상주 PS가 살아있는지 확인, 죽었으면 재시작
func (om *OCRManager) ensurePersistentPS() error {
	if om.ps != nil && om.ps.ready {
		return nil
	}
	log.Println("[OCR] 상주 PowerShell 재시작 중...")
	return om.startPersistentPS()
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

	// X=0이면 자동 계산 (왼쪽 위, 클라이언트 영역 기준)
	if cropX == 0 {
		clientOffX, clientOffY, offErr := om.wm.GetClientOffset(hwnd)
		if offErr != nil {
			clientOffX, clientOffY = 8, 31
		}
		cropX = clientOffX + 10
		cropY += clientOffY
	}
	// X!=0이면 사용자가 UI에서 설정한 좌표 (전체 창 이미지 기준, 그대로 사용)

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

	log.Printf("[OCR] 크롭 영역: (%d, %d) ~ (%d, %d) [%dx%d]",
		cropX, cropY, cropX+cropW, cropY+cropH, cropW, cropH)

	cropped := img.SubImage(image.Rect(cropX, cropY, cropX+cropW, cropY+cropH))
	return cropped, nil
}

// DetectCharacterNameRightTop 오른쪽 상단 패널에서 캐릭터 이름 OCR
func (om *OCRManager) DetectCharacterNameRightTop(hwnd uint64) (string, error) {
	// 창 활성화 후 캡처
	om.wm.ActivateWindow(hwnd)
	time.Sleep(500 * time.Millisecond)

	img, _, err := om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		return "", fmt.Errorf("창 캡처 실패: %v", err)
	}

	width := img.Bounds().Dx()

	clientOffX, clientOffY, offErr := om.wm.GetClientOffset(hwnd)
	if offErr != nil {
		clientOffX, clientOffY = 8, 31
	}

	// 오른쪽 상단 패널 — 캐릭터 닉네임 영역
	// 1602x932 기준: 닉네임은 오른쪽 끝에서 ~155px, 상단에서 ~47px 위치, 약 120x22 크기
	// 비율 기반으로 계산
	clientW := width - clientOffX*2
	cropW := clientW * 8 / 100  // 폭 ~8%
	cropH := 22
	cropX := width - clientOffX - clientW*10/100  // 오른쪽에서 ~10%
	cropY := clientOffY + 15  // 타이틀바 아래 15px

	if cropX < 0 {
		cropX = 0
	}
	if cropX+cropW > width {
		cropW = width - cropX
	}
	if cropW <= 0 || cropH <= 0 {
		return "", fmt.Errorf("유효하지 않은 크롭 영역")
	}

	log.Printf("[OCR] 오른쪽 상단 크롭: (%d, %d) ~ (%d, %d) [%dx%d] 전체=%dx%d",
		cropX, cropY, cropX+cropW, cropY+cropH, cropW, cropH, width, img.Bounds().Dy())

	cropped := img.SubImage(image.Rect(cropX, cropY, cropX+cropW, cropY+cropH))

	// 디버그: 전체 이미지 + 크롭 이미지 저장
	debugDir := filepath.Join(os.Getenv("USERPROFILE"), "Downloads", "Temp")
	os.MkdirAll(debugDir, 0755)
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())

	fullPath := filepath.Join(debugDir, "trial-full-"+ts+".png")
	if f, err := os.Create(fullPath); err == nil {
		png.Encode(f, img)
		f.Close()
		log.Printf("[OCR] 전체 이미지: %s", fullPath)
	}
	cropPath := filepath.Join(debugDir, "trial-crop-"+ts+".png")
	if f, err := os.Create(cropPath); err == nil {
		png.Encode(f, cropped)
		f.Close()
		log.Printf("[OCR] 크롭 이미지: %s", cropPath)
	}

	text, err := om.RecognizeText(cropped)
	if err != nil {
		return "", err
	}
	return text, nil
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

// addPadding 이미지에 흰색 여백 추가 (OCR 인식률 향상)
func addPadding(src *image.RGBA, pad int) *image.RGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w+2*pad, h+2*pad))
	// 흰색으로 채우기
	for y := 0; y < h+2*pad; y++ {
		for x := 0; x < w+2*pad; x++ {
			dst.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	// 원본 이미지를 중앙에 복사
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x+pad, y+pad, src.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return dst
}

// makeOCRVariants 하나의 크롭 이미지로부터 전처리 변형을 생성
// 테스트 결과 최적 전략:
// 1) 4x wsat 이진화 + 여백: "바이" 등 추가 글자 인식에 강점 (최우선)
// 2) 4x w230 이진화 + 여백: 가장 깨끗한 결과 (타라폼 dist=1, 데브섹옵스 dist=2)
// 3) 4x 원본 스케일 + 여백: 이진화 없이 원본 색상 보존 (OCR 한글 컨텍스트 인식)
// 4) 6x wsat 이진화 + 여백: 넓은 텍스트용 폴백
func makeOCRVariants(src image.Image) []*image.RGBA {
	var variants []*image.RGBA

	// 1) 4x wsat (흰색+저채도 필터) — "바이" 같은 직선 위주 글자 인식에 가장 효과적
	scaled4 := scaleImage(src, 4)
	{
		b := scaled4.Bounds()
		w, h := b.Dx(), b.Dy()
		binarized := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rv, gv, bv, _ := scaled4.At(x, y).RGBA()
				r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
				minC := r8
				if g8 < minC {
					minC = g8
				}
				if b8 < minC {
					minC = b8
				}
				maxC := r8
				if g8 > maxC {
					maxC = g8
				}
				if b8 > maxC {
					maxC = b8
				}
				if maxC > 200 && (maxC-minC) < 40 {
					binarized.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
				} else {
					binarized.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
				}
			}
		}
		variants = append(variants, addPadding(binarized, 20))
	}

	// 2) 4x w230 (엄격한 흰색 필터) — 가장 깨끗한 결과
	{
		b := scaled4.Bounds()
		w, h := b.Dx(), b.Dy()
		binarized := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rv, gv, bv, _ := scaled4.At(x, y).RGBA()
				r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
				if r8 > 230 && g8 > 230 && b8 > 230 {
					binarized.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
				} else {
					binarized.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
				}
			}
		}
		variants = append(variants, addPadding(binarized, 20))
	}

	// 3) 4x 원본 (이진화 없음) — 원본 색상 보존, OCR이 한글 컨텍스트를 더 잘 인식
	variants = append(variants, addPadding(scaled4, 20))

	// 4) 6x wsat — 넓은 텍스트용 폴백
	scaled6 := scaleImage(src, 6)
	{
		b := scaled6.Bounds()
		w, h := b.Dx(), b.Dy()
		binarized := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				rv, gv, bv, _ := scaled6.At(x, y).RGBA()
				r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
				minC := r8
				if g8 < minC {
					minC = g8
				}
				if b8 < minC {
					minC = b8
				}
				maxC := r8
				if g8 > maxC {
					maxC = g8
				}
				if b8 > maxC {
					maxC = b8
				}
				if maxC > 200 && (maxC-minC) < 40 {
					binarized.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
				} else {
					binarized.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
				}
			}
		}
		variants = append(variants, addPadding(binarized, 20))
	}

	return variants
}

// recognizeImageWithLang 지정 언어로 이미지를 PowerShell OCR 인식
func (om *OCRManager) recognizeImageWithLang(processed image.Image, lang string) (string, error) {
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
		"$lang = New-Object Windows.Globalization.Language('" + lang + "')\n" +
		"$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($lang)\n" +
		"if ($null -eq $engine) {\n" +
		"    Write-Error '" + lang + " OCR not available'\n" +
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

	psFile, err := os.CreateTemp("", "baram-ocr-*.ps1")
	if err != nil {
		return "", fmt.Errorf("PS1 임시 파일 생성 실패: %v", err)
	}
	psPath := psFile.Name()
	defer os.Remove(psPath)

	psFile.Write([]byte{0xEF, 0xBB, 0xBF})
	psFile.WriteString(psScript)
	psFile.Close()

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", psPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW

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

// recognizeImage 이미지를 PowerShell OCR로 인식 (내부 헬퍼, 한국어)
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
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW

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
// 여러 전처리 변형(4x/6x × w230/wsat/w210 + 여백)을 시도하되,
// 3글자 이상 인식되면 즉시 반환 (속도 최적화)
// 반환 결과는 한글 완성형만 포함 (Levenshtein 매칭은 호출측에서 처리)
func (om *OCRManager) RecognizeText(img image.Image) (string, error) {
	variants := makeOCRVariants(img)

	bestKorean := ""
	for _, variant := range variants {
		text, err := om.recognizeImage(variant)
		if err != nil {
			continue
		}

		// 한글만 추출
		korean := extractKorean(text)
		if len([]rune(korean)) >= 2 && len([]rune(korean)) > len([]rune(bestKorean)) {
			bestKorean = korean
		}
		// 3글자 이상이면 충분히 좋은 결과 → early return
		if len([]rune(bestKorean)) >= 3 {
			return bestKorean, nil
		}
	}

	return bestKorean, nil
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

// recognizeWithPositionsOnce 단일 이미지에 대해 OCR+좌표를 수행하는 내부 헬퍼
func (om *OCRManager) recognizeWithPositionsOnce(processed image.Image) ([]OCRWord, error) {
	tmpFile, err := os.CreateTemp("", "baram-ocr-pos-*.png")
	if err != nil {
		return nil, fmt.Errorf("임시 파일 생성 실패: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if err := png.Encode(tmpFile, processed); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("PNG 인코딩 실패: %v", err)
	}
	tmpFile.Close()

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
		"$words = @()\n" +
		"foreach ($line in $ocrResult.Lines) {\n" +
		"    foreach ($word in $line.Words) {\n" +
		"        $rect = $word.BoundingRect\n" +
		"        $words += @{ Text = $word.Text; X = $rect.X; Y = $rect.Y; W = $rect.Width; H = $rect.Height }\n" +
		"    }\n" +
		"}\n" +
		"if ($words.Count -eq 0) {\n" +
		"    Write-Output '[]'\n" +
		"} else {\n" +
		"    $words | ConvertTo-Json -Compress\n" +
		"}\n"

	psFile, err := os.CreateTemp("", "baram-ocr-pos-*.ps1")
	if err != nil {
		return nil, fmt.Errorf("PS1 임시 파일 생성 실패: %v", err)
	}
	psPath := psFile.Name()
	defer os.Remove(psPath)

	psFile.Write([]byte{0xEF, 0xBB, 0xBF})
	psFile.WriteString(psScript)
	psFile.Close()

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", psPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("PowerShell OCR 실패: %v (stderr: %s)", err, stderr.String())
	}

	text := strings.TrimSpace(stdout.String())
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")

	if text == "" || text == "[]" {
		return []OCRWord{}, nil
	}

	var words []OCRWord
	if strings.HasPrefix(text, "[") {
		if err := json.Unmarshal([]byte(text), &words); err != nil {
			return nil, fmt.Errorf("OCR JSON 파싱 실패: %v (raw: %s)", err, text)
		}
	} else {
		var word OCRWord
		if err := json.Unmarshal([]byte(text), &word); err != nil {
			return nil, fmt.Errorf("OCR JSON 파싱 실패: %v (raw: %s)", err, text)
		}
		words = []OCRWord{word}
	}

	return words, nil
}

// recognizeWithPositionsBatch 여러 이미지를 PowerShell 1회 호출로 배치 OCR 처리
// WinRT 타입 로드 + OcrEngine 생성을 1회만 수행하여 7-8초 → 1-2초로 단축
func (om *OCRManager) recognizeWithPositionsBatch(images []image.Image) (map[int][]OCRWord, error) {
	if len(images) == 0 {
		return make(map[int][]OCRWord), nil
	}

	// 1. 모든 변형 이미지를 임시 PNG 파일로 저장
	tmpPaths := make([]string, len(images))
	for i, img := range images {
		tmpFile, err := os.CreateTemp("", fmt.Sprintf("baram-ocr-batch-%d-*.png", i))
		if err != nil {
			// 이미 생성된 임시 파일 정리
			for j := 0; j < i; j++ {
				os.Remove(tmpPaths[j])
			}
			return nil, fmt.Errorf("임시 파일 생성 실패: %v", err)
		}
		tmpPaths[i] = tmpFile.Name()
		if err := png.Encode(tmpFile, img); err != nil {
			tmpFile.Close()
			for j := 0; j <= i; j++ {
				os.Remove(tmpPaths[j])
			}
			return nil, fmt.Errorf("PNG 인코딩 실패: %v", err)
		}
		tmpFile.Close()
	}
	defer func() {
		for _, p := range tmpPaths {
			os.Remove(p)
		}
	}()

	// 2. 파일 경로 목록을 PowerShell 배열 문법으로 구성
	var pathEntries []string
	for _, p := range tmpPaths {
		pathEntries = append(pathEntries, "'"+strings.ReplaceAll(p, "'", "''")+"'")
	}
	pathsArrayStr := strings.Join(pathEntries, ", ")

	// 3. 배치 PowerShell 스크립트 생성 (WinRT 로드 + 엔진 생성 1회, 이미지 루프)
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
		"$paths = @(" + pathsArrayStr + ")\n" +
		"$idx = 0\n" +
		"foreach ($path in $paths) {\n" +
		"    try {\n" +
		"        $storageFile = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($path)) ([Windows.Storage.StorageFile])\n" +
		"        $stream = Await ($storageFile.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])\n" +
		"        $bitmapDecoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])\n" +
		"        $bitmap = Await ($bitmapDecoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])\n" +
		"        $ocrResult = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])\n" +
		"        $stream.Dispose()\n" +
		"        $words = @()\n" +
		"        foreach ($line in $ocrResult.Lines) {\n" +
		"            foreach ($word in $line.Words) {\n" +
		"                $rect = $word.BoundingRect\n" +
		"                $words += @{ Text = $word.Text; X = $rect.X; Y = $rect.Y; W = $rect.Width; H = $rect.Height }\n" +
		"            }\n" +
		"        }\n" +
		"        $wordsJson = if ($words.Count -eq 0) { '[]' } else { $words | ConvertTo-Json -Compress }\n" +
		"        if ($words.Count -eq 1) { $wordsJson = '[' + $wordsJson + ']' }\n" +
		"        Write-Output ('{\"Index\":' + $idx + ',\"Words\":' + $wordsJson + ',\"Error\":\"\"}')\n" +
		"    } catch {\n" +
		"        $errMsg = $_.Exception.Message -replace '\"', '\\\"'\n" +
		"        Write-Output ('{\"Index\":' + $idx + ',\"Words\":[],\"Error\":\"' + $errMsg + '\"}')\n" +
		"    }\n" +
		"    $idx++\n" +
		"}\n"

	// 4. PS1 파일 작성 + 실행
	psFile, err := os.CreateTemp("", "baram-ocr-batch-*.ps1")
	if err != nil {
		return nil, fmt.Errorf("PS1 임시 파일 생성 실패: %v", err)
	}
	psPath := psFile.Name()
	defer os.Remove(psPath)

	psFile.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	psFile.WriteString(psScript)
	psFile.Close()

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", psPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("배치 OCR 실패: %v (stderr: %s)", err, stderr.String())
	}

	// 5. JSONL 파싱 (줄 단위 JSON)
	text := strings.TrimSpace(stdout.String())
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")

	if text == "" {
		return make(map[int][]OCRWord), nil
	}

	resultMap := make(map[int][]OCRWord)

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r batchOCRResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue // 파싱 실패 줄은 스킵
		}
		if r.Error == "" {
			resultMap[r.Index] = parseBatchWords(r.Words)
		}
	}

	return resultMap, nil
}

// BatchAllResult 맵 OCR + 아이템 OCR 통합 결과
type BatchAllResult struct {
	MapText   string    // 맵 이름 OCR 텍스트 (한글 추출 전)
	ItemWords []OCRWord // 아이템 OCR 단어 목록 (좌표 역산 전 원본)
}

// RecognizeBatchAll 맵 이미지 + 아이템 이미지를 상주 PowerShell로 모두 처리
// mapImages: 맵 이름 크롭 이미지 (8x 스케일된 것, 텍스트만 반환)
// itemImages: 아이템 변형 이미지 (4x 스케일된 것, Words+좌표 반환)
func (om *OCRManager) RecognizeBatchAll(mapImages []image.Image, itemImages []image.Image) (*BatchAllResult, error) {
	totalCount := len(mapImages) + len(itemImages)
	if totalCount == 0 {
		return &BatchAllResult{}, nil
	}

	// 상주 PS 확인/시작
	if err := om.ensurePersistentPS(); err != nil {
		return nil, fmt.Errorf("상주 PS 시작 실패: %v", err)
	}

	// 모든 이미지를 임시 PNG로 저장 (비압축 PNG — 인코딩 빠르고 BitmapDecoder 호환)
	allImages := make([]image.Image, 0, totalCount)
	allImages = append(allImages, mapImages...)
	allImages = append(allImages, itemImages...)

	encoder := &png.Encoder{CompressionLevel: png.NoCompression}
	batchID := time.Now().UnixNano()
	tmpPaths := make([]string, len(allImages))
	for i, img := range allImages {
		tmpPath := fmt.Sprintf("C:\\Temp\\baram-ocr-%d-%d.png", batchID, i)
		f, err := os.Create(tmpPath)
		if err != nil {
			for j := 0; j < i; j++ {
				os.Remove(tmpPaths[j])
			}
			return nil, fmt.Errorf("임시 파일 생성 실패: %v", err)
		}
		if err := encoder.Encode(f, img); err != nil {
			f.Close()
			for j := 0; j < i; j++ {
				os.Remove(tmpPaths[j])
			}
			return nil, fmt.Errorf("PNG 인코딩 실패: %v", err)
		}
		f.Close()
		tmpPaths[i] = tmpPath
	}
	defer func() {
		for _, p := range tmpPaths {
			os.Remove(p)
		}
	}()

	mapCount := len(mapImages)

	// 상주 PS에 명령 전송: BATCH|mapCount|path1|path2|...
	cmdLine := fmt.Sprintf("BATCH|%d|%s\n", mapCount, strings.Join(tmpPaths, "|"))

	om.ps.mutex.Lock()
	defer om.ps.mutex.Unlock()

	if !om.ps.ready {
		return nil, fmt.Errorf("상주 PS 프로세스가 준비되지 않음")
	}

	_, err := om.ps.stdin.Write([]byte(cmdLine))
	if err != nil {
		om.ps.ready = false
		return nil, fmt.Errorf("PS stdin 쓰기 실패: %v", err)
	}

	// 결과 읽기: JSONL 줄들 + "---END---" 마커
	result := &BatchAllResult{}
	var allItemWords []OCRWord

	for om.ps.stdout.Scan() {
		line := strings.TrimSpace(om.ps.stdout.Text())
		line = strings.TrimPrefix(line, "\xef\xbb\xbf")
		if line == "---END---" {
			break
		}
		if line == "" {
			continue
		}
		var r batchOCRResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		if r.Error != "" {
			log.Printf("[OCR] 배치 인덱스 %d 에러: %s", r.Index, r.Error)
			continue
		}
		if r.Index < mapCount {
			if r.Text != "" {
				result.MapText = r.Text
			}
		} else {
			words := parseBatchWords(r.Words)
			allItemWords = append(allItemWords, words...)
		}
	}

	if err := om.ps.stdout.Err(); err != nil {
		om.ps.ready = false
		return nil, fmt.Errorf("PS stdout 읽기 에러: %v", err)
	}

	result.ItemWords = allItemWords
	log.Printf("[OCR] 배치결과: 맵='%s', 아이템단어=%d개", result.MapText, len(allItemWords))
	return result, nil
}

// RecognizeWithPositions 이미지에서 텍스트를 인식하고 각 단어의 위치(BoundingRect)도 반환
// 전략: 게임 중앙 영역을 크롭 → 4x 확대 + 다양한 색상 필터 변형으로 OCR 수행
// 게임 바닥 아이템 텍스트는 작아서 전체 화면 OCR로는 감지 불가 → 크롭+확대 필수
// 반환되는 좌표는 원본 이미지 기준으로 역산된다.
func (om *OCRManager) RecognizeWithPositions(img image.Image) ([]OCRWord, error) {
	bounds := img.Bounds()
	fullW := bounds.Dx()
	fullH := bounds.Dy()

	// 게임 경계선 안쪽만 크롭 (아이템이 드롭되는 범위)
	// 경계선은 게임 윈도우의 약 18%~69% 위치 (좌우 UI 패널 제외)
	cropX := fullW * 17 / 100  // 좌측 경계선
	cropY := fullH * 20 / 100  // 상단 20%
	cropW := fullW * 53 / 100  // 17%~70% = 53%
	cropH := fullH * 50 / 100  // 20%~70% = 50%
	if cropW <= 0 || cropH <= 0 {
		return []OCRWord{}, nil
	}

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	si, ok := img.(subImager)
	if !ok {
		return []OCRWord{}, nil
	}
	cropped := si.SubImage(image.Rect(cropX, cropY, cropX+cropW, cropY+cropH))

	// 4x 확대
	scaled4 := scaleImage(cropped, 4)
	s4Bounds := scaled4.Bounds()
	w4, h4 := s4Bounds.Dx(), s4Bounds.Dy()

	type variant struct {
		processed image.Image
		scale     int
	}

	variants := []variant{
		{scaled4, 4},
	}

	// === 초록색 통합 필터 (Green채널 + HSV 합체, 넓은 범위) ===
	// G가 R,B보다 15 이상 높거나 HSV 녹색 범위에 해당하는 픽셀
	{
		result := image.NewRGBA(image.Rect(0, 0, w4, h4))
		for y := 0; y < h4; y++ {
			for x := 0; x < w4; x++ {
				rv, gv, bv, _ := scaled4.At(x, y).RGBA()
				r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)

				isGreen := false
				// 방법1: G채널 우세 (완화된 임계값)
				if g8 > r8+15 && g8 > b8+15 && g8 > 40 {
					isGreen = true
				}
				// 방법2: HSV 기반 (채도+색상)
				if !isGreen {
					maxC := r8
					if g8 > maxC {
						maxC = g8
					}
					if b8 > maxC {
						maxC = b8
					}
					minC := r8
					if g8 < minC {
						minC = g8
					}
					if b8 < minC {
						minC = b8
					}
					delta := maxC - minC
					if delta > 10 && maxC > 50 {
						sat := delta * 100 / maxC
						if sat > 15 {
							var hue int
							if maxC == g8 {
								hue = 120 + 60*(b8-r8)/delta
							} else if maxC == r8 {
								hue = 60 * (g8 - b8) / delta
								if hue < 0 {
									hue += 360
								}
							} else {
								hue = 240 + 60*(r8-g8)/delta
							}
							if hue >= 60 && hue <= 180 {
								isGreen = true
							}
						}
					}
				}

				if isGreen {
					result.Set(x, y, color.Black)
				} else {
					result.Set(x, y, color.White)
				}
			}
		}
		variants = append(variants, variant{result, 4})
	}

	// === 밝은 색상 필터 (lum>140, 노란/흰 아이템용) ===
	{
		result := image.NewRGBA(image.Rect(0, 0, w4, h4))
		for y := 0; y < h4; y++ {
			for x := 0; x < w4; x++ {
				rv, gv, bv, _ := scaled4.At(x, y).RGBA()
				r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
				lum := (299*r8 + 587*g8 + 114*b8) / 1000
				if lum > 140 {
					result.Set(x, y, color.Black)
				} else {
					result.Set(x, y, color.White)
				}
			}
		}
		variants = append(variants, variant{result, 4})
	}

	// 배치 OCR: PowerShell 1회 호출로 모든 변형 처리
	variantImages := make([]image.Image, len(variants))
	for i, v := range variants {
		variantImages[i] = v.processed
	}

	batchResults, err := om.recognizeWithPositionsBatch(variantImages)
	if err != nil {
		return []OCRWord{}, nil
	}

	// 결과 합산 + 좌표 역산 + 중복 제거
	allWords := make(map[string]OCRWord)
	for i, v := range variants {
		words, ok := batchResults[i]
		if !ok {
			continue
		}
		scale := float64(v.scale)
		offsetX := float64(cropX)
		offsetY := float64(cropY)
		for j := range words {
			words[j].X = words[j].X/scale + offsetX
			words[j].Y = words[j].Y/scale + offsetY
			words[j].Width /= scale
			words[j].Height /= scale
		}
		for _, w := range words {
			if w.Text == "" {
				continue
			}
			key := fmt.Sprintf("%s_%.0f_%.0f", w.Text, w.X/10, w.Y/10)
			if _, exists := allWords[key]; !exists {
				allWords[key] = w
			}
		}
	}

	result := make([]OCRWord, 0, len(allWords))
	for _, w := range allWords {
		result = append(result, w)
	}

	return result, nil
}

// GameCoords 게임 내 X,Y 좌표
type GameCoords struct {
	X int
	Y int
}

// ReadCoordinates 게임 화면 우하단에서 X,Y 좌표를 OCR로 읽어옴
func (om *OCRManager) ReadCoordinates(hwnd uint64) (GameCoords, error) {
	img, _, err := om.wm.CaptureWindowRaw(hwnd)
	if err != nil {
		return GameCoords{}, fmt.Errorf("캡처 실패: %v", err)
	}
	coords, _, coordErr := om.ReadCoordinatesFromImage(img)
	return coords, coordErr
}

// ReadCoordinatesFromImage 이미지에서 우하단 좌표 영역을 픽셀 기반으로 인식
// 적응적 이진화로 UI 프레임/배경/텍스트를 자동 분리
func (om *OCRManager) ReadCoordinatesFromImage(img *image.RGBA) (coords GameCoords, debugTexts []string, err error) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// 우하단 200x40 크롭 (넓게 잡아서 적응적 분석)
	cropW := 200
	cropH := 40
	cropX := w - cropW - 5
	cropY := h - cropH - 5
	if cropX < 0 {
		cropX = 0
	}
	if cropY < 0 {
		cropY = 0
	}

	cropped := img.SubImage(image.Rect(cropX, cropY, cropX+cropW, cropY+cropH))

	// 적응적 이진화 시도
	result, dbg := readCoordsAdaptive(cropped, "adaptive")
	debugTexts = append(debugTexts, dbg...)
	if result.X > 0 || result.Y > 0 {
		return result, debugTexts, nil
	}

	// 폴백: 고정 필터 (어두운 배경용 — 테스트 이미지 등)
	type filterConfig struct {
		name   string
		filter func(r, g, b int) bool
	}
	fallbackFilters := []filterConfig{
		{"yellow_loose", func(r, g, b int) bool {
			return r > 120 && g > 120 && b < 210
		}},
		{"yellow_medium", func(r, g, b int) bool {
			return r > 140 && g > 140 && b < 180
		}},
	}
	for _, fc := range fallbackFilters {
		result, dbg := readCoordsWithFilterFunc(cropped, fc.name, fc.filter)
		debugTexts = append(debugTexts, dbg...)
		if result.X > 0 || result.Y > 0 {
			return result, debugTexts, nil
		}
	}

	return GameCoords{}, debugTexts, fmt.Errorf("좌표를 인식할 수 없음")
}

// readCoordsAdaptive 적응적 이진화로 좌표 인식
// 1) 크롭 영역에서 행별 밝기 변동성 분석 → 텍스트 행 자동 감지
// 2) 텍스트 행에서 Otsu 이진화 → 글리프 분리 → 인식
func readCoordsAdaptive(src image.Image, filterName string) (GameCoords, []string) {
	srcBounds := src.Bounds()
	sw := srcBounds.Dx()
	sh := srcBounds.Dy()

	// 밝기 맵 생성
	lumMap := make([][]int, sh)
	for y := 0; y < sh; y++ {
		lumMap[y] = make([]int, sw)
		for x := 0; x < sw; x++ {
			rv, gv, bv, _ := src.At(srcBounds.Min.X+x, srcBounds.Min.Y+y).RGBA()
			r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
			lumMap[y][x] = (299*r8 + 587*g8 + 114*b8) / 1000
		}
	}

	// B 채널 맵도 생성 (좌표 텍스트는 파란빛)
	bMap := make([][]int, sh)
	rMap := make([][]int, sh)
	gMap := make([][]int, sh)
	for y := 0; y < sh; y++ {
		bMap[y] = make([]int, sw)
		rMap[y] = make([]int, sw)
		gMap[y] = make([]int, sw)
		for x := 0; x < sw; x++ {
			rv, gv, bv, _ := src.At(srcBounds.Min.X+x, srcBounds.Min.Y+y).RGBA()
			rMap[y][x] = int(rv >> 8)
			gMap[y][x] = int(gv >> 8)
			bMap[y][x] = int(bv >> 8)
		}
	}

	// 전략 목록: 각각 이진화 조건이 다름
	type binStrategy struct {
		name string
		fn   func(x, y int) bool
	}
	strategies := []binStrategy{
		// 전략1: 파란빛 좌표 숫자 (B>150, lum>140)
		// 게임 좌표 숫자는 파란빛 회색(R≈150-190, G≈160-195, B≈195-225)
		{"coord_blue", func(x, y int) bool {
			return bMap[y][x] > 150 && lumMap[y][x] > 140
		}},
		// 전략2: 밝은 텍스트 (lum>160이고 프레임 제외: B>80)
		// X/Y 문자(노란)도 포함, 프레임(B<80)은 제외
		{"bright_noframe", func(x, y int) bool {
			return lumMap[y][x] > 160 && bMap[y][x] > 80
		}},
		// 전략3: 노란색 텍스트 (어두운 배경용 — 합성 테스트 이미지)
		{"yellow_text", func(x, y int) bool {
			r, g, b := rMap[y][x], gMap[y][x], bMap[y][x]
			return r > 120 && g > 120 && b < 210
		}},
	}

	var allDebug []string

	for _, strat := range strategies {
		// 이진화
		binary := make([][]bool, sh)
		for y := 0; y < sh; y++ {
			binary[y] = make([]bool, sw)
			for x := 0; x < sw; x++ {
				binary[y][x] = strat.fn(x, y)
			}
		}

		// 행 프로젝션: 텍스트 행 범위 찾기
		// 연속된 활성 행 중 가장 긴 구간을 선택
		type rowSpan struct{ top, bot int }
		var spans []rowSpan
		spanStart := -1
		for y := 0; y < sh; y++ {
			rowActive := 0
			for x := 0; x < sw; x++ {
				if binary[y][x] {
					rowActive++
				}
			}
			isTextRow := rowActive >= 3 && rowActive < sw*60/100
			if isTextRow {
				if spanStart < 0 {
					spanStart = y
				}
			} else {
				if spanStart >= 0 {
					spans = append(spans, rowSpan{spanStart, y - 1})
					spanStart = -1
				}
			}
		}
		if spanStart >= 0 {
			spans = append(spans, rowSpan{spanStart, sh - 1})
		}

		// 가장 긴 연속 구간 선택 (최대 15행)
		textTop, textBot := -1, -1
		bestLen := 0
		for _, s := range spans {
			sLen := s.bot - s.top + 1
			if sLen > bestLen && sLen >= 3 && sLen <= 15 {
				bestLen = sLen
				textTop = s.top
				textBot = s.bot
			}
		}

		if textTop < 0 || textBot < 0 {
			allDebug = append(allDebug, fmt.Sprintf("[%s] 텍스트 행 감지 실패", strat.name))
			continue
		}

		// 텍스트 행 범위만 추출
		textH := textBot - textTop + 1
		textBinary := make([][]bool, textH)
		for y := 0; y < textH; y++ {
			textBinary[y] = binary[textTop+y]
		}

		result, dbg := recognizeFromBinary(textBinary, sw, textH, strat.name, textTop, textBot)
		allDebug = append(allDebug, dbg...)

		if result.X > 0 || result.Y > 0 {
			return result, allDebug
		}
	}

	return GameCoords{}, allDebug
}

// recognizeFromBinary 이진화된 텍스트 영역에서 글리프 분리 + 인식 + 좌표 추출
func recognizeFromBinary(textBinary [][]bool, sw, textH int, stratName string, textTop, textBot int) (GameCoords, []string) {
	// 열 프로젝션으로 글리프 분리
	colSum := make([]int, sw)
	for x := 0; x < sw; x++ {
		for y := 0; y < textH; y++ {
			if textBinary[y][x] {
				colSum[x]++
			}
		}
	}

	type glyphInfo struct {
		startX, endX int
	}
	var allGlyphs []glyphInfo
	inGlyph := false
	startX := 0
	for x := 0; x < sw; x++ {
		if colSum[x] > 0 {
			if !inGlyph {
				startX = x
				inGlyph = true
			}
		} else {
			if inGlyph {
				allGlyphs = append(allGlyphs, glyphInfo{startX, x})
				inGlyph = false
			}
		}
	}
	if inGlyph {
		allGlyphs = append(allGlyphs, glyphInfo{startX, sw})
	}

	// 노이즈 글리프 제거: 실제 높이가 4px 미만인 글리프는 UI 요소/산 그래픽
	var glyphs []glyphInfo
	for _, gl := range allGlyphs {
		minY, maxY := textH, 0
		for y := 0; y < textH; y++ {
			for x := gl.startX; x < gl.endX; x++ {
				if textBinary[y][x] {
					if y < minY {
						minY = y
					}
					if y > maxY {
						maxY = y
					}
				}
			}
		}
		glyphH := 0
		if maxY >= minY {
			glyphH = maxY - minY + 1
		}
		glyphW := gl.endX - gl.startX
		if glyphH >= 4 && glyphW <= 10 {
			glyphs = append(glyphs, gl)
		}
	}

	// 첫 글리프가 나머지와 극단적으로 떨어져 있으면 (gap>50px) 노이즈로 제거
	// UI 프레임 픽셀이 blue 필터를 통과하여 가짜 글리프를 생성하는 경우 방지
	if len(glyphs) >= 2 {
		firstGap := glyphs[1].startX - glyphs[0].endX
		if firstGap > 50 {
			glyphs = glyphs[1:]
		}
	}

	debugTexts := []string{fmt.Sprintf("[%s] textY=%d~%d, 글리프=%d개(전체%d)",
		stratName, textTop, textBot, len(glyphs), len(allGlyphs))}

	// 글리프 위치/gap 디버그
	for i, gl := range glyphs {
		if i == 0 {
			debugTexts = append(debugTexts, fmt.Sprintf("[%s] glyph[%d] x=%d~%d (w=%d)", stratName, i, gl.startX, gl.endX, gl.endX-gl.startX))
		} else {
			gap := gl.startX - glyphs[i-1].endX
			debugTexts = append(debugTexts, fmt.Sprintf("[%s] glyph[%d] x=%d~%d (w=%d) gap=%d", stratName, i, gl.startX, gl.endX, gl.endX-gl.startX, gap))
		}
	}

	if len(glyphs) < 4 {
		return GameCoords{}, debugTexts
	}

	// 글리프 간 gap 분석으로 그룹 분리
	// 게임 좌표는 "X nnn  Y nnn" 형식이고, 파란색 필터에서는 X/Y문자가 안 잡힘
	// 따라서 숫자 그룹 2개: [nnn] [gap>15px] [nnn]
	type glyphGroup struct {
		glyphs []glyphInfo
	}
	var groups []glyphGroup
	currentGroup := []glyphInfo{glyphs[0]}
	for i := 1; i < len(glyphs); i++ {
		gap := glyphs[i].startX - glyphs[i-1].endX
		// 큰 gap(>15px)이면 X좌표/Y좌표 경계
		if gap > 15 {
			groups = append(groups, glyphGroup{currentGroup})
			currentGroup = []glyphInfo{glyphs[i]}
		} else {
			currentGroup = append(currentGroup, glyphs[i])
		}
	}
	groups = append(groups, glyphGroup{currentGroup})

	// 각 글리프를 문자로 인식
	var recognized []string
	for _, g := range groups {
		word := ""
		for _, gl := range g.glyphs {
			ch := recognizeGlyph(textBinary, gl.startX, gl.endX, 0, textH-1)
			word += ch
		}
		recognized = append(recognized, word)
	}

	debugTexts = append(debugTexts, fmt.Sprintf("[%s] 그룹=%d, 인식=%v", stratName, len(groups), recognized))

	// 좌표 추출: X/Y 문자가 있으면 그 뒤의 숫자, 없으면 순서대로 X값/Y값
	xVal, yVal := -1, -1
	afterY := false
	numGroupIdx := 0
	for _, word := range recognized {
		if word == "X" || word == "x" {
			continue
		}
		if word == "Y" || word == "y" {
			afterY = true
			continue
		}
		num := 0
		hasDigit := false
		for _, ch := range word {
			if ch >= '0' && ch <= '9' {
				num = num*10 + int(ch-'0')
				hasDigit = true
			}
		}
		if hasDigit {
			if afterY && yVal < 0 {
				yVal = num
			} else if !afterY && xVal < 0 {
				xVal = num
				numGroupIdx++
			} else if numGroupIdx >= 1 && yVal < 0 {
				// X/Y 문자 없이 숫자 그룹만 2개인 경우 (파란색 필터)
				yVal = num
			}
		}
	}

	if xVal >= 0 && yVal >= 0 {
		return GameCoords{X: xVal, Y: yVal}, debugTexts
	}

	// 폴백: 모든 글리프를 순서대로 인식하여 X/Y 패턴 찾기
	allChars := ""
	for _, gl := range glyphs {
		allChars += recognizeGlyph(textBinary, gl.startX, gl.endX, 0, textH-1)
	}
	debugTexts = append(debugTexts, fmt.Sprintf("[%s] all=%s", stratName, allChars))

	xIdx := strings.IndexAny(allChars, "Xx")
	yIdx := strings.IndexAny(allChars, "Yy")
	if xIdx >= 0 && yIdx > xIdx {
		xPart := extractAllDigits(allChars[xIdx+1 : yIdx])
		yPart := extractAllDigits(allChars[yIdx+1:])
		if len(xPart) >= 1 && len(yPart) >= 1 {
			xv, _ := strconv.Atoi(xPart)
			yv, _ := strconv.Atoi(yPart)
			if xv >= 0 && yv >= 0 {
				return GameCoords{X: xv, Y: yv}, debugTexts
			}
		}
	}

	return GameCoords{}, debugTexts
}

// readCoordsWithFilter 지정 필터로 이진화 후 글리프 기반 좌표 인식 (레거시 호환)
func readCoordsWithFilter(src image.Image, filterName string, minR, minG, maxB int, useLum bool, lumTh int) (GameCoords, []string) {
	var filterFunc func(r, g, b int) bool
	if useLum {
		filterFunc = func(r, g, b int) bool {
			lum := (299*r + 587*g + 114*b) / 1000
			return lum > lumTh
		}
	} else {
		filterFunc = func(r, g, b int) bool {
			return r > minR && g > minG && b < maxB
		}
	}
	return readCoordsWithFilterFunc(src, filterName, filterFunc)
}

// readCoordsWithFilterFunc 함수 기반 필터로 이진화 후 글리프 기반 좌표 인식
func readCoordsWithFilterFunc(src image.Image, filterName string, filterFunc func(r, g, b int) bool) (GameCoords, []string) {
	srcBounds := src.Bounds()
	sw := srcBounds.Dx()
	sh := srcBounds.Dy()

	// 이진화 (확대 없이 원본에서 직접 — 픽셀 폰트이므로 확대 불필요)
	binary := make([][]bool, sh)
	for y := 0; y < sh; y++ {
		binary[y] = make([]bool, sw)
		for x := 0; x < sw; x++ {
			rv, gv, bv, _ := src.At(srcBounds.Min.X+x, srcBounds.Min.Y+y).RGBA()
			r8, g8, b8 := int(rv>>8), int(gv>>8), int(bv>>8)
			binary[y][x] = filterFunc(r8, g8, b8)
		}
	}

	// 행 프로젝션: 텍스트가 있는 행 범위 찾기
	rowSum := make([]int, sh)
	maxRow := 0
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			if binary[y][x] {
				rowSum[y]++
			}
		}
		if rowSum[y] > maxRow {
			maxRow = y
		}
	}

	// 텍스트 행 범위 (연속된 활성 행)
	textTop, textBot := -1, -1
	for y := 0; y < sh; y++ {
		if rowSum[y] >= 3 { // 최소 3픽셀 이상 활성인 행
			if textTop < 0 {
				textTop = y
			}
			textBot = y
		}
	}
	if textTop < 0 || textBot < 0 || textBot-textTop < 3 {
		return GameCoords{}, []string{fmt.Sprintf("[%s] 텍스트 행 없음", filterName)}
	}

	// 텍스트 행 범위 내에서 열 프로젝션
	colSum := make([]int, sw)
	for x := 0; x < sw; x++ {
		for y := textTop; y <= textBot; y++ {
			if binary[y][x] {
				colSum[x]++
			}
		}
	}

	// 글리프 분리
	type glyphInfo struct {
		startX, endX int
	}
	var glyphs []glyphInfo
	inGlyph := false
	startX := 0
	for x := 0; x < sw; x++ {
		if colSum[x] > 0 {
			if !inGlyph {
				startX = x
				inGlyph = true
			}
		} else {
			if inGlyph {
				glyphs = append(glyphs, glyphInfo{startX, x})
				inGlyph = false
			}
		}
	}
	if inGlyph {
		glyphs = append(glyphs, glyphInfo{startX, sw})
	}

	if len(glyphs) < 4 {
		return GameCoords{}, []string{fmt.Sprintf("[%s] 글리프 %d개 (최소 4개 필요)", filterName, len(glyphs))}
	}

	// 글리프 간 gap 분석으로 그룹 분리
	// X nnn Y nnn — X와 Y 사이, Y와 숫자 사이에 큰 gap이 있음
	// 그룹: [X] [n] [n] [n] ... [Y] [n] [n] [n]
	type glyphGroup struct {
		glyphs []glyphInfo
	}
	var groups []glyphGroup
	currentGroup := []glyphInfo{glyphs[0]}
	for i := 1; i < len(glyphs); i++ {
		gap := glyphs[i].startX - glyphs[i-1].endX
		glyphAvgW := 0
		for _, g := range glyphs {
			glyphAvgW += g.endX - g.startX
		}
		glyphAvgW /= len(glyphs)

		// gap이 평균 글리프 너비의 60% 이상이면 단어 구분
		if gap > glyphAvgW*6/10 {
			groups = append(groups, glyphGroup{currentGroup})
			currentGroup = []glyphInfo{glyphs[i]}
		} else {
			currentGroup = append(currentGroup, glyphs[i])
		}
	}
	groups = append(groups, glyphGroup{currentGroup})

	// 각 글리프를 숫자로 인식
	var recognized []string
	for _, g := range groups {
		word := ""
		for _, gl := range g.glyphs {
			ch := recognizeGlyph(binary, gl.startX, gl.endX, textTop, textBot)
			word += ch
		}
		recognized = append(recognized, word)
	}

	debugTexts := []string{fmt.Sprintf("[%s] 글리프=%d, 그룹=%d, 인식=%v", filterName, len(glyphs), len(groups), recognized)}

	// "X nnn Y nnn" 패턴에서 좌표 추출
	xVal, yVal := -1, -1
	afterY := false
	for _, word := range recognized {
		if word == "X" || word == "x" {
			continue
		}
		if word == "Y" || word == "y" {
			afterY = true
			continue
		}
		// 숫자 추출
		num := 0
		hasDigit := false
		for _, ch := range word {
			if ch >= '0' && ch <= '9' {
				num = num*10 + int(ch-'0')
				hasDigit = true
			}
		}
		if hasDigit {
			if !afterY && xVal < 0 {
				xVal = num
			} else if afterY && yVal < 0 {
				yVal = num
			}
		}
	}

	if xVal >= 0 && yVal >= 0 {
		return GameCoords{X: xVal, Y: yVal}, debugTexts
	}

	// 그룹 분리 실패 시, 모든 글리프를 순서대로 인식하여 X/Y 패턴 찾기
	allChars := ""
	for _, gl := range glyphs {
		allChars += recognizeGlyph(binary, gl.startX, gl.endX, textTop, textBot)
	}
	debugTexts = append(debugTexts, fmt.Sprintf("[%s] all=%s", filterName, allChars))

	// "X" 와 "Y" 위치로 분리
	xIdx := strings.IndexAny(allChars, "Xx")
	yIdx := strings.IndexAny(allChars, "Yy")
	if xIdx >= 0 && yIdx > xIdx {
		xPart := extractAllDigits(allChars[xIdx+1 : yIdx])
		yPart := extractAllDigits(allChars[yIdx+1:])
		if len(xPart) >= 1 && len(yPart) >= 1 {
			xv, _ := strconv.Atoi(xPart)
			yv, _ := strconv.Atoi(yPart)
			if xv >= 0 && yv >= 0 {
				return GameCoords{X: xv, Y: yv}, debugTexts
			}
		}
	}

	return GameCoords{}, debugTexts
}

// rawDigitTemplate 원본 해상도 비트맵 템플릿
type rawDigitTemplate struct {
	char   string
	width  int
	height int
	bitmap []string // 각 행: "..##.." 형식 ('#'=채움, '.'=빈칸) ASCII만 사용
}

// rawDigitTemplates 게임 좌표 폰트의 원본 픽셀 패턴 (pixelanalyze로 추출)
var rawDigitTemplates = []rawDigitTemplate{
	{"0", 6, 7, []string{
		"..##..",
		".####.",
		"##..##",
		"##..##",
		"##..##",
		"##..##",
		"##..##",
	}},
	{"1", 4, 7, []string{
		"..##",
		".###",
		"####",
		"..##",
		"..##",
		"..##",
		"..##",
	}},
	{"2", 6, 7, []string{
		"..###.",
		".#####",
		"##..##",
		"....##",
		"....##",
		"...##.",
		"..##..",
	}},
	{"3", 6, 7, []string{
		".#####",
		"######",
		"...##.",
		"..##..",
		".###..",
		"..###.",
		"...##.",
	}},
	{"4", 6, 7, []string{
		"....##",
		"...###",
		"..####",
		".##.##",
		"##..##",
		"######",
		"######",
	}},
	{"5", 6, 7, []string{
		".#####",
		".####.",
		"##....",
		"####..",
		"#####.",
		"...##.",
		"..###.",
	}},
	{"6", 6, 7, []string{
		"...###",
		"..###.",
		".##...",
		".####.",
		"##..##",
		"##..##",
		"##.###",
	}},
	{"7", 6, 7, []string{
		".#####",
		"######",
		"....##",
		"...##.",
		"..##..",
		".##...",
		"###...",
	}},
	{"8", 7, 7, []string{
		"..####.",
		".##..##",
		".##.###",
		"..####.",
		".####..",
		"##..##.",
		"##..##.",
	}},
	{"9", 6, 7, []string{
		"..###.",
		".##.##",
		"##..##",
		"##..##",
		"######",
		".####.",
		"...##.",
	}},
}

// bitmapToPattern 비트맵 문자열을 bool 배열로 변환
func bitmapToPattern(bitmap []string) [][]bool {
	h := len(bitmap)
	if h == 0 {
		return nil
	}
	w := len(bitmap[0])
	pat := make([][]bool, h)
	for y := 0; y < h; y++ {
		pat[y] = make([]bool, w)
		for x := 0; x < len(bitmap[y]); x++ {
			pat[y][x] = bitmap[y][x] == '#'
		}
	}
	return pat
}

// matchRawTemplate 원본 해상도 비트맵과 글리프를 직접 비교
// 크기가 다르면 매우 높은 거리 반환
func matchRawTemplate(binary [][]bool, actualLeft, actualTop, aw, ah int, tmpl rawDigitTemplate) int {
	if aw != tmpl.width || ah != tmpl.height {
		return 9999 // 크기 불일치
	}
	pat := bitmapToPattern(tmpl.bitmap)
	dist := 0
	for y := 0; y < ah; y++ {
		for x := 0; x < aw; x++ {
			srcY := actualTop + y
			srcX := actualLeft + x
			src := false
			if srcY >= 0 && srcY < len(binary) && srcX >= 0 && srcX < len(binary[0]) {
				src = binary[srcY][srcX]
			}
			if src != pat[y][x] {
				dist++
			}
		}
	}
	return dist
}

// recognizeGlyph 원본 해상도 비트맵 매칭으로 글리프 인식
func recognizeGlyph(binary [][]bool, startX, endX, top, bot int) string {
	gw := endX - startX
	gh := bot - top + 1
	if gw <= 0 || gh <= 0 {
		return "?"
	}

	// 글리프 실제 경계 (trim)
	actualTop, actualBot := bot, top
	actualLeft, actualRight := endX, startX
	for y := top; y <= bot; y++ {
		for x := startX; x < endX; x++ {
			if binary[y][x] {
				if y < actualTop {
					actualTop = y
				}
				if y > actualBot {
					actualBot = y
				}
				if x < actualLeft {
					actualLeft = x
				}
				if x > actualRight {
					actualRight = x
				}
			}
		}
	}
	if actualTop > actualBot || actualLeft > actualRight {
		return "?"
	}

	aw := actualRight - actualLeft + 1
	ah := actualBot - actualTop + 1

	// 원본 해상도 매칭 시도
	bestChar := "?"
	bestDist := 9999

	for _, tmpl := range rawDigitTemplates {
		dist := matchRawTemplate(binary, actualLeft, actualTop, aw, ah, tmpl)
		if dist < bestDist {
			bestDist = dist
			bestChar = tmpl.char
		}
	}

	// 원본 매칭 성공 (허용 오차: 전체 픽셀의 20% 이하)
	maxDist := aw * ah * 20 / 100
	if maxDist < 3 {
		maxDist = 3
	}

	if bestDist <= maxDist {
		return bestChar
	}

	// 폴백: 1은 폭이 좁은 것으로 판단
	if float64(aw) < float64(ah)*0.45 {
		return "1"
	}

	return bestChar // 최선의 결과 반환
}

// extractAllDigits 문자열에서 모든 숫자를 이어붙여 반환
func extractAllDigits(s string) string {
	var digits []byte
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			digits = append(digits, s[i])
		}
	}
	return string(digits)
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
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
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
