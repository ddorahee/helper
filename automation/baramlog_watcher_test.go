package automation

import "testing"

// 실제 baramlog.com 채팅에서 관찰된 문구로 매칭 규칙 검증.
// 붙여쓰기/어순 뒤바뀜이 흔해서 정확 문구 매칭으로는 대부분 놓친다.
func TestMatchBaramlogTarget(t *testing.T) {
	cases := []struct {
		text      string
		wantMatch bool
		wantLabel string
	}{
		// --- 실제 매칭돼야 하는 글 ---
		{"해적단 시련왕궁격수구해요", true, "왕궁 시련"},
		{"뿌꾸 시련 나타라자 10판 같이 하실 도사분?", true, "나타라자 시련"},
		{"주카마도 나타라자 시련 10판이나 15판가실 도사분", true, "나타라자 시련"},
		{"태궁수 12시땡 나타시련 10판 가실 분?", true, "나타 시련"},
		{"삿월 나타 시련 10~15판 가실 도사분 구합니다", true, "나타 시련"},
		{"마한빛 스투파시련 가실분", true, "스투파 시련"},
		{"마한빛 스투파가실 아무나 시련100", true, "스투파 시련"},
		{"홍수진 시련 스투파가실분 계실까요 ?", true, "스투파 시련"},

		// --- 매칭되면 안 되는 글 ---
		{"빡집중 왕궁 1시간 훈화초 캐러가실분??", false, ""},           // 시련 없음
		{"르샤 신라왕궁 1시간 가실 랏 격수님구해요 2/3", false, ""},      // 시련 없음
		{"소파 나타라자 보리수캐실 격수분 한분 모십니다", false, ""},      // 시련 없음
		{"쇼타임 835 마도사랑 시련 10~15판가실분", false, ""},          // 장소 없음
		{"승기 연마석(용) 17만 50개만 더 팝니다..", false, ""},
	}

	for _, c := range cases {
		label, ok := matchBaramlogTarget(c.text)
		if ok != c.wantMatch {
			t.Errorf("matchBaramlogTarget(%q) = %v, want %v", c.text, ok, c.wantMatch)
			continue
		}
		if ok && label != c.wantLabel {
			t.Errorf("matchBaramlogTarget(%q) label = %q, want %q", c.text, label, c.wantLabel)
		}
	}
}