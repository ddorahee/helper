package automation

import (
	"fmt"
	"image"
	"log"
)

// OCRManager 에 붙는 글리프 읽기 진입점.
// 전부 "창 전체 이미지 + 클라이언트 오프셋" 을 받는다 — 맵 이름 바를 스스로 찾아야 하기 때문.

// clientBox 창 이미지에서 클라이언트(게임 렌더) 영역의 오프셋과 크기
func (om *OCRManager) clientBox(img image.Image, hwnd uint64) (offX, offY, cw, ch int) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	var err error
	offX, offY, err = om.wm.GetClientOffset(hwnd)
	if err != nil {
		offX, offY = 8, 31
	}
	cw, ch = W-offX*2, H-offY-offX
	if cw <= 0 || ch <= 0 {
		offX, offY, cw, ch = 0, 0, W, H
	}
	return offX + b.Min.X, offY + b.Min.Y, cw, ch
}

// ReadMapGlyph 창 전체 이미지에서 맵 이름을 읽는다. 사전에 없는 글자가 하나라도 있으면
// 틀린 값을 지어내는 대신 ok=false 를 돌려준다.
func (om *OCRManager) ReadMapGlyph(img image.Image, hwnd uint64) (string, bool) {
	if img == nil {
		return "", false
	}
	offX, offY, cw, ch := om.clientBox(img, hwnd)
	reg := GlyphMapRegion(img, offX, offY, cw, ch)
	return RecognizeGlyphs(BinarizeGlyph(img, reg), LoadGlyphDict())
}

// ReadNickGlyph 창 전체 이미지에서 우상단 닉네임을 읽는다.
func (om *OCRManager) ReadNickGlyph(img image.Image, hwnd uint64) (string, bool) {
	if img == nil {
		return "", false
	}
	offX, offY, cw, ch := om.clientBox(img, hwnd)
	reg := GlyphNickRegion(offX, offY, cw, ch)
	return RecognizeGlyphs(BinarizeGlyph(img, reg), LoadGlyphDict())
}

// GlyphSnapshot 학습/진단 화면에 보여줄 한 창의 현재 인식 상태
type GlyphSnapshot struct {
	MapName   string
	MapOK     bool
	MapImage  image.Image
	NickName  string
	NickOK    bool
	NickImage image.Image
}

// glyphCropImage 영역을 잘라 이미지로 (UI 표시용)
func glyphCropImage(img image.Image, r GlyphRegion) image.Image {
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return nil
	}
	return sub.SubImage(image.Rect(r.X0, r.Y0, r.X1, r.Y1))
}

// glyphCapture 창 한 장 캡처. bg=false 면 창을 앞으로 가져온다(사용자가 눈으로 확인하도록).
// 실제 픽셀은 Z-order 와 무관한 PrintWindow 를 우선 쓴다 — captureRead 주석 참고.
func (om *OCRManager) glyphCapture(hwnd uint64, bg bool) (image.Image, error) {
	return om.captureRead(hwnd, !bg)
}

// GlyphInspect 창을 한 번 캡처해 맵/닉네임 인식 결과와 크롭 이미지를 함께 돌려준다.
func (om *OCRManager) GlyphInspect(hwnd uint64, bg bool) (*GlyphSnapshot, error) {
	img, err := om.glyphCapture(hwnd, bg)
	if err != nil {
		return nil, err
	}
	offX, offY, cw, ch := om.clientBox(img, hwnd)
	mr := GlyphMapRegion(img, offX, offY, cw, ch)
	nr := GlyphNickRegion(offX, offY, cw, ch)
	d := LoadGlyphDict()
	s := &GlyphSnapshot{
		MapImage:  glyphCropImage(img, mr),
		NickImage: glyphCropImage(img, nr),
	}
	s.MapName, s.MapOK = RecognizeGlyphs(BinarizeGlyph(img, mr), d)
	s.NickName, s.NickOK = RecognizeGlyphs(BinarizeGlyph(img, nr), d)
	return s, nil
}

// GlyphLearn 창을 캡처해, 사용자가 알려준 맵 이름/닉네임으로 새 글자를 배운다.
// mapText 나 nickText 가 빈 문자열이면 그쪽은 건너뛴다.
func (om *OCRManager) GlyphLearn(hwnd uint64, bg bool, mapText, nickText string) (int, []string, error) {
	img, err := om.glyphCapture(hwnd, bg)
	if err != nil {
		return 0, nil, err
	}
	offX, offY, cw, ch := om.clientBox(img, hwnd)
	known := LoadGlyphDict()

	add := GlyphDict{}
	var notes []string
	learn := func(label, text string, reg GlyphRegion) {
		if text == "" {
			return
		}
		got, err := FitGlyphs(BinarizeGlyph(img, reg), text, known)
		if err != nil {
			notes = append(notes, fmt.Sprintf("%s 실패: %v", label, err))
			return
		}
		n := 0
		for k, v := range got {
			if _, dup := add[k]; !dup {
				add[k] = v
				n++
			}
		}
		if n == 0 {
			notes = append(notes, fmt.Sprintf("%s: 이미 다 아는 글자입니다", label))
		} else {
			notes = append(notes, fmt.Sprintf("%s: 새 글자 %d개", label, n))
		}
	}
	learn("맵 이름", mapText, GlyphMapRegion(img, offX, offY, cw, ch))
	learn("닉네임", nickText, GlyphNickRegion(offX, offY, cw, ch))

	if len(add) == 0 {
		return 0, notes, nil
	}
	added, err := MergeGlyphDict(add)
	if err != nil {
		return 0, notes, fmt.Errorf("사전 저장 실패: %v", err)
	}
	log.Printf("[글리프] 학습 완료: %d개 추가 (총 %d개)", added, GlyphDictSize())
	return added, notes, nil
}
