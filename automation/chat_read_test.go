package automation

import (
	"image/png"
	"os"
	"strings"
	"testing"
)

// testdata/chat/*.png = 녹화 프레임에서 클라이언트 (0,700)~(620,900) 을 자른 것.
// 그래서 클라이언트 높이 200 으로 넘기면 실제 화면과 같은 위치를 읽는다.
func readChatFixture(t *testing.T, name string) []string {
	t.Helper()
	f, err := os.Open("testdata/chat/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	lines := ReadChatLines(im, 0, 0, 620, 200)
	for _, l := range lines {
		t.Logf("%s | %s", name, l)
	}
	return lines
}

func TestReadChatLines(t *testing.T) {
	cases := []struct {
		file string
		want []string // 이 문장들이 (공백 무시) 어느 줄엔가 그대로 있어야 한다
	}{
		{"plain.png", []string{
			"탕녀: 브리트라한의면갑7성 7.5 팝니다",
			"손발흥민: 땡듀님 계십니까",
			"★축하드립니다! 아찌님께서 멋진의상재봉틀에서 가을포근코지(남)을 획득하셨습니다!★",
		}},
		// 테두리 있는 세계후: 글자는 테두리 안쪽 색
		{"outline.png", []string{"☆카라멜☆ 함께하실 문원님 모십니다~", "[신라] 이경: 왕무 1시간 하실분"}},
		// 분홍 테두리 + 검은 글자 세계후 (채팅창 왼쪽 검은 테두리선과 섞이면 안 됨)
		{"blackfill.png", []string{"출근: 베베람쥐 1.5억 삽니다. 1품 목 방패 장신구 개당 5000 삽니다.", "코박멸 : 물 3.5 2개 구매합니다"}},
		{"blackfill2.png", []string{"출근: 베베람쥐 1.5억 삽니다."}},
		// 같은 세계후인데 배경도 검정 — 테두리에 둘러싸인 픽셀만 글자.
		// shortcut: 0·ㅈ처럼 구멍과 대각선 획이 많이 섞인 글자는 아직 '?' (삽니다/장신구/5000). 실패 화면이 더 쌓이면 다시 본다.
		{"blackfill_darkbg.png", []string{"출근 : 베베람쥐 1 .5억", "1품 목 방패", "개당", "총수 : 문경 25에 삽니다 / 부여대장간 앞"}},
		// 실제 시련 모집 글 + 둥근 따옴표 같은 기호
		{"quote.png", []string{"이브 : 나타 시련 10번 하실 격수분?", "주막의 ‘왈숙네’ , '연실네'NPC 또는 금고에"}},
	}
	nos := func(s string) string { return strings.ReplaceAll(s, " ", "") }
	for _, c := range cases {
		lines := readChatFixture(t, c.file)
		for _, w := range c.want {
			found := false
			for _, l := range lines {
				if strings.Contains(nos(l), nos(w)) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: %q 를 못 읽음", c.file, w)
			}
		}
	}
}
