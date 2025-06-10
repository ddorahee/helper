// keymapping/utils.go 완전한 코드
package keymapping

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// keyCodeToString 키 코드를 문자열로 변환 (시작키만 포함)
func (km *KeyMappingManager) keyCodeToString(keycode uint16) string {
	// 시작키로 사용할 수 있는 키 코드만 매핑
	keyMap := map[uint16]string{
		// 시작키로 허용되는 키들만
		35: "end",    // End 키
		46: "delete", // Delete 키
	}

	if key, exists := keyMap[keycode]; exists {
		return key
	}

	// 시작키가 아닌 다른 키는 빈 문자열 반환
	return ""
}

// stringToKeyCode 문자열을 키 코드로 변환 (시작키 포함)
func (km *KeyMappingManager) stringToKeyCode(keyStr string) uint16 {
	keyStr = strings.ToLower(keyStr)

	// 시작키 매핑
	startKeyMap := map[string]uint16{
		"end":    35, // End 키
		"delete": 46, // Delete 키
	}

	// 일반 키 매핑 (실행할 키들)
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

// GetAvailableKeys 사용 가능한 키 목록 반환 (시작키 분리)
func (km *KeyMappingManager) GetAvailableKeys() map[string][]string {
	return map[string][]string{
		"시작 키":  {"delete", "end"}, // 시작키는 이 두개만
		"숫자 키":  {"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
		"알파벳 키": {"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"},
		"펑션 키":  {"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"},
		"특수 키":  {"space", "enter", "esc", "tab"},
		"화살표 키": {"left", "up", "right", "down"},
		"넘패드":   {"num0", "num1", "num2", "num3", "num4", "num5", "num6", "num7", "num8", "num9"},
		"조합 키":  {"shift", "ctrl", "alt"},
	}
}

// GetStartKeys 시작키 목록만 반환하는 새로운 함수
func (km *KeyMappingManager) GetStartKeys() []string {
	return []string{"delete", "end"}
}

// LoadConfig 설정 파일 로드
func (km *KeyMappingManager) LoadConfig() error {
	// 파일이 존재하지 않으면 빈 설정으로 시작
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

	// 맵핑 로드
	km.mappings = make(map[string]*KeyMapping)
	for _, mapping := range mappings {
		// 기존 설정에서 유효하지 않은 시작키는 제외
		if km.isValidStartKey(mapping.StartKey) {
			km.mappings[mapping.StartKey] = mapping
		}
	}

	return nil
}

// isValidStartKey 시작키가 유효한지 확인하는 헬퍼 함수
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
	// 맵핑을 슬라이스로 변환
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

		// 키와 딜레이 파싱
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

// parseKeyWithDelay 개별 키와 딜레이 파싱 (딜레이 범위 수정: 0ms~1000ms)
func (km *KeyMappingManager) parseKeyWithDelay(keyStr string) (string, int, error) {
	// 기본 딜레이 (0ms로 변경)
	defaultDelay := 0

	// 괄호가 없는 경우
	if !strings.Contains(keyStr, "(") {
		return strings.ToLower(keyStr), defaultDelay, nil
	}

	// 괄호 파싱
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

	// 딜레이 범위 수정: 0ms~1000ms
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
		parts[i] = fmt.Sprintf("%s(%d)", key.Key, key.Delay)
	}

	return strings.Join(parts, ", ")
}

// IsValidKey 키가 유효한지 확인 (실행키만 확인)
func (km *KeyMappingManager) IsValidKey(key string) bool {
	availableKeys := km.GetAvailableKeys()

	key = strings.ToLower(key)

	// 시작키는 별도로 확인하지 않음 (실행키만 확인)
	for category, keyList := range availableKeys {
		if category == "시작 키" {
			continue // 시작키는 건너뜀
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
	}
}
