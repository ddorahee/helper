// keymapping/manager.go - 중복키 허용 및 조합키 지원
package keymapping

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

// KeyMapping 구조체 - 개별 키 맵핑 설정
type KeyMapping struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	StartKey  string      `json:"start_key"`
	Keys      []MappedKey `json:"keys"`
	Enabled   bool        `json:"enabled"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// MappedKey 구조체 - 개별 키와 딜레이 설정 (조합키 지원)
type MappedKey struct {
	Key   string `json:"key"`   // 기본 키 (예: "x", "alt+x", "ctrl+shift+a")
	Delay int    `json:"delay"` // 딜레이 (ms)
}

// KeyMappingManager 구조체 - 키 맵핑 관리자
type KeyMappingManager struct {
	mappings    map[string][]*KeyMapping // 시작키별로 여러 맵핑 저장
	configFile  string
	mutex       sync.RWMutex
	running     bool
	stopChan    chan bool
	hookRunning bool
}

// NewKeyMappingManager 새로운 키 맵핑 매니저 생성
func NewKeyMappingManager(configDir string) *KeyMappingManager {
	configFile := filepath.Join(configDir, "keymappings.json")

	km := &KeyMappingManager{
		mappings:   make(map[string][]*KeyMapping),
		configFile: configFile,
		stopChan:   make(chan bool),
	}

	km.LoadConfig()
	return km
}

// Start 키 훅 시작
func (km *KeyMappingManager) Start() error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if km.running {
		return fmt.Errorf("키 맵핑이 이미 실행 중입니다")
	}

	km.running = true
	km.hookRunning = true

	go km.runKeyHook()

	log.Println("키 맵핑 시스템이 시작되었습니다 (중복키 허용, 조합키 지원)")
	return nil
}

// Stop 키 훅 중지
func (km *KeyMappingManager) Stop() {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if !km.running {
		return
	}

	km.running = false
	km.hookRunning = false

	select {
	case km.stopChan <- true:
	default:
	}

	hook.End()

	log.Println("키 맵핑 시스템이 중지되었습니다")
}

// IsRunning 실행 상태 확인
func (km *KeyMappingManager) IsRunning() bool {
	km.mutex.RLock()
	defer km.mutex.RUnlock()
	return km.running
}

// runKeyHook 키 훅 실행
func (km *KeyMappingManager) runKeyHook() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 훅 실행 중 패닉: %v", r)
		}
		km.mutex.Lock()
		km.running = false
		km.hookRunning = false
		km.mutex.Unlock()
	}()

	evChan := hook.Start()
	defer hook.End()

	for {
		select {
		case <-km.stopChan:
			return
		case ev := <-evChan:
			// KeyDown 이벤트만 처리하여 반응속도 향상
			if ev.Kind == hook.KeyDown {
				// 고루틴으로 처리하여 키 훅 블로킹 방지
				go km.handleKeyPress(ev)
			}
		}
	}
}

// handleKeyPress 키 입력 처리 (중복키 허용)
func (km *KeyMappingManager) handleKeyPress(ev hook.Event) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if !km.hookRunning {
		return
	}

	// 원시 키 코드 사용
	rawKeycode := ev.Rawcode

	// 허용된 원시 키 코드인지 확인 (빠른 체크)
	var keyStr string
	switch rawKeycode {
	case 46: // Delete 키
		keyStr = "delete"
	case 35: // End 키
		keyStr = "end"
	default:
		return // 지원하지 않는 키는 즉시 반환
	}

	// 해당 키로 시작하는 맵핑들 찾기 (중복 허용)
	mappings, exists := km.mappings[keyStr]
	if !exists || len(mappings) == 0 {
		return
	}

	// 활성화된 맵핑 중 하나만 실행 (첫 번째 활성화된 맵핑)
	for _, mapping := range mappings {
		if mapping.Enabled {
			log.Printf("키 맵핑 실행: %s (시작키: %s)", mapping.Name, keyStr)
			// 비동기로 실행하여 키 훅 블로킹 방지
			go km.executeKeySequence(mapping)
			break // 첫 번째 활성화된 맵핑만 실행
		}
	}
}

// executeKeySequence 키 시퀀스 실행 (조합키 지원)
func (km *KeyMappingManager) executeKeySequence(mapping *KeyMapping) {
	for i, key := range mapping.Keys {
		if !km.IsRunning() {
			break
		}

		if err := km.sendKey(key.Key); err != nil {
			log.Printf("키 입력 실패 (%s): %v", key.Key, err)
			continue
		}

		// 딜레이 적용 최적화
		if i < len(mapping.Keys)-1 {
			if key.Delay == 0 {
				// 딜레이가 0이면 최소 딜레이만 적용
				time.Sleep(10 * time.Millisecond)
			} else if key.Delay < 50 {
				// 50ms 미만이면 50ms로 조정 (시스템 안정성)
				time.Sleep(50 * time.Millisecond)
			} else {
				// 지정된 딜레이 적용
				time.Sleep(time.Duration(key.Delay) * time.Millisecond)
			}
		}

		if !km.IsRunning() {
			break
		}
	}
}

// sendKey 키 입력 전송 (조합키 지원)
func (km *KeyMappingManager) sendKey(key string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 입력 중 패닉 복구: %v", r)
		}
	}()

	// 키 입력 전 딜레이를 최소화 (50ms -> 10ms)
	time.Sleep(10 * time.Millisecond)

	// 조합키 처리
	if km.isComboKey(key) {
		return km.sendComboKey(key)
	}

	// 일반 키 처리 (속도 최적화)
	robotgo.KeyTap(key)
	log.Printf("키 입력: %s", key)
	return nil
}

// isComboKey 조합키인지 확인
func (km *KeyMappingManager) isComboKey(key string) bool {
	return len(key) > 1 && (contains(key, "ctrl+") ||
		contains(key, "shift+") ||
		contains(key, "alt+") ||
		contains(key, "cmd+") ||
		contains(key, "win+"))
}

// sendComboKey 조합키 전송
func (km *KeyMappingManager) sendComboKey(comboKey string) error {
	// 조합키 파싱
	modifiers, mainKey := km.parseComboKey(comboKey)

	if mainKey == "" {
		return fmt.Errorf("잘못된 조합키 형식: %s", comboKey)
	}

	// 수정키들을 interface{} 슬라이스로 변환
	modifierInterfaces := make([]interface{}, len(modifiers))
	for i, modifier := range modifiers {
		modifierInterfaces[i] = modifier
	}

	// 조합키 입력 (딜레이 없이 즉시 실행)
	robotgo.KeyTap(mainKey, modifierInterfaces...)
	log.Printf("조합키 입력: %s (수정키: %v, 메인키: %s)", comboKey, modifiers, mainKey)

	return nil
}

// parseComboKey 조합키를 파싱하여 수정키와 메인키로 분리
func (km *KeyMappingManager) parseComboKey(comboKey string) ([]string, string) {
	var modifiers []string
	var mainKey string

	// 소문자로 변환하여 처리
	key := strings.ToLower(comboKey)

	// 수정키 확인 및 제거
	if strings.Contains(key, "ctrl+") {
		modifiers = append(modifiers, "ctrl")
		key = strings.Replace(key, "ctrl+", "", 1)
	}
	if strings.Contains(key, "shift+") {
		modifiers = append(modifiers, "shift")
		key = strings.Replace(key, "shift+", "", 1)
	}
	if strings.Contains(key, "alt+") {
		modifiers = append(modifiers, "alt")
		key = strings.Replace(key, "alt+", "", 1)
	}
	if strings.Contains(key, "cmd+") {
		modifiers = append(modifiers, "cmd")
		key = strings.Replace(key, "cmd+", "", 1)
	}
	if strings.Contains(key, "win+") {
		modifiers = append(modifiers, "cmd") // Windows 키는 cmd로 매핑
		key = strings.Replace(key, "win+", "", 1)
	}

	mainKey = strings.TrimSpace(key)
	return modifiers, mainKey
}

// validateKeys 키 유효성 검사 (조합키 지원)
func (km *KeyMappingManager) validateKeys(startKey string, keys []MappedKey) error {
	if startKey == "" {
		return fmt.Errorf("시작 키가 비어있습니다")
	}

	// 시작키는 DELETE 또는 END만 허용
	if !km.isValidStartKey(startKey) {
		return fmt.Errorf("시작 키는 'delete' 또는 'end'만 사용할 수 있습니다")
	}

	if len(keys) == 0 {
		return fmt.Errorf("실행할 키가 없습니다")
	}

	for i, key := range keys {
		if key.Key == "" {
			return fmt.Errorf("키 %d가 비어있습니다", i+1)
		}

		// 조합키 유효성 검사
		if km.isComboKey(key.Key) {
			if err := km.validateComboKey(key.Key); err != nil {
				return fmt.Errorf("키 %d의 조합키 형식이 잘못됨: %v", i+1, err)
			}
		}

		if key.Delay < 0 || key.Delay > 1000 {
			return fmt.Errorf("키 %d의 딜레이는 0ms~1000ms 사이여야 합니다", i+1)
		}
	}

	return nil
}

// validateComboKey 조합키 유효성 검사
func (km *KeyMappingManager) validateComboKey(comboKey string) error {
	key := strings.ToLower(comboKey)

	// 지원되는 수정키들
	validModifiers := []string{"ctrl+", "shift+", "alt+", "cmd+", "win+"}

	// 적어도 하나의 수정키가 있어야 함
	hasModifier := false
	for _, modifier := range validModifiers {
		if strings.Contains(key, modifier) {
			hasModifier = true
			key = strings.Replace(key, modifier, "", 1)
		}
	}

	if !hasModifier {
		return fmt.Errorf("조합키에는 최소 하나의 수정키가 필요합니다")
	}

	// 메인키가 남아있어야 함
	mainKey := strings.TrimSpace(key)
	if mainKey == "" {
		return fmt.Errorf("조합키에는 메인키가 필요합니다")
	}

	return nil
}

// CRUD 메서드들 (중복키 허용으로 수정)
func (km *KeyMappingManager) AddMapping(name, startKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if err := km.validateKeys(startKey, keys); err != nil {
		return err
	}

	// 중복키 허용 - 같은 시작키를 가진 맵핑이 여러 개 있을 수 있음
	// 단, 이미 활성화된 맵핑이 있으면 새 맵핑은 비활성화 상태로 생성
	existingMappings := km.mappings[startKey]
	hasActiveMapping := false
	for _, existing := range existingMappings {
		if existing.Enabled {
			hasActiveMapping = true
			break
		}
	}

	mapping := &KeyMapping{
		ID:        fmt.Sprintf("%s_%d", name, time.Now().Unix()),
		Name:      name,
		StartKey:  startKey,
		Keys:      keys,
		Enabled:   !hasActiveMapping, // 활성화된 맵핑이 없으면 활성화, 있으면 비활성화
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 맵핑 추가
	if km.mappings[startKey] == nil {
		km.mappings[startKey] = make([]*KeyMapping, 0)
	}
	km.mappings[startKey] = append(km.mappings[startKey], mapping)

	if err := km.SaveConfig(); err != nil {
		// 실패 시 롤백
		mappings := km.mappings[startKey]
		km.mappings[startKey] = mappings[:len(mappings)-1]
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 추가: %s (시작키: %s, 활성화: %t)", name, startKey, mapping.Enabled)
	if hasActiveMapping {
		log.Printf("경고: 시작키 '%s'에 이미 활성화된 맵핑이 있어 새 맵핑은 비활성화됨", startKey)
	}

	return nil
}

func (km *KeyMappingManager) UpdateMapping(oldStartKey, newName, newStartKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	// 기존 맵핑 찾기
	oldMappings := km.mappings[oldStartKey]
	if len(oldMappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", oldStartKey)
	}

	// 첫 번째 맵핑을 수정 대상으로 함 (실제로는 ID로 구분해야 함)
	mapping := oldMappings[0]

	if err := km.validateKeys(newStartKey, keys); err != nil {
		return err
	}

	// 시작키가 변경된 경우 처리
	if oldStartKey != newStartKey {
		// 기존 위치에서 제거
		km.mappings[oldStartKey] = oldMappings[1:]
		if len(km.mappings[oldStartKey]) == 0 {
			delete(km.mappings, oldStartKey)
		}

		// 새 위치에 추가
		if km.mappings[newStartKey] == nil {
			km.mappings[newStartKey] = make([]*KeyMapping, 0)
		}
		km.mappings[newStartKey] = append(km.mappings[newStartKey], mapping)
	}

	// 맵핑 정보 업데이트
	mapping.Name = newName
	mapping.StartKey = newStartKey
	mapping.Keys = keys
	mapping.UpdatedAt = time.Now()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 수정: %s (시작키: %s -> %s)", newName, oldStartKey, newStartKey)
	return nil
}

func (km *KeyMappingManager) RemoveMapping(startKey string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	mappings := km.mappings[startKey]
	if len(mappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", startKey)
	}

	// 첫 번째 맵핑 제거 (실제로는 ID로 구분해야 함)
	mapping := mappings[0]
	km.mappings[startKey] = mappings[1:]

	// 맵핑이 모두 제거되면 키 자체를 삭제
	if len(km.mappings[startKey]) == 0 {
		delete(km.mappings, startKey)
	}

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 제거: %s (시작키: %s)", mapping.Name, startKey)
	return nil
}

func (km *KeyMappingManager) ToggleMapping(startKey string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	mappings := km.mappings[startKey]
	if len(mappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", startKey)
	}

	// 첫 번째 맵핑 토글
	mapping := mappings[0]

	// 활성화하려는 경우, 같은 시작키의 다른 맵핑들을 비활성화
	if !mapping.Enabled {
		for _, other := range mappings {
			if other != mapping {
				other.Enabled = false
			}
		}
	}

	mapping.Enabled = !mapping.Enabled
	mapping.UpdatedAt = time.Now()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	status := "비활성화"
	if mapping.Enabled {
		status = "활성화"
	}

	log.Printf("키 맵핑 %s: %s (시작키: %s)", status, mapping.Name, startKey)
	return nil
}

// ToggleMappingByID ID로 특정 맵핑 토글 (새로 추가)
func (km *KeyMappingManager) ToggleMappingByID(mappingID string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	// 모든 맵핑에서 ID로 찾기
	var targetMapping *KeyMapping
	var targetStartKey string

	for startKey, mappings := range km.mappings {
		for _, mapping := range mappings {
			if mapping.ID == mappingID {
				targetMapping = mapping
				targetStartKey = startKey
				break
			}
		}
		if targetMapping != nil {
			break
		}
	}

	if targetMapping == nil {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: ID %s", mappingID)
	}

	// 활성화하려는 경우, 같은 시작키의 다른 맵핑들을 비활성화
	if !targetMapping.Enabled {
		for _, other := range km.mappings[targetStartKey] {
			if other != targetMapping {
				other.Enabled = false
			}
		}
	}

	targetMapping.Enabled = !targetMapping.Enabled
	targetMapping.UpdatedAt = time.Now()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	status := "비활성화"
	if targetMapping.Enabled {
		status = "활성화"
	}

	log.Printf("키 맵핑 %s: %s (ID: %s, 시작키: %s)", status, targetMapping.Name, mappingID, targetStartKey)
	return nil
}

// RemoveMappingByID ID로 특정 맵핑 제거 (새로 추가)
func (km *KeyMappingManager) RemoveMappingByID(mappingID string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	// 모든 맵핑에서 ID로 찾기
	var targetMapping *KeyMapping
	var targetStartKey string
	var targetIndex int

	for startKey, mappings := range km.mappings {
		for i, mapping := range mappings {
			if mapping.ID == mappingID {
				targetMapping = mapping
				targetStartKey = startKey
				targetIndex = i
				break
			}
		}
		if targetMapping != nil {
			break
		}
	}

	if targetMapping == nil {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: ID %s", mappingID)
	}

	// 맵핑 제거
	mappings := km.mappings[targetStartKey]
	km.mappings[targetStartKey] = append(mappings[:targetIndex], mappings[targetIndex+1:]...)

	// 맵핑이 모두 제거되면 키 자체를 삭제
	if len(km.mappings[targetStartKey]) == 0 {
		delete(km.mappings, targetStartKey)
	}

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 제거: %s (ID: %s, 시작키: %s)", targetMapping.Name, mappingID, targetStartKey)
	return nil
}

func (km *KeyMappingManager) GetMappings() map[string]*KeyMapping {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	result := make(map[string]*KeyMapping)

	// 각 시작키별로 첫 번째 맵핑만 반환 (호환성을 위해)
	for startKey, mappings := range km.mappings {
		if len(mappings) > 0 {
			result[startKey] = mappings[0]
		}
	}

	return result
}

// GetAllMappings 모든 맵핑을 시작키별로 그룹화하여 반환
func (km *KeyMappingManager) GetAllMappings() map[string][]*KeyMapping {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	result := make(map[string][]*KeyMapping)
	for k, v := range km.mappings {
		result[k] = make([]*KeyMapping, len(v))
		copy(result[k], v)
	}

	return result
}

func (km *KeyMappingManager) GetMapping(startKey string) (*KeyMapping, bool) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	mappings := km.mappings[startKey]
	if len(mappings) == 0 {
		return nil, false
	}

	return mappings[0], true
}

// contains 문자열 포함 여부 확인 헬퍼 함수
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					strings.Contains(s, substr))))
}
