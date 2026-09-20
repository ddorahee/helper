package automation

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// 글리프 인식 검증 ─ 바탕화면의 dataset-* 폴더(실제 게임 캡처 714장)로 돌린다.
// 폴더가 없으면 건너뛴다.
//
// 기준:
//   닉네임 714/714 = 100%
//   맵      707/714 — 나머지 7장은 맵 전환 중 암전이거나 마우스 커서가 글자를 가려
//           글자 자체가 화면에 없다. 그런 경우 틀린 값 대신 실패를 돌려주는 게 정상 동작.

const glyphTestRoot = `C:\Users\Home\Desktop`

// glyphTestDirs 폴더 → 그 캐릭터의 닉네임
var glyphTestDirs = map[string]string{
	"dataset-사막": "재모바이",
	"dataset-왕무": "햐루루",
	"dataset-왕궁": "햐루루",
	"dataset-나타": "햐루루",
}

// glyphTestMaps 이 dataset 들에 실제로 등장하는 맵 이름 (눈으로 확인해 붙인 라벨)
var glyphTestMaps = map[string]bool{
	"태양의사막-계곡입구": true, "태양의사막1": true, "태양의사막2": true, "나가의오아시스": true,
	"왕궁외곽길": true, "왕의무덤내부": true,
	"신라왕궁동편": true, "신라왕궁비밀의방1": true, "신라왕궁비밀의방2": true, "신라왕궁비밀의방3": true,
	"신라왕궁비밀의방4": true, "신라왕궁비밀의방5": true, "신라왕궁비밀의방6": true, "신라왕궁비밀의방7": true,
	"신라왕궁비밀의방8": true, "신라왕궁비밀의방깊은곳": true,
	"나타라자사원입구": true, "나타라자사원": true, "돌아온나타라자사원": true,
	"아공간-번개의시험": true, "아공간-불의시험": true, "아공간-물의시험": true, "아공간-나타라자사원": true,
}

func glyphTestLoad(p string) image.Image {
	fh, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer fh.Close()
	im, err := png.Decode(fh)
	if err != nil {
		return nil
	}
	return im
}

// glyphTestClient dataset 캡처는 창 전체(타이틀바 포함)다. 게임 클라이언트는 1600x900 고정이라
// 좌우 테두리 두께로 오프셋을 역산한다. 실제 동작에서는 GetClientOffset 이 준다.
func glyphTestClient(im image.Image) (offX, offY, cw, ch int) {
	b := im.Bounds()
	offX = (b.Dx() - 1600) / 2
	return offX, b.Dy() - 900 - offX, 1600, 900
}

func glyphTestFiles(t *testing.T) []string {
	var out []string
	for dir := range glyphTestDirs {
		fs, _ := filepath.Glob(filepath.Join(glyphTestRoot, dir, "*.png"))
		out = append(out, fs...)
	}
	sort.Strings(out)
	return out
}

func TestGlyphRecognitionOnDataset(t *testing.T) {
	files := glyphTestFiles(t)
	if len(files) == 0 {
		t.Skipf("dataset 폴더가 없어 건너뜀 (%s)", glyphTestRoot)
	}
	d := LoadGlyphDict()
	if len(d) == 0 {
		t.Fatal("사전이 비었다")
	}

	nickOK, nickN := 0, 0
	mapOK, mapN := 0, 0
	sizes := map[string]int{}
	var nickBad, mapBad []string

	for _, f := range files {
		im := glyphTestLoad(f)
		if im == nil {
			continue
		}
		b := im.Bounds()
		sizes[b.Size().String()]++
		offX, offY, cw, ch := glyphTestClient(im)

		want := glyphTestDirs[filepath.Base(filepath.Dir(f))]
		nickN++
		got, ok := RecognizeGlyphs(BinarizeGlyph(im, GlyphNickRegion(offX, offY, cw, ch)), d)
		if ok && got == want {
			nickOK++
		} else if len(nickBad) < 5 {
			nickBad = append(nickBad, filepath.Base(f)+" → "+got)
		}

		mapN++
		mp, ok := RecognizeGlyphs(BinarizeGlyph(im, GlyphMapRegion(im, offX, offY, cw, ch)), d)
		if ok && glyphTestMaps[mp] {
			mapOK++
		} else if len(mapBad) < 8 {
			mapBad = append(mapBad, filepath.Base(f)+" → "+mp)
		}
	}

	t.Logf("창 크기 %v", sizes)
	t.Logf("닉네임 %d/%d, 맵 %d/%d", nickOK, nickN, mapOK, mapN)

	if nickOK != nickN {
		t.Errorf("닉네임 %d/%d (100%% 여야 한다): %v", nickOK, nickN, nickBad)
	}
	// 읽을 수 없는 프레임(암전·커서 가림)이 7장 있으므로 그만큼만 허용한다
	if mapN-mapOK > 7 {
		t.Errorf("맵 %d/%d — 실패 %d장은 허용치(7장)를 넘는다: %v",
			mapOK, mapN, mapN-mapOK, mapBad)
	}
}

func BenchmarkGlyphRecognition(b *testing.B) {
	var files []string
	for dir := range glyphTestDirs {
		fs, _ := filepath.Glob(filepath.Join(glyphTestRoot, dir, "*.png"))
		if len(fs) > 15 {
			fs = fs[:15]
		}
		files = append(files, fs...)
	}
	if len(files) == 0 {
		b.Skip("dataset 없음")
	}
	var imgs []image.Image
	for _, f := range files {
		if im := glyphTestLoad(f); im != nil {
			imgs = append(imgs, im)
		}
	}
	d := LoadGlyphDict()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		im := imgs[i%len(imgs)]
		offX, offY, cw, ch := glyphTestClient(im)
		RecognizeGlyphs(BinarizeGlyph(im, GlyphMapRegion(im, offX, offY, cw, ch)), d)
		RecognizeGlyphs(BinarizeGlyph(im, GlyphNickRegion(offX, offY, cw, ch)), d)
	}
}

// TestGlyphLearn 이미 아는 화면으로 학습 경로를 태워, 배운 글리프가 같은 화면을
// 다시 읽을 때 재현되는지 본다.
func TestGlyphLearn(t *testing.T) {
	fs, _ := filepath.Glob(filepath.Join(glyphTestRoot, "dataset-왕무", "baram_*.png"))
	if len(fs) == 0 {
		t.Skip("dataset 없음")
	}
	sort.Strings(fs)
	im := glyphTestLoad(fs[0])
	if im == nil {
		t.Skip("읽기 실패")
	}
	offX, offY, cw, ch := glyphTestClient(im)
	bin := BinarizeGlyph(im, GlyphMapRegion(im, offX, offY, cw, ch))

	// 사전에서 '왕궁외곽길' 글자들을 빼고, 학습으로 복원되는지 본다
	full := LoadGlyphDict()
	partial := GlyphDict{}
	drop := map[string]bool{"왕": true, "궁": true, "외": true, "곽": true, "길": true}
	for k, v := range full {
		if !drop[v] {
			partial[k] = v
		}
	}
	if _, ok := RecognizeGlyphs(bin, partial); ok {
		t.Fatal("글자를 뺐는데도 읽힌다 — 사전 구성이 이상하다")
	}
	learned, err := FitGlyphs(bin, "왕궁외곽길", partial)
	if err != nil {
		t.Fatalf("학습 실패: %v", err)
	}
	if len(learned) != 5 {
		t.Errorf("새 글리프 %d개 (5개 기대): %v", len(learned), learned)
	}
	for k, v := range learned {
		partial[k] = v
	}
	got, ok := RecognizeGlyphs(bin, partial)
	if !ok || got != "왕궁외곽길" {
		t.Errorf("학습 후 재인식 '%s' (ok=%v)", got, ok)
	}
	_ = time.Now
}
