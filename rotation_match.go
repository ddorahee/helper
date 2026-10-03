package main

import (
	"log"

	"example.com/m/config"
)

// rotationWindowResult 자동사냥 창 감지 결과 한 창분 (/api/rotation/detect-with-ocr 응답)
type rotationWindowResult struct {
	HWND         uint64 `json:"hwnd"`
	Title        string `json:"title"`
	PID          uint32 `json:"pid"`
	DetectedName string `json:"detectedName"`
	NameExact    bool   `json:"nameExact"` // 글리프로 읽은 정확한 이름 (false 면 OCR 추정이거나 못 읽음)
	MatchedID    string `json:"matchedId,omitempty"`
	MatchedName  string `json:"matchedName,omitempty"`
	Confidence   string `json:"confidence"`         // "exact" | "partial"(OCR 유사도) | "remaining"(소거법) | "none"
	NickCrop     string `json:"nickCrop,omitempty"` // 이름을 글자로 못 읽었을 때만 닉네임 이미지(눈으로 구분용)
	Error        string `json:"error,omitempty"`
}

// matchRotationCharacters 창마다 읽은 이름을 등록된 캐릭터에 붙인다.
//
//   - 글리프로 읽은 이름(NameExact)은 정확하다 → **정확히 같은 이름**의 캐릭터에만 붙인다.
//     예전 유사도 매칭을 쓰면 '하루루'와 '햐루루'처럼 한 획 다른 캐릭터에 붙을 수 있다.
//   - OCR 로 추정한 이름은 오인식이 있을 수 있어 예전처럼 유사도로 맞춘다(partial).
//   - 소거법은 **이름을 못 읽은 창**에만 쓴다. 남은 창 수와 남은 캐릭터 수가 같으면 순서대로 배정.
//     이름을 정확히 읽었는데 등록된 캐릭터가 아니면 그 창은 어떤 캐릭터도 아니다 — 소거법 대상이
//     되면 등록 안 된 캐릭터 창이 남은 캐릭터에 붙어버린다.
func matchRotationCharacters(results []rotationWindowResult, chars []config.CharacterProfile) {
	for i := range results {
		res := &results[i]
		name := res.DetectedName
		if name == "" || res.Error != "" {
			continue
		}
		if res.NameExact {
			for _, c := range chars {
				if c.Name == name {
					res.MatchedID, res.MatchedName, res.Confidence = c.ID, c.Name, "exact"
					break
				}
			}
			continue
		}
		// OCR 추정 이름 — 유사도
		bestDist := 999
		nameRunes := []rune(name)
		for _, c := range chars {
			charRunes := []rune(c.Name)
			charLen := len(charRunes)
			if c.Name == name {
				res.MatchedID, res.MatchedName, res.Confidence = c.ID, c.Name, "exact"
				bestDist = 0
				break
			}
			if dist := levenshtein(nameRunes, charRunes); dist <= 2 && dist < bestDist {
				bestDist = dist
				res.MatchedID, res.MatchedName, res.Confidence = c.ID, c.Name, "partial"
			}
			// OCR 결과가 더 길면 캐릭터 이름 길이만큼 미끄러뜨리며 최소 거리 탐색
			if len(nameRunes) > charLen {
				for start := 0; start <= len(nameRunes)-charLen; start++ {
					if d := levenshtein(nameRunes[start:start+charLen], charRunes); d <= 1 && d < bestDist {
						bestDist = d
						res.MatchedID, res.MatchedName, res.Confidence = c.ID, c.Name, "partial"
					}
				}
			}
		}
		if bestDist > 0 && bestDist <= 2 && res.MatchedID != "" {
			log.Printf("[창감지] OCR 유사도 매칭: '%s' ≈ '%s' (거리=%d)", name, res.MatchedName, bestDist)
		}
	}

	// 소거법 — 이름을 못 읽은 창만
	matched := map[string]bool{}
	for _, res := range results {
		if res.MatchedID != "" {
			matched[res.MatchedID] = true
		}
	}
	var leftChars []config.CharacterProfile
	for _, c := range chars {
		if !matched[c.ID] {
			leftChars = append(leftChars, c)
		}
	}
	var leftIdx []int
	for i, res := range results {
		if res.MatchedID == "" && !res.NameExact && res.Error == "" {
			leftIdx = append(leftIdx, i)
		}
	}
	if len(leftIdx) > 0 && len(leftIdx) == len(leftChars) {
		for j, idx := range leftIdx {
			results[idx].MatchedID = leftChars[j].ID
			results[idx].MatchedName = leftChars[j].Name
			results[idx].Confidence = "remaining"
			log.Printf("[창감지] 소거법 매칭: hwnd=%d → '%s'", results[idx].HWND, leftChars[j].Name)
		}
	}
}
