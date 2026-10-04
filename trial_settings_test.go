package main

import (
	"testing"

	"example.com/m/config"
)

// 화면이 보낸 횟수: 숫자가 바뀌면 '처음 값'으로도 기억, 같은 값이 같이 오는 저장은 처음 값을 안 바꿈,
// 시련이 도는 동안엔 무시(엔진이 판마다 줄여 저장하는 값을 옛 화면 값이 덮지 않게).
func TestTrialApplyMaxRuns(t *testing.T) {
	s := config.TrialSettings{MaxRuns: 10, FullRuns: 10}

	s = trialApplyMaxRuns(s, 7, false) // 사용자가 7로 바꿈
	if s.MaxRuns != 7 || s.FullRuns != 7 {
		t.Fatalf("7로 바꿈: %+v", s)
	}
	s.MaxRuns = 4                      // 엔진이 판마다 줄여 저장한 상태
	s = trialApplyMaxRuns(s, 4, false) // 스킬 키 저장 등으로 같은 횟수가 같이 옴
	if s.MaxRuns != 4 || s.FullRuns != 7 {
		t.Fatalf("같은 값 저장이 처음 값을 바꿈: %+v", s)
	}
	s = trialApplyMaxRuns(s, 10, true) // 도는 중 옛 화면 값
	if s.MaxRuns != 4 || s.FullRuns != 7 {
		t.Fatalf("도는 중인데 바뀜: %+v", s)
	}
}
