package main

import (
	"testing"

	"example.com/m/config"
)

func TestMatchRotationCharacters(t *testing.T) {
	chars := []config.CharacterProfile{
		{ID: "c1", Name: "명멸멸"},
		{ID: "c2", Name: "데브섹옵스"},
		{ID: "c3", Name: "하루루"},
	}

	t.Run("글리프 이름은 정확히 같을 때만 배정", func(t *testing.T) {
		rs := []rotationWindowResult{
			{HWND: 1, DetectedName: "명멸멸", NameExact: true},
			{HWND: 2, DetectedName: "햐루루", NameExact: true}, // 등록 안 됨. '하루루'와 한 획 차이
		}
		matchRotationCharacters(rs, chars)
		if rs[0].MatchedID != "c1" || rs[0].Confidence != "exact" {
			t.Errorf("명멸멸 → %+v", rs[0])
		}
		if rs[1].MatchedID != "" {
			t.Errorf("등록 안 된 햐루루가 %q 에 붙었다 (%s)", rs[1].MatchedName, rs[1].Confidence)
		}
	})

	t.Run("이름을 읽은 미등록 창은 소거법 대상이 아니다", func(t *testing.T) {
		// 예전엔 남은 창 1 = 남은 캐릭터 1 이면 순서대로 붙였다 → 미등록 창이 남은 캐릭터를 가져갔다
		rs := []rotationWindowResult{
			{HWND: 1, DetectedName: "명멸멸", NameExact: true},
			{HWND: 2, DetectedName: "데브섹옵스", NameExact: true},
			{HWND: 3, DetectedName: "다른캐릭", NameExact: true},
		}
		matchRotationCharacters(rs, chars) // 남은 캐릭터: 하루루 1개, 남은 창: '다른캐릭' 1개
		if rs[2].MatchedID != "" {
			t.Errorf("미등록 '다른캐릭' 창이 %q 에 배정됐다 (%s)", rs[2].MatchedName, rs[2].Confidence)
		}
	})

	t.Run("이름을 못 읽은 창은 소거법으로 배정", func(t *testing.T) {
		rs := []rotationWindowResult{
			{HWND: 1, DetectedName: "명멸멸", NameExact: true},
			{HWND: 2, DetectedName: "데브섹옵스", NameExact: true},
			{HWND: 3, DetectedName: "", NameExact: false},
		}
		matchRotationCharacters(rs, chars)
		if rs[2].MatchedID != "c3" || rs[2].Confidence != "remaining" {
			t.Errorf("못 읽은 창 → %+v (하루루 소거법 기대)", rs[2])
		}
	})

	t.Run("OCR 추정 이름은 예전처럼 유사도", func(t *testing.T) {
		rs := []rotationWindowResult{
			{HWND: 1, DetectedName: "명멸말", NameExact: false}, // OCR 오인식 1글자
		}
		matchRotationCharacters(rs, chars)
		if rs[0].MatchedID != "c1" || rs[0].Confidence != "partial" {
			t.Errorf("OCR '명멸말' → %+v (명멸멸 부분일치 기대)", rs[0])
		}
	})

	t.Run("캡처 실패 창은 건드리지 않는다", func(t *testing.T) {
		rs := []rotationWindowResult{
			{HWND: 1, DetectedName: "명멸멸", NameExact: true},
			{HWND: 2, DetectedName: "데브섹옵스", NameExact: true},
			{HWND: 3, Error: "창 캡처 실패"},
		}
		matchRotationCharacters(rs, chars)
		if rs[2].MatchedID != "" {
			t.Errorf("캡처 실패 창이 %q 에 배정됐다", rs[2].MatchedName)
		}
	})
}
