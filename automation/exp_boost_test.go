package automation

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// 경험치 칸 감지 — 실제 스샷에서 칸 둘레만 잘라 둔 그림으로 확인한다(캐릭터 이름은 안 들어감).
// 전체 스샷 765장(시련 9 · 왕무 200 · 사막 200 · 왕궁 203 · 나타 111 · 악귀문 49)으로도 오판 0을 확인했다.
func TestDetectExpBoost(t *testing.T) {
	cases := []struct {
		file   string
		found  bool
		active [3]bool
	}{
		{"on3.png", true, [3]bool{true, true, true}},          // 시련 스샷 — 3칸 다 켜짐
		{"on3_shifted.png", true, [3]bool{true, true, true}},  // 파티 체력바 때문에 버프 창이 밀린 창
		{"off3.png", true, [3]bool{false, false, false}},      // 3칸 다 꺼짐 (나나노·요강·물약 회색)
		{"partial.png", true, [3]bool{false, true, true}},     // 나나노만 꺼짐
		{"tiles.png", true, [3]bool{false, true, true}},       // 위쪽 맵 바닥 타일이 꺼진 칸 색과 비슷 — 진짜 칸을 골라야 함
		{"covered.png", false, [3]bool{}},                     // 창고 창이 덮음 — 못 찾음
	}
	for _, c := range cases {
		f, err := os.Open(filepath.Join("testdata", "expboost", c.file))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		got := DetectExpBoost(img, img.Bounds())
		if got.Found != c.found || got.Active != c.active {
			t.Errorf("%s: got found=%v active=%v (%s), want found=%v active=%v", c.file, got.Found, got.Active, got, c.found, c.active)
		}
	}
}

func TestExpBoostText(t *testing.T) {
	if s := (ExpBoost{Found: true, Active: [3]bool{false, true, false}}).String(); s != "나나노·물약 꺼짐" {
		t.Errorf("String = %q", s)
	}
	if !(ExpBoost{Found: true, Active: [3]bool{true, true, true}}).AllActive() {
		t.Error("3칸 켜짐인데 AllActive=false")
	}
	if (ExpBoost{Active: [3]bool{true, true, true}}).AllActive() {
		t.Error("못 찾았는데 AllActive=true")
	}
}
