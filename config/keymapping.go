package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// KeySequenceConfig 키 시퀀스 설정 (JSON 직렬화용)
type KeySequenceConfig struct {
	Keys   []string  `json:"keys"`
	Delays []float64 `json:"delays"` // 초 단위
}

// KeyMappingData JSON 저장 구조
type KeyMappingData struct {
	Sequences map[string]KeySequenceConfig `json:"sequences"`
}

// KeyMappingStore 키 맵핑 저장소
type KeyMappingStore struct {
	mu       sync.RWMutex
	data     KeyMappingData
	filePath string
}

// NewKeyMappingStore 새로운 키 맵핑 저장소 생성
func NewKeyMappingStore() *KeyMappingStore {
	return &KeyMappingStore{
		filePath: filepath.Join("config", "keymapping.json"),
		data: KeyMappingData{
			Sequences: map[string]KeySequenceConfig{
				"daeya-enter": {
					Keys:   []string{"o", "enter", "enter", "esc", "d", "x", "5"},
					Delays: []float64{3, 1, 1, 1, 0, 0},
				},
				"daeya-party": {
					Keys:   []string{"x", "d"},
					Delays: []float64{1, 1},
				},
				"kanchen-enter": {
					Keys:   []string{"o", "enter", "enter", "esc", "d"},
					Delays: []float64{3, 1, 1, 1},
				},
				"kanchen-party": {
					Keys:   []string{"x", "d"},
					Delays: []float64{1, 1},
				},
			},
		},
	}
}

// Load JSON 파일에서 데이터 로드
func (ks *KeyMappingStore) Load() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	content, err := os.ReadFile(ks.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 파일 없으면 기본값 사용
		}
		return err
	}

	return json.Unmarshal(content, &ks.data)
}

// Save JSON 파일로 저장
func (ks *KeyMappingStore) Save() error {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	dir := filepath.Dir(ks.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(ks.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ks.filePath, content, 0666)
}

// GetAll 모든 키 맵핑 반환
func (ks *KeyMappingStore) GetAll() KeyMappingData {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.data
}

// SetAll 모든 키 맵핑 설정
func (ks *KeyMappingStore) SetAll(data KeyMappingData) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.data = data
}

// GetSequence 특정 모드의 키 시퀀스 반환
func (ks *KeyMappingStore) GetSequence(mode string) (KeySequenceConfig, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	seq, ok := ks.data.Sequences[mode]
	return seq, ok
}
