// keymapping/utils.go 완전한 코드 - 최종 버전 (한국어 기계식 키보드 지원)
package keymapping

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// keyCodeToString 키 코드를 문자열로 변환 - 한국어 기계식 키보드 지원
func (km *KeyMappingManager) keyCodeToString(keycode uint16) string {
	// 한국어 기계식 키보드 특수 키 처리
	switch keycode {
	case 61011: // 한국어 기계식 키보드 Delete 키 (0xEE53)
		return "delete"
	case 46: // 표준 Delete 키
		return "delete"
	case 8: // Backspace
		return "delete"
	case 127: // ASCII Delete
		return "delete"
	case 35: // End 키
		return "end"
	}

	// 일반 키 매핑
	keyMap := map[uint16]string{
		// 숫자 키
		49: "1", 50: "2", 51: "3", 52: "4", 53: "5",
		54: "6", 55: "7", 56: "8", 57: "9", 48: "0",

		// 알파벳 키
		65: "a", 66: "b", 67: "c", 68: "d", 69: "e",
		70: "f", 71: "g", 72: "h", 73: "i", 74: "j",
		75: "k", 76: "l", 77: "m", 78: "n", 79: "o",
		80: "p", 81: "q", 82: "r", 83: "s", 84: "t",
		85: "u", 86: "v", 87: "w", 88: "x", 89: "y",
		90: "z",

		// 펑션 키
		112: "f1", 113: "f2", 114: "f3", 115: "f4",
		116: "f5", 117: "f6", 118: "f7", 119: "f8",
		120: "f9", 121: "f10", 122: "f11", 123: "f12",

		// 화살표 키
		37: "left", 38: "up", 39: "right", 40: "down",

		// 기타 키
		32: "space", 13: "enter", 27: "esc", 9: "tab",
		16: "shift", 17: "ctrl", 18: "alt",

		// 넘패드
		96: "num0", 97: "num1", 98: "num2", 99: "num3",
		100: "num4", 101: "num5", 102: "num6", 103: "num7",
		104: "num8", 105: "num9",
	}

	if key, exists := keyMap[keycode]; exists {
		return key
	}

	return ""
}

// stringToKeyCode 문자열을 키 코드로 변환
func (km *KeyMappingManager) stringToKeyCode(keyStr string) uint16 {
	keyStr = strings.ToLower(keyStr)

	// 시작키 매핑
	startKeyMap := map[string]uint16{
		"end":    35,    // End 키
		"delete": 61011, // 한국어 기계식 키보드 Delete 키
	}

	// 일반 키 매핑
	keyMap := map[string]uint16{
		// 숫자 키
		"1": 49, "2": 50, "3": 51, "4": 52, "5": 53,
		"6": 54, "7": 55, "8": 56, "9": 57, "0": 48,

		// 알파벳 키
		"a": 65, "b": 66, "c": 67, "d": 68, "e": 69,
		"f": 70, "g": 71, "h": 72, "i": 73, "j": 74,
		"k": 75, "l": 76, "m": 77, "n": 78, "o": 79,
		"p": 80, "q": 81, "r": 82, "s": 83, "t": 84,
		"u": 85, "v": 86, "w": 87, "x": 88, "y": 89,
		"z": 90,

		// 펑션 키
		"f1": 112, "f2": 113, "f3": 114, "f4": 115,
		"f5": 116, "f6": 117, "f7": 118, "f8": 119,
		"f9": 120, "f10": 121, "f11": 122, "f12": 123,

		// 화살표 키
		"left": 37, "up": 38, "right": 39, "down": 40,

		// 기타 키
		"space": 32, "enter": 13, "esc": 27, "tab": 9,
		"shift": 16, "ctrl": 17, "alt": 18,

		// 넘패드
		"num0": 96, "num1": 97, "num2": 98, "num3": 99,
		"num4": 100, "num5": 101, "num6": 102, "num7": 103,
		"num8": 104, "num9": 105,
	}

	// 시작키 먼저 확인
	if code, exists := startKeyMap[keyStr]; exists {
		return code
	}

	// 일반 키 확인
	if code, exists := keyMap[keyStr]; exists {
		return code
	}

	return 0
}

// GetAvailableKeys 사용 가능한 키 목록 반환
func (km *KeyMappingManager) GetAvailableKeys() map[string][]string {
	return map[string][]string{
		"시작 키":  {"delete", "end"},
		"숫자 키":  {"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
		"알파벳 키": {"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"},
		"펑션 키":  {"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"},
		"특수 키":  {"space", "enter", "esc", "tab"},
		"화살표 키": {"left", "up", "right", "down"},
		"넘패드":   {"num0", "num1", "num2", "num3", "num4", "num5", "num6", "num7", "num8", "num9"},
		"조합 키":  {"shift", "ctrl", "alt"},
	}
}

// GetStartKeys 시작키 목록만 반환
func (km *KeyMappingManager) GetStartKeys() []string {
	return []string{"delete", "end"}
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
