package automation

import (
	"reflect"
	"testing"
)

// 시련 맵 이름 → 위치. 글리프는 정확한 이름을, OCR 보완은 한글만 남거나 글자가 틀린 이름을 준다.
// 메인화면 사냥맵(환상 없음)은 시련 던전이 아니다.
func TestTrialPlaceOfMap(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		// 글리프로 읽는 정확한 이름
		{"환상의시련장", "lobby"},
		{"[환상]칸첸중가설산", "kanchen"},
		{"[환상]대야전투", "daeya"},
		{" 환상의시련장 ", "lobby"},
		// 메인화면 맵 — 시련 던전 아님
		{"칸첸중가설산", ""},
		{"대야전투", ""},
		{"칸첸중가설산초입", ""},
		{"대야산기슭", ""},
		{"국내성", ""},
		// 못 읽음
		{"", ""},
		{"   ", ""},
		// OCR 보완 값 (한글만 남음 / 글자 틀림) — 시련장은 예전처럼 너그럽게
		{"환상의시련", "lobby"},
		{"상의시련장", "lobby"},
		{"환상의시련징", "lobby"},
		{"환싱의시렌장", "lobby"}, // 키워드 없이 편집 거리 2
		{"환상칸첸중가설산", "kanchen"},
		{"환상대야전투", "daeya"},
	}
	for _, c := range cases {
		if got := TrialPlaceOfMap(c.name); got != c.want {
			t.Errorf("TrialPlaceOfMap(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// 칸첸 시련 스킬 키 — 시련 전용 표에서 캐릭터 키가 있으면 그것, 없거나 이름을 못 읽었으면 기본 키.
func TestTrialSkillKeysFor(t *testing.T) {
	keys := []string{"1", "2"}
	byChar := map[string][]string{"명멸멸": {"3", "4"}, "하루루": {}}
	cases := []struct {
		name string
		want []string
	}{
		{"명멸멸", []string{"3", "4"}},
		{" 명멸멸 ", []string{"3", "4"}}, // 앞뒤 공백 무시
		{"데브섹옵스", []string{"1", "2"}},
		{"하루루", []string{"1", "2"}}, // 캐릭터 키가 비면 기본 키
		{"", []string{"1", "2"}},    // 이름 못 읽음
	}
	for _, c := range cases {
		if got := TrialSkillKeysFor(keys, byChar, c.name); !reflect.DeepEqual(got, c.want) {
			t.Errorf("TrialSkillKeysFor(%q) = %v, want %v", c.name, got, c.want)
		}
	}
	if got := TrialSkillKeysFor(nil, nil, "명멸멸"); len(got) != 0 {
		t.Errorf("아무것도 없으면 빈 키여야 함: %v", got)
	}
}

// 시련 설정 정리 — 던전 daeya|kanchen, 횟수 1~99(아니면 10), 목표 좌표 0이면 기본값.
func TestTrialSettingsNormalize(t *testing.T) {
	dungeons := map[string]string{
		"kanchen":    "kanchen",
		" Kanchen ":  "kanchen",
		"daeya":      "daeya",
		"":           "daeya",
		"wangmu":     "daeya",
		"kanchen123": "daeya",
	}
	for in, want := range dungeons {
		if got := NormalizeTrialDungeon(in); got != want {
			t.Errorf("NormalizeTrialDungeon(%q) = %q, want %q", in, got, want)
		}
	}

	runs := map[int]int{0: 10, -3: 10, 1: 1, 25: 25, 99: 99, 100: 10}
	for in, want := range runs {
		if got := NormalizeTrialMaxRuns(in); got != want {
			t.Errorf("NormalizeTrialMaxRuns(%d) = %d, want %d", in, got, want)
		}
	}

	if x, y := TrialDaeyaTarget(0, 0); x != 29 || y != 32 {
		t.Errorf("TrialDaeyaTarget(0,0) = (%d,%d), want (29,32)", x, y)
	}
	if x, y := TrialDaeyaTarget(30, 0); x != 30 || y != 32 {
		t.Errorf("TrialDaeyaTarget(30,0) = (%d,%d), want (30,32)", x, y)
	}
	if x, y := TrialKanchenTarget(0, 0); x != 34 || y != 37 {
		t.Errorf("TrialKanchenTarget(0,0) = (%d,%d), want (34,37)", x, y)
	}
	if x, y := TrialKanchenTarget(35, 36); x != 35 || y != 36 {
		t.Errorf("TrialKanchenTarget(35,36) = (%d,%d), want (35,36)", x, y)
	}
}

// 상태 — 시작 전엔 다음 시작에 쓸 설정(SetConfig)을 보여준다. 창·키는 건드리지 않는다.
func TestTrialStatusIdle(t *testing.T) {
	tr := NewTrial(nil, nil, nil)
	want := TrialStatus{Running: false, RunCount: 0, MaxRuns: 10, Dungeon: "daeya", Phase: "대기"}
	if got := tr.Status(); got != want {
		t.Errorf("Status() = %+v, want %+v", got, want)
	}
	tr.SetConfig(TrialConfig{MaxRuns: 5, Dungeon: TrialDungeonKanchen, TargetX: 34, TargetY: 37})
	want.MaxRuns, want.Dungeon = 5, "kanchen"
	if got := tr.Status(); got != want {
		t.Errorf("SetConfig 뒤 Status() = %+v, want %+v", got, want)
	}
}
