package automation

import "testing"

// 슬롯 등록 시 '전환 시 복사할 텍스트'를 캐릭터 이름으로 자동으로 채우되,
// 사용자가 직접 고친 칸은 다시 덮어쓰지 않아야 한다.
func TestSwitcherAutofillClip(t *testing.T) {
	ws := NewWindowSwitcher(nil)
	nick := "명멸멸"
	readable := true
	ws.SetNickReader(func(hwnd uint64) (string, bool) {
		if !readable {
			return "", false
		}
		return nick, true
	})

	get := func() string {
		ws.mu.Lock()
		defer ws.mu.Unlock()
		return ws.clipText[0]
	}

	// 1) 빈 칸이면 채운다
	ws.autofillClip(0, 111)
	if got := get(); got != "명멸멸" {
		t.Fatalf("자동 채움 실패: %q", got)
	}

	// 2) 자동으로 채운 값은 다음 등록 때 새 이름으로 갱신된다
	nick = "데브섹옵스"
	ws.autofillClip(0, 222)
	if got := get(); got != "데브섹옵스" {
		t.Errorf("자동 채운 칸이 갱신되지 않았다: %q", got)
	}

	// 3) 사용자가 직접 고치면 그 뒤로는 안 건드린다
	if err := ws.SetText(1, "내가 쓴 문구"); err != nil {
		t.Fatal(err)
	}
	nick = "햐루루"
	ws.autofillClip(0, 333)
	if got := get(); got != "내가 쓴 문구" {
		t.Errorf("직접 고친 칸을 덮어썼다: %q", got)
	}

	// 4) 칸을 비우면 다시 자동 채움 대상이 된다
	if err := ws.SetText(1, ""); err != nil {
		t.Fatal(err)
	}
	ws.autofillClip(0, 444)
	if got := get(); got != "햐루루" {
		t.Errorf("비운 뒤 다시 채워지지 않았다: %q", got)
	}

	// 5) 이름을 못 읽는 창(게임 창이 아님)은 건드리지 않는다
	readable = false
	ws.autofillClip(0, 555)
	if got := get(); got != "햐루루" {
		t.Errorf("읽기 실패인데 값이 바뀌었다: %q", got)
	}

	// 6) 리더가 없으면 아무 일도 없어야 한다
	ws2 := NewWindowSwitcher(nil)
	ws2.autofillClip(0, 111)
	ws2.mu.Lock()
	empty := ws2.clipText[0]
	ws2.mu.Unlock()
	if empty != "" {
		t.Errorf("리더 없이 값이 들어갔다: %q", empty)
	}
}
