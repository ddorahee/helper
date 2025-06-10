// keymapping/manager.go 완전한 코드 - 최종 버전 (한국어 기계식 키보드 지원)
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

// MappedKey 구조체 - 개별 키와 딜레이 설정
type MappedKey struct {
	Key   string `json:"key"`
	Delay int    `json:"delay"`
}

// KeyMappingManager 구조체 - 키 맵핑 관리자
type KeyMappingManager struct {
	mappings    map[string]*KeyMapping
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
		mappings:   make(map[string]*KeyMapping),
		configFile: configFile,
		stopChan:   make(chan bool),
	}

	km.LoadConfig()
	return km
}

// AddMapping 새로운 키 맵핑 추가
func (km *KeyMappingManager) AddMapping(name, startKey string, keys []MappedKey) error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if _, exists := km.mappings[startKey]; exists {
		return fmt.Errorf("시작 키 '%s'는 이미 사용 중입니다", startKey)
	}

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

	mapping, exists := km.mappings[oldStartKey]
	if !exists {
		return fmt.Errorf("키 맵핑을 찾을 수 없습니다: %s", oldStartKey)
	}

	if oldStartKey != newStartKey {
		if _, exists := km.mappings[newStartKey]; exists {
			return fmt.Errorf("시작 키 '%s'는 이미 사용 중입니다", newStartKey)
		}
	}

	if err := km.validateKeys(newStartKey, keys); err != nil {
		return err
	}

	if oldStartKey != newStartKey {
		delete(km.mappings, oldStartKey)
	}

	mapping.Name = newName
	mapping.StartKey = newStartKey
	mapping.Keys = keys
	mapping.UpdatedAt = time.Now()

	km.mappings[newStartKey] = mapping

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

	keyStr := km.keyCodeToString(ev.Keycode)
	if keyStr == "" {
		return
	}

	mapping, exists := km.mappings[keyStr]
	if !exists || !mapping.Enabled {
		return
	}

	log.Printf("키 맵핑 실행: %s (시작키: %s)", mapping.Name, keyStr)
	go km.executeKeySequence(mapping)
}

// executeKeySequence 키 시퀀스 실행
func (km *KeyMappingManager) executeKeySequence(mapping *KeyMapping) {
	for i, key := range mapping.Keys {
		if !km.IsRunning() {
			break
		}

		if err := km.sendKey(key.Key); err != nil {
			log.Printf("키 입력 실패 (%s): %v", key.Key, err)
			continue
		}

		if i < len(mapping.Keys)-1 && key.Delay > 0 {
			time.Sleep(time.Duration(key.Delay) * time.Millisecond)
		}

		if !km.IsRunning() {
			break
		}
	}
}

// sendKey 키 입력 전송
func (km *KeyMappingManager) sendKey(key string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("키 입력 중 패닉 복구: %v", r)
		}
	}()

	time.Sleep(50 * time.Millisecond)
	robotgo.KeyTap(key)
	return nil
}

// validateKeys 키 유효성 검사
func (km *KeyMappingManager) validateKeys(startKey string, keys []MappedKey) error {
	if startKey == "" {
		return fmt.Errorf("시작 키가 비어있습니다")
	}

	allowedStartKeys := []string{"delete", "end"}
	isValidStartKey := false
	for _, validKey := range allowedStartKeys {
		if strings.ToLower(startKey) == validKey {
			isValidStartKey = true
			break
		}
	}

	if !isValidStartKey {
		return fmt.Errorf("시작 키는 'delete' 또는 'end'만 사용할 수 있습니다")
	}

	if len(keys) == 0 {
		return fmt.Errorf("실행할 키가 없습니다")
	}

	for i, key := range keys {
		if key.Key == "" {
			return fmt.Errorf("키 %d가 비어있습니다", i+1)
		}

		if key.Delay < 0 || key.Delay > 1000 {
			return fmt.Errorf("키 %d의 딜레이는 0ms~1000ms 사이여야 합니다", i+1)
		}
	}

	return nil
}
