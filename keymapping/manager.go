// keymapping/manager.go 완전한 코드
package keymapping

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

// KeyMapping 구조체 - 개별 키 맵핑 설정
type KeyMapping struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`      // 도사, 천인 등
	StartKey  string      `json:"start_key"` // 시작 키 (DEL, F1 등)
	Keys      []MappedKey `json:"keys"`      // 실행할 키들
	Enabled   bool        `json:"enabled"`   // 활성화 여부
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// MappedKey 구조체 - 개별 키와 딜레이 설정
type MappedKey struct {
	Key   string `json:"key"`   // 1, 2, 3, 4, 5 등
	Delay int    `json:"delay"` // 딜레이 (ms)
}

// KeyMappingManager 구조체 - 키 맵핑 관리자
type KeyMappingManager struct {
	mappings    map[string]*KeyMapping // 키 맵핑들 (start_key를 키로 사용)
	configFile  string                 // 설정 파일 경로
	mutex       sync.RWMutex
	running     bool
	stopChan    chan bool
	hookRunning bool
}

// NewKeyMappingManager 새로운 키 맵핑 매니저 생성
func NewKeyMappingManager(configDir string) *KeyMappingManager {
	configFile := filepath.Join(configDir, "keymappings.json")

	km := &KeyMappingManager{
		mappings:   make(map[string]*KeyMapping),
		configFile: configFile,
		stopChan:   make(chan bool),
	}

	// 설정 파일 로드
	km.LoadConfig()

	return km
}

// AddMapping 새로운 키 맵핑 추가
func (km *KeyMappingManager) AddMapping(name, startKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	// 시작 키 중복 검사
	if _, exists := km.mappings[startKey]; exists {
		return fmt.Errorf("시작 키 '%s'는 이미 사용 중입니다", startKey)
	}

	// 키 유효성 검사
	if err := km.validateKeys(startKey, keys); err != nil {
		return err
	}

	mapping := &KeyMapping{
		ID:        fmt.Sprintf("%s_%d", name, time.Now().Unix()),
		Name:      name,
		StartKey:  startKey,
		Keys:      keys,
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	km.mappings[startKey] = mapping

	// 설정 저장
	if err := km.SaveConfig(); err != nil {
		delete(km.mappings, startKey)
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 추가: %s (시작키: %s)", name, startKey)
	return nil
}

// UpdateMapping 기존 키 맵핑 수정
func (km *KeyMappingManager) UpdateMapping(oldStartKey, newName, newStartKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	// 기존 맵핑 존재 확인
	mapping, exists := km.mappings[oldStartKey]
	if !exists {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", oldStartKey)
	}

	// 새로운 시작 키가 다르고 이미 사용 중인지 확인
	if oldStartKey != newStartKey {
		if _, exists := km.mappings[newStartKey]; exists {
			return fmt.Errorf("시작 키 '%s'는 이미 사용 중입니다", newStartKey)
		}
	}

	// 키 유효성 검사
	if err := km.validateKeys(newStartKey, keys); err != nil {
		return err
	}

	// 기존 맵핑 제거 (시작 키가 변경된 경우)
	if oldStartKey != newStartKey {
		delete(km.mappings, oldStartKey)
	}

	// 맵핑 업데이트
	mapping.Name = newName
	mapping.StartKey = newStartKey
	mapping.Keys = keys
	mapping.UpdatedAt = time.Now()

	// 새로운 키로 저장
	km.mappings[newStartKey] = mapping

	// 설정 저장
	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 수정: %s (시작키: %s -> %s)", newName, oldStartKey, newStartKey)
	return nil
}

// RemoveMapping 키 맵핑 제거
func (km *KeyMappingManager) RemoveMapping(startKey string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	mapping, exists := km.mappings[startKey]
	if !exists {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", startKey)
	}

	delete(km.mappings, startKey)

	// 설정 저장
	if err := km.SaveConfig(); err != nil {
		return fmt.Errorf("설정 저장 실패: %v", err)
	}

	log.Printf("키 맵핑 제거: %s (시작키: %s)", mapping.Name, startKey)
	return nil
}

// ToggleMapping 키 맵핑 활성화/비활성화
func (km *KeyMappingManager) ToggleMapping(startKey string) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	mapping, exists := km.mappings[startKey]
	if !exists {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", startKey)
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

// GetMappings 모든 키 맵핑 반환
func (km *KeyMappingManager) GetMappings() map[string]*KeyMapping {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	// 복사본 반환
	result := make(map[string]*KeyMapping)
	for k, v := range km.mappings {
		result[k] = v
	}

	return result
}

// GetMapping 특정 키 맵핑 반환
func (km *KeyMappingManager) GetMapping(startKey string) (*KeyMapping, bool) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	mapping, exists := km.mappings[startKey]
	return mapping, exists
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

	log.Println("키 맵핑 시스템이 시작되었습니다")
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

	// 훅 중지 신호
	select {
	case km.stopChan <- true:
	default:
	}

	// gohook 이벤트 종료
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

	// 키 이벤트 처리
	evChan := hook.Start()
	defer hook.End()

	for {
		select {
		case <-km.stopChan:
			return
		case ev := <-evChan:
			if ev.Kind == hook.KeyDown {
				km.handleKeyPress(ev)
			}
		}
	}
}

// handleKeyPress 키 입력 처리
func (km *KeyMappingManager) handleKeyPress(ev hook.Event) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if !km.hookRunning {
		return
	}

	// 키 코드를 문자열로 변환
	keyStr := km.keyCodeToString(ev.Keycode)
	if keyStr == "" {
		return
	}

	// 해당 키로 시작하는 맵핑 찾기
	mapping, exists := km.mappings[keyStr]
	if !exists || !mapping.Enabled {
		return
	}

	// 키 시퀀스 실행 (별도 고루틴에서)
	go km.executeKeySequence(mapping)
}

// executeKeySequence 키 시퀀스 실행
func (km *KeyMappingManager) executeKeySequence(mapping *KeyMapping) {
	log.Printf("키 맵핑 실행: %s (시작키: %s)", mapping.Name, mapping.StartKey)

	for i, key := range mapping.Keys {
		// 중지 확인
		if !km.IsRunning() {
			break
		}

		// 키 입력
		if err := km.sendKey(key.Key); err != nil {
			log.Printf("키 입력 실패 (%s): %v", key.Key, err)
			continue
		}

		log.Printf("키 입력: %s", key.Key)

		// 딜레이 (마지막 키가 아닌 경우, 딜레이가 0이 아닌 경우)
		if i < len(mapping.Keys)-1 && key.Delay > 0 {
			time.Sleep(time.Duration(key.Delay) * time.Millisecond)
		}

		// 중지 확인
		if !km.IsRunning() {
			break
		}
	}

	log.Printf("키 맵핑 완료: %s", mapping.Name)
}

// sendKey 키 입력 전송
func (km *KeyMappingManager) sendKey(key string) error {
	// 안전한 키 입력을 위한 recover 처리
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 입력 중 패닉 복구: %v", r)
		}
	}()

	// 약간의 딜레이
	time.Sleep(50 * time.Millisecond)

	// robotgo를 사용하여 키 입력
	robotgo.KeyTap(key)

	return nil
}

// validateKeys 키 유효성 검사 (딜레이 범위 수정: 0ms~1000ms)
func (km *KeyMappingManager) validateKeys(startKey string, keys []MappedKey) error {
	// 시작 키 검사 - home과 delete만 허용
	if startKey == "" {
		return fmt.Errorf("시작 키가 비어있습니다")
	}

	// 키 목록 검사
	if len(keys) == 0 {
		return fmt.Errorf("실행할 키가 없습니다")
	}

	// 각 키와 딜레이 검사 (딜레이 범위 수정: 0ms~1000ms)
	for i, key := range keys {
		if key.Key == "" {
			return fmt.Errorf("키 %d가 비어있습니다", i+1)
		}

		// 딜레이 범위 수정: 0ms~1000ms
		if key.Delay < 0 || key.Delay > 1000 {
			return fmt.Errorf("키 %d의 딜레이는 0ms~1000ms 사이여야 합니다", i+1)
		}
	}

	return nil
}
