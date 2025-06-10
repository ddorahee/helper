// keymapping/utils.go - 원시 키 코드 기반으로 수정
package keymapping

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// rawKeyCodeToString 원시 키 코드를 문자열로 변환 (수정됨)
func (km *KeyMappingManager) rawKeyCodeToString(rawKeycode uint16) string {
	// 허용된 시작키만 처리 (원시 키 코드 기준)
	switch rawKeycode {
	case 46: // Delete 키 (원시)
		return "delete"
	case 35: // End 키 (원시)
		return "end"
	}

	// 시작키가 아닌 다른 키는 빈 문자열 반환 (무시)
	return ""
}

// getAllowedRawKeyCodes 허용된 원시 키 코드 목록 반환
func (km *KeyMappingManager) getAllowedRawKeyCodes() []uint16 {
	return []uint16{
		46, // Delete (원시 키 코드)
		35, // End (원시 키 코드)
	}
}

// stringToRawKeyCode 문자열을 원시 키 코드로 변환
func (km *KeyMappingManager) stringToRawKeyCode(keyStr string) uint16 {
	keyStr = strings.ToLower(keyStr)

	// 시작키 매핑 (원시 키 코드 기준)
	startKeyMap := map[string]uint16{
		"delete": 46, // Delete 원시 키 코드
		"end":    35, // End 원시 키 코드
	}

	if code, exists := startKeyMap[keyStr]; exists {
		return code
	}

	return 0
}

// GetAvailableKeys 사용 가능한 키 목록 반환
func (km *KeyMappingManager) GetAvailableKeys() map[string][]string {
	return map[string][]string{
		"시작 키":  {"delete", "end"}, // 시작키는 이 두 개만
		"숫자 키":  {"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
		"알파벳 키": {"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"},
		"펑션 키":  {"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"},
		"특수 키":  {"space", "enter", "esc", "tab"},
		"화살표 키": {"left", "up", "right", "down"},
		"넘패드":   {"num0", "num1", "num2", "num3", "num4", "num5", "num6", "num7", "num8", "num9"},
		"조합 키":  {"shift", "ctrl", "alt"},
	}
}

// LoadConfig 설정 파일 로드
func (km *KeyMappingManager) LoadConfig() error {
	if _, err := os.Stat(km.configFile); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(km.configFile)
	if err != nil {
		return fmt.Errorf("설정 파일 읽기 실패: %v", err)
	}

	var mappings []*KeyMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return fmt.Errorf("설정 파일 파싱 실패: %v", err)
	}

	km.mutex.Lock()
	defer km.mutex.Unlock()

	km.mappings = make(map[string]*KeyMapping)
	for _, mapping := range mappings {
		if km.isValidStartKey(mapping.StartKey) {
			km.mappings[mapping.StartKey] = mapping
		}
	}

	return nil
}

// isValidStartKey 시작키가 유효한지 확인
func (km *KeyMappingManager) isValidStartKey(key string) bool {
	allowedKeys := []string{"delete", "end"}
	for _, validKey := range allowedKeys {
		if strings.ToLower(key) == strings.ToLower(validKey) {
			return true
		}
	}
	return false
}

// SaveConfig 설정 파일 저장
func (km *KeyMappingManager) SaveConfig() error {
	mappings := make([]*KeyMapping, 0, len(km.mappings))
	for _, mapping := range km.mappings {
		mappings = append(mappings, mapping)
	}

	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("설정 직렬화 실패: %v", err)
	}

	if err := os.WriteFile(km.configFile, data, 0644); err != nil {
		return fmt.Errorf("설정 파일 저장 실패: %v", err)
	}

	return nil
}

// ParseKeySequence 문자열에서 키 시퀀스 파싱
func (km *KeyMappingManager) ParseKeySequence(sequence string) ([]MappedKey, error) {
	if sequence == "" {
		return nil, fmt.Errorf("키 시퀀스가 비어있습니다")
	}

	keys := strings.Split(sequence, ",")
	result := make([]MappedKey, 0, len(keys))

	for i, keyStr := range keys {
		keyStr = strings.TrimSpace(keyStr)
		if keyStr == "" {
			continue
		}

		key, delay, err := km.parseKeyWithDelay(keyStr)
		if err != nil {
			return nil, fmt.Errorf("키 %d 파싱 실패: %v", i+1, err)
		}

		result = append(result, MappedKey{
			Key:   key,
			Delay: delay,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("유효한 키가 없습니다")
	}

	return result, nil
}

// parseKeyWithDelay 개별 키와 딜레이 파싱
func (km *KeyMappingManager) parseKeyWithDelay(keyStr string) (string, int, error) {
	defaultDelay := 0

	if !strings.Contains(keyStr, "(") {
		return strings.ToLower(keyStr), defaultDelay, nil
	}

	openIdx := strings.Index(keyStr, "(")
	closeIdx := strings.Index(keyStr, ")")

	if openIdx == -1 || closeIdx == -1 || closeIdx <= openIdx {
		return "", 0, fmt.Errorf("잘못된 형식: %s", keyStr)
	}

	key := strings.TrimSpace(keyStr[:openIdx])
	delayStr := strings.TrimSpace(keyStr[openIdx+1 : closeIdx])

	if key == "" {
		return "", 0, fmt.Errorf("키가 비어있습니다")
	}

	delay, err := strconv.Atoi(delayStr)
	if err != nil {
		return "", 0, fmt.Errorf("딜레이 파싱 실패: %s", delayStr)
	}

	if delay < 0 || delay > 1000 {
		return "", 0, fmt.Errorf("딜레이는 0~1000ms 사이여야 합니다: %d", delay)
	}

	return strings.ToLower(key), delay, nil
}

// FormatKeySequence 키 시퀀스를 문자열로 포맷팅
func (km *KeyMappingManager) FormatKeySequence(keys []MappedKey) string {
	if len(keys) == 0 {
		return ""
	}

	parts := make([]string, len(keys))
	for i, key := range keys {
		if key.Delay == 0 {
			parts[i] = fmt.Sprintf("%s(즉시)", key.Key)
		} else {
			parts[i] = fmt.Sprintf("%s(%dms)", key.Key, key.Delay)
		}
	}

	return strings.Join(parts, ", ")
}

// IsValidKey 키가 유효한지 확인
func (km *KeyMappingManager) IsValidKey(key string) bool {
	availableKeys := km.GetAvailableKeys()
	key = strings.ToLower(key)

	for category, keyList := range availableKeys {
		if category == "시작 키" {
			continue
		}
		for _, validKey := range keyList {
			if key == strings.ToLower(validKey) {
				return true
			}
		}
	}

	return false
}

// GetMappingStats 맵핑 통계 반환
func (km *KeyMappingManager) GetMappingStats() map[string]interface{} {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	total := len(km.mappings)
	enabled := 0
	disabled := 0

	for _, mapping := range km.mappings {
		if mapping.Enabled {
			enabled++
		} else {
			disabled++
		}
	}

	return map[string]interface{}{
		"total":    total,
		"enabled":  enabled,
		"disabled": disabled,
		"running":  km.running,
		"os":       runtime.GOOS,
	}
}
