package automation

import (
	"reflect"
	"testing"
	"time"
)

// 칸첸 캐릭터별 스킬 키 — 캐릭터 키가 있으면 그것, 없거나 이름을 못 읽었으면 기본 키.
func TestSkillKeysFor(t *testing.T) {
	cfg := ItemScannerConfig{
		SkillKeys:       []string{"1", "2", "8"},
		SkillKeysByChar: map[string][]string{"명멸멸": {"3", "4"}},
	}
	cases := []struct {
		name string
		want []string
	}{
		{"명멸멸", []string{"3", "4"}},
		{" 명멸멸 ", []string{"3", "4"}}, // 앞뒤 공백 무시
		{"데브섹옵스", []string{"1", "2", "8"}},
		{"", []string{"1", "2", "8"}}, // 이름 못 읽음
	}
	for _, c := range cases {
		if got := cfg.SkillKeysFor(c.name); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SkillKeysFor(%q) = %v, want %v", c.name, got, c.want)
		}
	}
	if got := (ItemScannerConfig{}).SkillKeysFor("명멸멸"); len(got) != 0 {
		t.Errorf("아무것도 없으면 빈 키여야 함: %v", got)
	}
}

func TestNormalizeSkillKeys(t *testing.T) {
	got := NormalizeSkillKeys([]string{" 1", "D", "", "  ", "x "})
	if want := []string{"1", "d", "x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("NormalizeSkillKeys = %v, want %v", got, want)
	}
	m := NormalizeSkillKeysByChar(map[string][]string{
		" 명멸멸 ": {"3", " 4"},
		"하루루":   {"", " "}, // 키를 비우면 빠진다(기본 키를 쓰게)
		"":      {"1"},
	})
	if want := map[string][]string{"명멸멸": {"3", "4"}}; !reflect.DeepEqual(m, want) {
		t.Errorf("NormalizeSkillKeysByChar = %v, want %v", m, want)
	}
	if NormalizeSkillKeysByChar(map[string][]string{"하루루": {}}) != nil {
		t.Error("전부 비면 nil 이어야 함(저장 파일에서 빠지게)")
	}
}

// 칸첸(입장) 시퀀스의 스킬 자리(esc 뒤)만 캐릭터 키로 바뀐다.
func TestWithSkillKeys(t *testing.T) {
	s := time.Second
	got := withSkillKeys(DefaultKanchenEnterSequence, []string{"1", "2", "8"})
	if want := []string{"o", "enter", "enter", "esc", "1", "2", "8"}; !reflect.DeepEqual(got.KeyPresses, want) {
		t.Errorf("keys = %v, want %v", got.KeyPresses, want)
	}
	if want := []time.Duration{3 * s, s, s, s, kanchenSkillGap, kanchenSkillGap}; !reflect.DeepEqual(got.Delays, want) {
		t.Errorf("delays = %v, want %v", got.Delays, want)
	}
	// 원본은 그대로
	if want := []string{"o", "enter", "enter", "esc", "d"}; !reflect.DeepEqual(DefaultKanchenEnterSequence.KeyPresses, want) {
		t.Errorf("기본 시퀀스가 바뀜: %v", DefaultKanchenEnterSequence.KeyPresses)
	}
	// 키가 없으면 그대로(기존 d)
	if same := withSkillKeys(DefaultKanchenEnterSequence, nil); !reflect.DeepEqual(same, DefaultKanchenEnterSequence) {
		t.Errorf("빈 키면 원래 시퀀스여야 함: %v", same.KeyPresses)
	}
	// esc 가 없는(직접 바꾼) 시퀀스는 뒤에 붙인다. 대기 시간이 모자라면 1초.
	custom := KeySequence{KeyPresses: []string{"x", "d"}, Delays: []time.Duration{2 * s}}
	got = withSkillKeys(custom, []string{"5"})
	if want := []string{"x", "d", "5"}; !reflect.DeepEqual(got.KeyPresses, want) {
		t.Errorf("custom keys = %v, want %v", got.KeyPresses, want)
	}
	if want := []time.Duration{2 * s, s}; !reflect.DeepEqual(got.Delays, want) {
		t.Errorf("custom delays = %v, want %v", got.Delays, want)
	}
	// 대문자 ESC 도 입장 키 끝으로 본다
	upper := KeySequence{KeyPresses: []string{"o", "ESC", "d"}, Delays: []time.Duration{s, s}}
	if got := withSkillKeys(upper, []string{"3"}); !reflect.DeepEqual(got.KeyPresses, []string{"o", "ESC", "3"}) {
		t.Errorf("ESC 처리 = %v", got.KeyPresses)
	}
}
