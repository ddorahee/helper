// keymapping/manager.go - 불필요한 딜레이 제거로 초고속 키 맵핑 시스템
package keymapping

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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

// MappedKey 구조체 - 개별 키와 딜레이 설정
type MappedKey struct {
	Key   string `json:"key"`   // 기본 키 (예: "x", "alt+x", "ctrl+shift+a")
	Delay int    `json:"delay"` // 딜레이 (ms) - 사용자 설정만 사용
}

// KeyMappingManager 구조체 - 초고속 키 상태 추적
type KeyMappingManager struct {
	// 핵심 데이터 (읽기 최적화)
	mappings   map[string][]*KeyMapping
	activeKeys map[uint16]*KeyMapping // 원시 키 코드 -> 활성 맵핑 (빠른 조회)

	// 동기화 및 상태 관리
	configFile string
	mutex      sync.RWMutex
	running    int32 // atomic 사용으로 빠른 상태 체크
	stopChan   chan struct{}

	// 성능 최적화 필드
	keyPool     sync.Pool // 메모리 재사용
	lastTrigger int64     // 마지막 트리거 시간 (중복 방지)

	// 키 상태 추적 (중복 입력 방지)
	keyStates  map[uint16]bool // 키 눌림 상태 추적
	stateMutex sync.Mutex      // 키 상태 뮤텍스

	// 허용된 키 코드 맵 (상수)
	allowedKeys map[uint16]string
}

// NewKeyMappingManager 새로운 초고속 키 맵핑 매니저 생성
func NewKeyMappingManager(configDir string) *KeyMappingManager {
	configFile := filepath.Join(configDir, "keymappings.json")

	km := &KeyMappingManager{
		mappings:   make(map[string][]*KeyMapping),
		activeKeys: make(map[uint16]*KeyMapping),
		configFile: configFile,
		stopChan:   make(chan struct{}),
		keyStates:  make(map[uint16]bool),
		allowedKeys: map[uint16]string{
			46: "delete", // Delete 키
			35: "end",    // End 키
		},
		keyPool: sync.Pool{
			New: func() interface{} {
				return make([]interface{}, 0, 4) // 조합키용 슬라이스 풀
			},
		},
	}

	km.LoadConfig()
	km.rebuildActiveKeys() // 활성 키 맵 구축
	return km
}

// rebuildActiveKeys 활성 키 맵을 다시 구축 (성능 최적화)
func (km *KeyMappingManager) rebuildActiveKeys() {
	km.activeKeys = make(map[uint16]*KeyMapping)

	for startKey, mappings := range km.mappings {
		for _, mapping := range mappings {
			if mapping.Enabled {
				if keyCode := km.stringToRawKeyCode(startKey); keyCode != 0 {
					km.activeKeys[keyCode] = mapping
				}
				break // 첫 번째 활성 맵핑만 사용
			}
		}
	}

	log.Printf("활성 키 맵 재구축: %d개 키", len(km.activeKeys))
}

// Start 키 훅 시작 (성능 최적화)
func (km *KeyMappingManager) Start() error {
	if !atomic.CompareAndSwapInt32(&km.running, 0, 1) {
		return fmt.Errorf("키 맵핑이 이미 실행 중입니다")
	}

	km.stopChan = make(chan struct{})
	go km.runUltraFastKeyHook()

	log.Println("초고속 키 맵핑 시스템 시작 (딜레이 제거)")
	return nil
}

// Stop 키 훅 중지
func (km *KeyMappingManager) Stop() {
	if !atomic.CompareAndSwapInt32(&km.running, 1, 0) {
		return
	}

	close(km.stopChan)
	hook.End()

	log.Println("초고속 키 맵핑 시스템 중지")
}

// IsRunning 실행 상태 확인 (atomic 사용으로 빠른 체크)
func (km *KeyMappingManager) IsRunning() bool {
	return atomic.LoadInt32(&km.running) == 1
}

// runUltraFastKeyHook 딜레이 제거된 초고속 키 훅 실행
func (km *KeyMappingManager) runUltraFastKeyHook() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 훅 패닉 복구: %v", r)
		}
		atomic.StoreInt32(&km.running, 0)
	}()

	evChan := hook.Start()
	defer hook.End()

	for {
		select {
		case <-km.stopChan:
			return
		case ev := <-evChan:
			// KeyDown과 KeyUp 모두 처리
			if ev.Kind == hook.KeyDown {
				km.handleKeyDown(ev.Rawcode)
			} else if ev.Kind == hook.KeyUp {
				km.handleKeyUp(ev.Rawcode)
			}
		}
	}
}

// handleKeyDown 키 눌림 처리 (딜레이 제거로 최대 속도)
func (km *KeyMappingManager) handleKeyDown(rawKeycode uint16) {
	// 1. 빠른 상태 체크
	if atomic.LoadInt32(&km.running) == 0 {
		return
	}

	// 2. 키 상태 확인 (이미 눌린 키는 무시)
	km.stateMutex.Lock()
	if km.keyStates[rawKeycode] {
		km.stateMutex.Unlock()
		return // 이미 눌린 키는 무시
	}
	km.keyStates[rawKeycode] = true // 키 눌림 상태로 설정
	km.stateMutex.Unlock()

	// 3. 중복 방지 시간 최소화 (10ms)
	now := time.Now().UnixNano()
	if now-atomic.LoadInt64(&km.lastTrigger) < 10000000 { // 10ms
		return
	}
	atomic.StoreInt64(&km.lastTrigger, now)

	// 4. 활성 키 맵에서 빠른 조회
	km.mutex.RLock()
	mapping, exists := km.activeKeys[rawKeycode]
	km.mutex.RUnlock()

	if !exists {
		return
	}

	// 5. 동기 실행 (순서 보장) - 딜레이 제거
	log.Printf("키 맵핑 시작: %s", mapping.Name)
	km.executeInstantKeySequence(mapping)

	// 6. 실행 완료 후 즉시 키 상태 초기화 (다음 실행 허용)
	km.stateMutex.Lock()
	km.keyStates[rawKeycode] = false
	km.stateMutex.Unlock()

	// 7. 다음 실행을 위한 최소 대기 (즉시 가능)
	atomic.StoreInt64(&km.lastTrigger, 0) // 트리거 시간 즉시 초기화
}

// handleKeyUp 키 놓음 처리 (상태 초기화)
func (km *KeyMappingManager) handleKeyUp(rawKeycode uint16) {
	// 키 놓음 상태로 설정
	km.stateMutex.Lock()
	km.keyStates[rawKeycode] = false
	km.stateMutex.Unlock()
}

// executeInstantKeySequence 딜레이 제거된 초고속 키 시퀀스 실행
func (km *KeyMappingManager) executeInstantKeySequence(mapping *KeyMapping) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 시퀀스 실행 중 패닉: %v", r)
		}
		log.Printf("키 맵핑 완료: %s", mapping.Name)
	}()

	log.Printf("키 시퀀스 실행: %s (%d개 키)", mapping.Name, len(mapping.Keys))

	// 시작 딜레이 제거 - 즉시 실행

	for i, key := range mapping.Keys {
		if atomic.LoadInt32(&km.running) == 0 {
			return
		}

		// 키 입력 실행 (딜레이 제거)
		if err := km.sendInstantKey(key.Key); err != nil {
			log.Printf("키 입력 실패: %v", err)
			continue
		}

		// 키 간격 처리 - 사용자 설정 딜레이만 적용
		if i < len(mapping.Keys)-1 {
			if key.Delay > 0 {
				// 사용자 설정 딜레이만 적용
				time.Sleep(time.Duration(key.Delay) * time.Millisecond)
			}
			// 기본 간격 딜레이 제거 - 즉시 다음 키 실행
		}

		// 계속 실행 중인지 확인
		if !km.IsRunning() {
			break
		}
	}
}

// sendInstantKey 딜레이 제거된 초고속 키 입력 전송
func (km *KeyMappingManager) sendInstantKey(key string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 입력 패닉 복구: %v", r)
		}
	}()

	// 키 입력 전 딜레이 제거 - 즉시 실행

	if km.isComboKey(key) {
		return km.sendInstantComboKey(key)
	}

	// 일반 키 처리 - 즉시 실행
	robotgo.KeyTap(key)
	return nil
}

// sendInstantComboKey 딜레이 제거된 초고속 조합키 전송
func (km *KeyMappingManager) sendInstantComboKey(comboKey string) error {
	modifiers, mainKey := km.parseComboKey(comboKey)
	if mainKey == "" {
		return fmt.Errorf("잘못된 조합키: %s", comboKey)
	}

	// 조합키 입력 전 딜레이 제거 - 즉시 실행

	// 메모리 풀 사용으로 GC 압박 감소
	modifierSlice := km.keyPool.Get().([]interface{})
	modifierSlice = modifierSlice[:0] // 슬라이스 재사용

	for _, modifier := range modifiers {
		modifierSlice = append(modifierSlice, modifier)
	}

	// 조합키 즉시 입력
	robotgo.KeyTap(mainKey, modifierSlice...)

	// 메모리 풀 반환
	km.keyPool.Put(modifierSlice)
	return nil
}

// AddMapping 키 맵핑 추가 (캐시 업데이트 포함)
func (km *KeyMappingManager) AddMapping(name, startKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if err := km.validateKeys(startKey, keys); err != nil {
		return err
	}

	// 기존 맵핑 확인
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
		Enabled:   !hasActiveMapping,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 맵핑 추가
	if km.mappings[startKey] == nil {
		km.mappings[startKey] = make([]*KeyMapping, 0)
	}
	km.mappings[startKey] = append(km.mappings[startKey], mapping)

	// 활성 키 맵 업데이트
	if mapping.Enabled {
		if keyCode := km.stringToRawKeyCode(startKey); keyCode != 0 {
			km.activeKeys[keyCode] = mapping
		}
	}

	if err := km.SaveConfig(); err != nil {
		// 실패 시 롤백
		mappings := km.mappings[startKey]
		km.mappings[startKey] = mappings[:len(mappings)-1]
		km.rebuildActiveKeys()
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("초고속 키 맵핑 추가: %s (딜레이 제거)", name)
	return nil
}

// RemoveMapping 키 맵핑 제거 (캐시 업데이트 포함)
func (km *KeyMappingManager) RemoveMapping(startKey string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	mappings := km.mappings[startKey]
	if len(mappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", startKey)
	}

	// 첫 번째 맵핑 제거
	mapping := mappings[0]
	km.mappings[startKey] = mappings[1:]

	if len(km.mappings[startKey]) == 0 {
		delete(km.mappings, startKey)
	}

	// 활성 키 맵 업데이트
	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("초고속 키 맵핑 제거: %s", mapping.Name)
	return nil
}

// ToggleMapping 키 맵핑 토글 (캐시 업데이트 포함)
func (km *KeyMappingManager) ToggleMapping(startKey string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	mappings := km.mappings[startKey]
	if len(mappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", startKey)
	}

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

	// 활성 키 맵 업데이트
	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	status := "비활성화"
	if mapping.Enabled {
		status = "활성화"
	}

	log.Printf("초고속 키 맵핑 %s: %s", status, mapping.Name)
	return nil
}

func (km *KeyMappingManager) ToggleMappingByID(mappingID string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

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
				other.UpdatedAt = time.Now()
			}
		}
	}

	targetMapping.Enabled = !targetMapping.Enabled
	targetMapping.UpdatedAt = time.Now()

	// 활성 키 맵 업데이트
	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	status := "비활성화"
	if targetMapping.Enabled {
		status = "활성화"
	}

	log.Printf("ID 기반 초고속 키 맵핑 %s: %s (ID: %s)", status, targetMapping.Name, mappingID)
	return nil
}

// UpdateMapping 키 맵핑 수정
func (km *KeyMappingManager) UpdateMapping(oldStartKey, newName, newStartKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	// 기존 맵핑 찾기
	oldMappings := km.mappings[oldStartKey]
	if len(oldMappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", oldStartKey)
	}

	// 첫 번째 맵핑을 수정 대상으로 함
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

	// 활성 키 맵 업데이트
	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("초고속 키 맵핑 수정: %s", newName)
	return nil
}

// 유틸리티 메서드들

// isComboKey 조합키인지 확인
func (km *KeyMappingManager) isComboKey(key string) bool {
	return strings.Contains(key, "+")
}

// parseComboKey 조합키 파싱
func (km *KeyMappingManager) parseComboKey(comboKey string) ([]string, string) {
	var modifiers []string
	key := strings.ToLower(comboKey)

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
		modifiers = append(modifiers, "cmd")
		key = strings.Replace(key, "win+", "", 1)
	}

	return modifiers, strings.TrimSpace(key)
}

// stringToRawKeyCode 문자열을 원시 키 코드로 변환
func (km *KeyMappingManager) stringToRawKeyCode(keyStr string) uint16 {
	keyStr = strings.ToLower(keyStr)

	for keyCode, keyName := range km.allowedKeys {
		if keyName == keyStr {
			return keyCode
		}
	}

	return 0
}

// validateKeys 키 유효성 검사
func (km *KeyMappingManager) validateKeys(startKey string, keys []MappedKey) error {
	if startKey == "" {
		return fmt.Errorf("시작 키가 비어있습니다")
	}

	if !km.isValidStartKey(startKey) {
		return fmt.Errorf("시작 키는 'delete' 또는 'end'만 사용할 수 있습니다")
	}

	if len(keys) == 0 {
		return fmt.Errorf("실행할 키가 없습니다")
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

// isValidStartKey 시작키 유효성 검사
func (km *KeyMappingManager) isValidStartKey(key string) bool {
	return km.stringToRawKeyCode(key) != 0
}

// 기존 메서드들 (수정 없음)
func (km *KeyMappingManager) GetMappings() map[string]*KeyMapping {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	result := make(map[string]*KeyMapping)
	for startKey, mappings := range km.mappings {
		if len(mappings) > 0 {
			result[startKey] = mappings[0]
		}
	}
	return result
}

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

func (km *KeyMappingManager) UpdateMappingByID(mappingID, newName, newStartKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	// ID로 맵핑 찾기
	var targetMapping *KeyMapping
	var oldStartKey string

	for startKey, mappings := range km.mappings {
		for _, mapping := range mappings {
			if mapping.ID == mappingID {
				targetMapping = mapping
				oldStartKey = startKey
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

	if err := km.validateKeys(newStartKey, keys); err != nil {
		return err
	}

	// 시작키가 변경된 경우 처리
	if oldStartKey != newStartKey {
		// 기존 위치에서 제거
		oldMappings := km.mappings[oldStartKey]
		for i, mapping := range oldMappings {
			if mapping.ID == mappingID {
				km.mappings[oldStartKey] = append(oldMappings[:i], oldMappings[i+1:]...)
				break
			}
		}

		// 기존 시작키에 맵핑이 없으면 삭제
		if len(km.mappings[oldStartKey]) == 0 {
			delete(km.mappings, oldStartKey)
		}

		// 새 위치에 추가
		if km.mappings[newStartKey] == nil {
			km.mappings[newStartKey] = make([]*KeyMapping, 0)
		}
		km.mappings[newStartKey] = append(km.mappings[newStartKey], targetMapping)
	}

	// 맵핑 정보 업데이트
	targetMapping.Name = newName
	targetMapping.StartKey = newStartKey
	targetMapping.Keys = keys
	targetMapping.UpdatedAt = time.Now()

	// 활성 키 맵 업데이트
	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("ID 기반 초고속 키 맵핑 수정: %s (ID: %s)", newName, mappingID)
	return nil
}

// GetMappingByID ID로 특정 맵핑 조회
func (km *KeyMappingManager) GetMappingByID(mappingID string) (*KeyMapping, bool) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	for _, mappings := range km.mappings {
		for _, mapping := range mappings {
			if mapping.ID == mappingID {
				return mapping, true
			}
		}
	}
	return nil, false
}
