// keymapping/utils.go - 조합키 지원 및 중복키 처리
package keymapping

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// rawKeyCodeToString 원시 키 코드를 문자열로 변환
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

// GetAvailableKeys 사용 가능한 키 목록 반환 (조합키 지원)
func (km *KeyMappingManager) GetAvailableKeys() map[string][]string {
	return map[string][]string{
		"시작 키":  {"delete", "end"}, // 시작키는 이 두 개만
		"숫자 키":  {"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
		"알파벳 키": {"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"},
		"펑션 키":  {"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"},
		"특수 키":  {"space", "enter", "esc", "tab", "backspace", "delete", "home", "end", "pageup", "pagedown", "insert"},
		"화살표 키": {"left", "up", "right", "down"},
		"넘패드":   {"num0", "num1", "num2", "num3", "num4", "num5", "num6", "num7", "num8", "num9", "num_add", "num_subtract", "num_multiply", "num_divide", "num_enter", "num_decimal"},
		"수정키":   {"shift", "ctrl", "alt", "cmd", "win"},
		"조합키 예시": {
			"ctrl+c", "ctrl+v", "ctrl+x", "ctrl+z", "ctrl+y", "ctrl+a", "ctrl+s",
			"shift+tab", "shift+f10", "shift+home", "shift+end",
			"alt+f4", "alt+tab", "alt+enter", "alt+left", "alt+right",
			"ctrl+shift+n", "ctrl+alt+delete", "ctrl+shift+esc",
			"win+r", "win+l", "win+d", "win+tab", "win+x",
		},
	}
}

// LoadConfig 설정 파일 로드 (중복키 지원)
func (km *KeyMappingManager) LoadConfig() error {
	if _, err := os.Stat(km.configFile); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(km.configFile)
	if err != nil {
		return fmt.Errorf("설정 파일 읽기 실패: %v", err)
	}

	// 새로운 형식 (중복키 지원)
	var newMappings map[string][]*KeyMapping
	if err := json.Unmarshal(data, &newMappings); err == nil {
		km.mutex.Lock()
		defer km.mutex.Unlock()

		km.mappings = make(map[string][]*KeyMapping)
		for key, mappings := range newMappings {
			if km.isValidStartKey(key) {
				km.mappings[key] = mappings
			}
		}
		return nil
	}

	// 기존 형식 (호환성 유지)
	var oldMappings []*KeyMapping
	if err := json.Unmarshal(data, &oldMappings); err != nil {
		return fmt.Errorf("설정 파일 파싱 실패: %v", err)
	}

	km.mutex.Lock()
	defer km.mutex.Unlock()

	km.mappings = make(map[string][]*KeyMapping)
	for _, mapping := range oldMappings {
		if km.isValidStartKey(mapping.StartKey) {
			if km.mappings[mapping.StartKey] == nil {
				km.mappings[mapping.StartKey] = make([]*KeyMapping, 0)
			}
			km.mappings[mapping.StartKey] = append(km.mappings[mapping.StartKey], mapping)
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

// SaveConfig 설정 파일 저장 (중복키 지원)
func (km *KeyMappingManager) SaveConfig() error {
	// 새로운 형식으로 저장 (중복키 지원)
	data, err := json.MarshalIndent(km.mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("설정 직렬화 실패: %v", err)
	}

	if err := os.WriteFile(km.configFile, data, 0644); err != nil {
		return fmt.Errorf("설정 파일 저장 실패: %v", err)
	}

	return nil
}

// ParseKeySequence 문자열에서 키 시퀀스 파싱 (조합키 지원)
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

// parseKeyWithDelay 개별 키와 딜레이 파싱 (조합키 지원)
func (km *KeyMappingManager) parseKeyWithDelay(keyStr string) (string, int, error) {
	defaultDelay := 0

	if !strings.Contains(keyStr, "(") {
		// 딜레이가 없는 경우 - 조합키 검증
		key := strings.ToLower(strings.TrimSpace(keyStr))
		if km.isComboKey(key) {
			if err := km.validateComboKey(key); err != nil {
				return "", 0, fmt.Errorf("조합키 형식 오류: %v", err)
			}
		}
		return key, defaultDelay, nil
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

	// 조합키 검증
	key = strings.ToLower(key)
	if km.isComboKey(key) {
		if err := km.validateComboKey(key); err != nil {
			return "", 0, fmt.Errorf("조합키 형식 오류: %v", err)
		}
	}

	delay, err := strconv.Atoi(delayStr)
	if err != nil {
		return "", 0, fmt.Errorf("딜레이 파싱 실패: %s", delayStr)
	}

	if delay < 0 || delay > 1000 {
		return "", 0, fmt.Errorf("딜레이는 0~1000ms 사이여야 합니다: %d", delay)
	}

	return key, delay, nil
}

// FormatKeySequence 키 시퀀스를 문자열로 포맷팅 (조합키 지원)
func (km *KeyMappingManager) FormatKeySequence(keys []MappedKey) string {
	if len(keys) == 0 {
		return ""
	}

	parts := make([]string, len(keys))
	for i, key := range keys {
		keyDisplay := key.Key
		if km.isComboKey(key.Key) {
			keyDisplay = strings.ToUpper(key.Key)
		}

		if key.Delay == 0 {
			parts[i] = fmt.Sprintf("%s(즉시)", keyDisplay)
		} else {
			parts[i] = fmt.Sprintf("%s(%dms)", keyDisplay, key.Delay)
		}
	}

	return strings.Join(parts, ", ")
}

// IsValidKey 키가 유효한지 확인 (조합키 지원)
func (km *KeyMappingManager) IsValidKey(key string) bool {
	key = strings.ToLower(key)

	// 조합키인 경우
	if km.isComboKey(key) {
		return km.validateComboKey(key) == nil
	}

	// 일반 키인 경우
	availableKeys := km.GetAvailableKeys()
	for category, keyList := range availableKeys {
		if category == "시작 키" || category == "조합키 예시" {
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

// GetMappingStats 맵핑 통계 반환 (중복키 지원)
func (km *KeyMappingManager) GetMappingStats() map[string]interface{} {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	totalMappings := 0
	enabledMappings := 0
	disabledMappings := 0
	duplicateKeys := 0

	for _, mappings := range km.mappings {
		totalMappings += len(mappings)

		if len(mappings) > 1 {
			duplicateKeys++
		}

		for _, mapping := range mappings {
			if mapping.Enabled {
				enabledMappings++
			} else {
				disabledMappings++
			}
		}
	}

	return map[string]interface{}{
		"total":          totalMappings,
		"enabled":        enabledMappings,
		"disabled":       disabledMappings,
		"running":        km.running,
		"unique_keys":    len(km.mappings),
		"duplicate_keys": duplicateKeys,
		"os":             runtime.GOOS,
		"features": map[string]bool{
			"combo_keys":     true,
			"duplicate_keys": true,
			"auto_disable":   true,
		},
	}
}

// GetComboKeyExamples 조합키 예시 반환
func (km *KeyMappingManager) GetComboKeyExamples() map[string][]string {
	return map[string][]string{
		"복사/붙여넣기": {"ctrl+c", "ctrl+v", "ctrl+x", "ctrl+z", "ctrl+y"},
		"파일 작업":   {"ctrl+n", "ctrl+o", "ctrl+s", "ctrl+shift+s"},
		"편집":      {"ctrl+a", "ctrl+f", "ctrl+h", "ctrl+shift+f"},
		"창 관리":    {"alt+tab", "alt+f4", "win+d", "win+l", "win+r"},
		"시스템":     {"ctrl+alt+delete", "ctrl+shift+esc", "win+x"},
		"특수 기능":   {"shift+f10", "alt+enter", "ctrl+shift+n"},
	}
}

// ValidateComboKeySequence 조합키 시퀀스 전체 유효성 검사
func (km *KeyMappingManager) ValidateComboKeySequence(sequence string) error {
	keys, err := km.ParseKeySequence(sequence)
	if err != nil {
		return err
	}

	for i, key := range keys {
		if km.isComboKey(key.Key) {
			if err := km.validateComboKey(key.Key); err != nil {
				return fmt.Errorf("키 %d의 조합키 오류: %v", i+1, err)
			}
		}
	}

	return nil
}

// GetKeyDescription 키 설명 반환
func (km *KeyMappingManager) GetKeyDescription(key string) string {
	key = strings.ToLower(key)

	descriptions := map[string]string{
		// 일반 키
		"space":     "스페이스바",
		"enter":     "엔터",
		"esc":       "ESC",
		"tab":       "탭",
		"backspace": "백스페이스",
		"delete":    "Delete",
		"home":      "Home",
		"end":       "End",
		"pageup":    "Page Up",
		"pagedown":  "Page Down",
		"insert":    "Insert",

		// 화살표 키
		"left":  "왼쪽 화살표",
		"right": "오른쪽 화살표",
		"up":    "위쪽 화살표",
		"down":  "아래쪽 화살표",

		// 수정키
		"shift": "Shift",
		"ctrl":  "Ctrl",
		"alt":   "Alt",
		"cmd":   "Cmd",
		"win":   "Windows",
	}

	if desc, exists := descriptions[key]; exists {
		return desc
	}

	// 조합키인 경우
	if km.isComboKey(key) {
		return km.formatComboKeyDescription(key)
	}

	return strings.ToUpper(key)
}

// formatComboKeyDescription 조합키 설명 포맷팅
func (km *KeyMappingManager) formatComboKeyDescription(comboKey string) string {
	parts := strings.Split(comboKey, "+")
	descriptions := make([]string, len(parts))

	for i, part := range parts {
		descriptions[i] = km.GetKeyDescription(part)
	}

	return strings.Join(descriptions, " + ")
}
