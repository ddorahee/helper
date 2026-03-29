package keymapping

import (
	"fmt"
	"log"
	"math/rand"
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
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	StartKey       string      `json:"start_key"`
	Keys           []MappedKey `json:"keys"`
	Enabled        bool        `json:"enabled"`
	RandomDelay    bool        `json:"random_delay"`
	RandomDelayMin int         `json:"random_delay_min"` // ms
	RandomDelayMax int         `json:"random_delay_max"` // ms
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
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

	// 허용된 키 코드 맵
	allowedKeys map[uint16]string
}

// NewKeyMappingManager 새로운 키 맵핑 매니저 생성
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
			36: "home",   // Home 키
		},
		keyPool: sync.Pool{
			New: func() interface{} {
				return make([]interface{}, 0, 4)
			},
		},
	}

	km.LoadConfig()
	km.rebuildActiveKeys()
	return km
}

// rebuildActiveKeys 활성 키 맵을 다시 구축
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

// Start 키 훅 시작
func (km *KeyMappingManager) Start() error {
	if !atomic.CompareAndSwapInt32(&km.running, 0, 1) {
		return fmt.Errorf("키 맵핑이 이미 실행 중입니다")
	}

	km.stopChan = make(chan struct{})
	go km.runKeyHook()

	log.Println("키 맵핑 시스템 시작")
	return nil
}

// Stop 키 훅 중지
func (km *KeyMappingManager) Stop() {
	if !atomic.CompareAndSwapInt32(&km.running, 1, 0) {
		return
	}

	close(km.stopChan)
	hook.End()

	log.Println("키 맵핑 시스템 중지")
}

// IsRunning 실행 상태 확인
func (km *KeyMappingManager) IsRunning() bool {
	return atomic.LoadInt32(&km.running) == 1
}

// runKeyHook gohook 기반 키 훅 실행
func (km *KeyMappingManager) runKeyHook() {
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
			if ev.Kind == hook.KeyDown {
				km.handleKeyDown(ev.Rawcode)
			} else if ev.Kind == hook.KeyUp {
				km.handleKeyUp(ev.Rawcode)
			}
		}
	}
}

// handleKeyDown 키 눌림 처리
func (km *KeyMappingManager) handleKeyDown(rawKeycode uint16) {
	if atomic.LoadInt32(&km.running) == 0 {
		return
	}

	// 키 상태 확인 (이미 눌린 키는 무시)
	km.stateMutex.Lock()
	if km.keyStates[rawKeycode] {
		km.stateMutex.Unlock()
		return
	}
	km.keyStates[rawKeycode] = true
	km.stateMutex.Unlock()

	// 중복 방지 (10ms)
	now := time.Now().UnixNano()
	if now-atomic.LoadInt64(&km.lastTrigger) < 10000000 {
		return
	}
	atomic.StoreInt64(&km.lastTrigger, now)

	// 활성 키 맵에서 조회
	km.mutex.RLock()
	mapping, exists := km.activeKeys[rawKeycode]
	km.mutex.RUnlock()

	if !exists {
		return
	}

	log.Printf("키 맵핑 트리거: %s", mapping.Name)
	km.executeKeySequence(mapping)

	// 실행 완료 후 키 상태 초기화
	km.stateMutex.Lock()
	km.keyStates[rawKeycode] = false
	km.stateMutex.Unlock()

	atomic.StoreInt64(&km.lastTrigger, 0)
}

// handleKeyUp 키 놓음 처리
func (km *KeyMappingManager) handleKeyUp(rawKeycode uint16) {
	km.stateMutex.Lock()
	km.keyStates[rawKeycode] = false
	km.stateMutex.Unlock()
}

// executeKeySequence 키 시퀀스 실행
func (km *KeyMappingManager) executeKeySequence(mapping *KeyMapping) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 시퀀스 실행 중 패닉: %v", r)
		}
		log.Printf("키 맵핑 완료: %s", mapping.Name)
	}()

	for i, key := range mapping.Keys {
		if atomic.LoadInt32(&km.running) == 0 {
			return
		}

		km.sendKey(key.Key)

		// 딜레이 적용
		if i < len(mapping.Keys)-1 {
			if mapping.RandomDelay && mapping.RandomDelayMax > 0 {
				// 랜덤 딜레이: min~max ms
				minD := mapping.RandomDelayMin
				maxD := mapping.RandomDelayMax
				if minD > maxD {
					minD, maxD = maxD, minD
				}
				delay := minD + rand.Intn(maxD-minD+1)
				log.Printf("[키맵핑] '%s' → 랜덤 딜레이 %dms", key.Key, delay)
				time.Sleep(time.Duration(delay) * time.Millisecond)
			} else if key.Delay > 0 {
				// 고정 딜레이
				time.Sleep(time.Duration(key.Delay) * time.Millisecond)
			}
		}
	}
}

// sendKey 단일 키 전송
func (km *KeyMappingManager) sendKey(key string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 입력 패닉 복구: %v", r)
		}
	}()

	if km.isComboKey(key) {
		km.sendComboKey(key)
		return
	}

	robotgo.KeyTap(key)
}

// sendComboKey 조합키 전송
func (km *KeyMappingManager) sendComboKey(comboKey string) {
	modifiers, mainKey := km.parseComboKey(comboKey)
	if mainKey == "" {
		return
	}

	modifierSlice := km.keyPool.Get().([]interface{})
	modifierSlice = modifierSlice[:0]

	for _, modifier := range modifiers {
		modifierSlice = append(modifierSlice, modifier)
	}

	robotgo.KeyTap(mainKey, modifierSlice...)

	km.keyPool.Put(modifierSlice)
}

// AddMapping 키 맵핑 추가 (기존 호환)
func (km *KeyMappingManager) AddMapping(name, startKey string, keys []MappedKey) error {
	return km.AddMappingWithRandom(name, startKey, keys, false, 0, 0)
}

// AddMappingWithRandom 키 맵핑 추가 (랜덤 딜레이 포함)
func (km *KeyMappingManager) AddMappingWithRandom(name, startKey string, keys []MappedKey, randomDelay bool, randomMin, randomMax int) error {
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
		ID:             fmt.Sprintf("%s_%d", name, time.Now().Unix()),
		Name:           name,
		StartKey:       startKey,
		Keys:           keys,
		Enabled:        !hasActiveMapping,
		RandomDelay:    randomDelay,
		RandomDelayMin: randomMin,
		RandomDelayMax: randomMax,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if km.mappings[startKey] == nil {
		km.mappings[startKey] = make([]*KeyMapping, 0)
	}
	km.mappings[startKey] = append(km.mappings[startKey], mapping)

	if mapping.Enabled {
		if keyCode := km.stringToRawKeyCode(startKey); keyCode != 0 {
			km.activeKeys[keyCode] = mapping
		}
	}

	if err := km.SaveConfig(); err != nil {
		mappings := km.mappings[startKey]
		km.mappings[startKey] = mappings[:len(mappings)-1]
		km.rebuildActiveKeys()
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 추가: %s", name)
	return nil
}

// RemoveMapping 키 맵핑 제거
func (km *KeyMappingManager) RemoveMapping(startKey string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	mappings := km.mappings[startKey]
	if len(mappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", startKey)
	}

	mapping := mappings[0]
	km.mappings[startKey] = mappings[1:]

	if len(km.mappings[startKey]) == 0 {
		delete(km.mappings, startKey)
	}

	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 제거: %s", mapping.Name)
	return nil
}

// ToggleMapping 키 맵핑 토글
func (km *KeyMappingManager) ToggleMapping(startKey string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	mappings := km.mappings[startKey]
	if len(mappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", startKey)
	}

	mapping := mappings[0]

	if !mapping.Enabled {
		for _, other := range mappings {
			if other != mapping {
				other.Enabled = false
			}
		}
	}

	mapping.Enabled = !mapping.Enabled
	mapping.UpdatedAt = time.Now()

	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 토글: %s (enabled=%v)", mapping.Name, mapping.Enabled)
	return nil
}

// ToggleMappingByID ID로 매핑 토글
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

	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("ID 기반 키 맵핑 토글: %s (ID: %s, enabled=%v)", targetMapping.Name, mappingID, targetMapping.Enabled)
	return nil
}

// UpdateMapping 키 맵핑 수정
func (km *KeyMappingManager) UpdateMapping(oldStartKey, newName, newStartKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	oldMappings := km.mappings[oldStartKey]
	if len(oldMappings) == 0 {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", oldStartKey)
	}

	mapping := oldMappings[0]

	if err := km.validateKeys(newStartKey, keys); err != nil {
		return err
	}

	if oldStartKey != newStartKey {
		km.mappings[oldStartKey] = oldMappings[1:]
		if len(km.mappings[oldStartKey]) == 0 {
			delete(km.mappings, oldStartKey)
		}

		if km.mappings[newStartKey] == nil {
			km.mappings[newStartKey] = make([]*KeyMapping, 0)
		}
		km.mappings[newStartKey] = append(km.mappings[newStartKey], mapping)
	}

	mapping.Name = newName
	mapping.StartKey = newStartKey
	mapping.Keys = keys
	mapping.UpdatedAt = time.Now()

	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 수정: %s", newName)
	return nil
}

// UpdateMappingByID ID로 매핑 수정 (기존 호환)
func (km *KeyMappingManager) UpdateMappingByID(mappingID, newName, newStartKey string, keys []MappedKey) error {
	return km.UpdateMappingByIDWithRandom(mappingID, newName, newStartKey, keys, false, 0, 0)
}

// UpdateMappingByIDWithRandom ID로 매핑 수정 (랜덤 딜레이 포함)
func (km *KeyMappingManager) UpdateMappingByIDWithRandom(mappingID, newName, newStartKey string, keys []MappedKey, randomDelay bool, randomMin, randomMax int) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

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

	if oldStartKey != newStartKey {
		oldMappings := km.mappings[oldStartKey]
		for i, mapping := range oldMappings {
			if mapping.ID == mappingID {
				km.mappings[oldStartKey] = append(oldMappings[:i], oldMappings[i+1:]...)
				break
			}
		}

		if len(km.mappings[oldStartKey]) == 0 {
			delete(km.mappings, oldStartKey)
		}

		if km.mappings[newStartKey] == nil {
			km.mappings[newStartKey] = make([]*KeyMapping, 0)
		}
		km.mappings[newStartKey] = append(km.mappings[newStartKey], targetMapping)
	}

	targetMapping.Name = newName
	targetMapping.StartKey = newStartKey
	targetMapping.Keys = keys
	targetMapping.RandomDelay = randomDelay
	targetMapping.RandomDelayMin = randomMin
	targetMapping.RandomDelayMax = randomMax
	targetMapping.UpdatedAt = time.Now()

	km.rebuildActiveKeys()

	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("ID 기반 키 맵핑 수정: %s (ID: %s, 랜덤=%v)", newName, mappingID, randomDelay)
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

// GetMappings 전체 맵핑 반환 (시작키별 첫 번째만)
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

// GetAllMappings 전체 맵핑 반환 (중복키 지원)
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

// GetMapping 특정 시작키의 맵핑 반환
func (km *KeyMappingManager) GetMapping(startKey string) (*KeyMapping, bool) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	mappings := km.mappings[startKey]
	if len(mappings) == 0 {
		return nil, false
	}
	return mappings[0], true
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
		return fmt.Errorf("시작 키는 'delete', 'end', 'home'만 사용할 수 있습니다")
	}

	if len(keys) == 0 {
		return fmt.Errorf("실행할 키가 없습니다")
	}

	return nil
}

// validateComboKey 조합키 유효성 검사
func (km *KeyMappingManager) validateComboKey(comboKey string) error {
	key := strings.ToLower(comboKey)

	validModifiers := []string{"ctrl+", "shift+", "alt+", "cmd+", "win+"}

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
