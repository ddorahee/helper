package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// HuntingArea 사냥터 정보
type HuntingArea struct {
	Name          string `json:"name"`          // 사냥터 이름 (예: "아젠타석굴")
	DropdownIndex int    `json:"dropdownIndex"` // 게임 내 드롭다운 인덱스 (0부터)
}

// CharacterProfile 캐릭터 프로필
type CharacterProfile struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`         // 캐릭터 이름 (예: "테라폼")
	HuntingArea  HuntingArea `json:"huntingArea"`   // 사냥터
	DurationMins int         `json:"durationMins"`  // 사냥 시간 (분)
	Order        int         `json:"order"`         // 순환 순서 (0부터)
	Enabled      bool        `json:"enabled"`       // 순환에 포함할지 여부
	WindowHWND   uint64      `json:"-"`             // 런타임 전용 (매 실행마다 재할당)
	Assigned     bool        `json:"-"`             // 런타임 전용
}

// GameUICoordinates 게임 UI 요소의 상대 좌표
type GameUICoordinates struct {
	SwordButtonX       int `json:"swordButtonX"`
	SwordButtonY       int `json:"swordButtonY"`
	DropdownArrowX     int `json:"dropdownArrowX"`
	DropdownArrowY     int `json:"dropdownArrowY"`
	DropdownItemHeight int `json:"dropdownItemHeight"` // 드롭다운 항목 간 높이
	DropdownFirstItemY int `json:"dropdownFirstItemY"` // 첫 항목의 Y좌표
	StartButtonX       int `json:"startButtonX"`
	StartButtonY       int `json:"startButtonY"`
	ConfirmButtonX     int `json:"confirmButtonX"`     // 확인 대화창 "확인" 버튼 X
	ConfirmButtonY     int `json:"confirmButtonY"`     // 확인 대화창 "확인" 버튼 Y
}

// OCRRegionConfig OCR 이름 영역 좌표 설정
type OCRRegionConfig struct {
	NameRegionX      int  `json:"nameRegionX"`      // 크롭 시작 X (0 = 자동계산: 오른쪽 위)
	NameRegionY      int  `json:"nameRegionY"`      // 크롭 시작 Y
	NameRegionWidth  int  `json:"nameRegionWidth"`  // 크롭 너비
	NameRegionHeight int  `json:"nameRegionHeight"` // 크롭 높이
	Enabled          bool `json:"enabled"`          // OCR 자동 감지 사용 여부
}

// CharacterData JSON 저장 구조
type CharacterData struct {
	Characters  []CharacterProfile `json:"characters"`
	Coordinates GameUICoordinates  `json:"coordinates"`
	OCRConfig   OCRRegionConfig    `json:"ocrConfig"`
}

// CharacterStore 캐릭터 저장소
type CharacterStore struct {
	mu          sync.RWMutex
	data        CharacterData
	filePath    string
}

// NewCharacterStore 새로운 캐릭터 저장소 생성
func NewCharacterStore() *CharacterStore {
	return &CharacterStore{
		filePath: filepath.Join("config", "characters.json"),
		data: CharacterData{
			Characters: []CharacterProfile{},
			Coordinates: GameUICoordinates{
				SwordButtonX:       100,
				SwordButtonY:       450,
				DropdownArrowX:     200,
				DropdownArrowY:     300,
				DropdownItemHeight: 40,
				DropdownFirstItemY: 340,
				StartButtonX:       250,
				StartButtonY:       500,
			},
			OCRConfig: OCRRegionConfig{
				NameRegionX:      0,
				NameRegionY:      5,
				NameRegionWidth:  180,
				NameRegionHeight: 25,
				Enabled:          true,
			},
		},
	}
}

// Load JSON 파일에서 데이터 로드
func (cs *CharacterStore) Load() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	content, err := os.ReadFile(cs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 파일 없으면 기본값 사용
		}
		return err
	}

	return json.Unmarshal(content, &cs.data)
}

// Save JSON 파일로 저장
func (cs *CharacterStore) Save() error {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// config 폴더 생성
	dir := filepath.Dir(cs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(cs.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cs.filePath, content, 0666)
}

// GetAll 모든 캐릭터 반환
func (cs *CharacterStore) GetAll() []CharacterProfile {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]CharacterProfile, len(cs.data.Characters))
	copy(result, cs.data.Characters)
	return result
}

// GetByOrder 순서대로 정렬된 캐릭터 반환
func (cs *CharacterStore) GetByOrder() []CharacterProfile {
	chars := cs.GetAll()
	sort.Slice(chars, func(i, j int) bool {
		return chars[i].Order < chars[j].Order
	})
	return chars
}

// Add 캐릭터 추가
func (cs *CharacterStore) Add(profile CharacterProfile) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// order를 현재 최대값+1로 설정 (중복 방지)
	maxOrder := -1
	for _, c := range cs.data.Characters {
		if c.Order > maxOrder {
			maxOrder = c.Order
		}
	}
	profile.Order = maxOrder + 1

	cs.data.Characters = append(cs.data.Characters, profile)
}

// Remove 캐릭터 삭제
func (cs *CharacterStore) Remove(id string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, c := range cs.data.Characters {
		if c.ID == id {
			cs.data.Characters = append(cs.data.Characters[:i], cs.data.Characters[i+1:]...)
			return
		}
	}
}

// Update 캐릭터 업데이트
func (cs *CharacterStore) Update(profile CharacterProfile) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, c := range cs.data.Characters {
		if c.ID == profile.ID {
			cs.data.Characters[i] = profile
			return
		}
	}
}

// GetCoordinates 좌표 설정 반환
func (cs *CharacterStore) GetCoordinates() GameUICoordinates {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.data.Coordinates
}

// SetCoordinates 좌표 설정 업데이트
func (cs *CharacterStore) SetCoordinates(coords GameUICoordinates) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.data.Coordinates = coords
}

// SetWindowHWND 캐릭터에 윈도우 핸들 할당
func (cs *CharacterStore) SetWindowHWND(id string, hwnd uint64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, c := range cs.data.Characters {
		if c.ID == id {
			cs.data.Characters[i].WindowHWND = hwnd
			cs.data.Characters[i].Assigned = true
			return
		}
	}
}

// SetEnabled 캐릭터 활성화/비활성화
func (cs *CharacterStore) SetEnabled(id string, enabled bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, c := range cs.data.Characters {
		if c.ID == id {
			cs.data.Characters[i].Enabled = enabled
			return
		}
	}
}

// MoveOrder 캐릭터 순서 변경 (direction: -1=위, +1=아래)
func (cs *CharacterStore) MoveOrder(id string, direction int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 먼저 order 기준으로 정렬
	sort.Slice(cs.data.Characters, func(i, j int) bool {
		return cs.data.Characters[i].Order < cs.data.Characters[j].Order
	})

	// order 값을 0, 1, 2, 3... 으로 정규화 (중복 order 문제 방지)
	for i := range cs.data.Characters {
		cs.data.Characters[i].Order = i
	}

	// 대상 인덱스 찾기
	idx := -1
	for i, c := range cs.data.Characters {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}

	// 스왑 대상 계산
	swapIdx := idx + direction
	if swapIdx < 0 || swapIdx >= len(cs.data.Characters) {
		return
	}

	// 순서 교환
	cs.data.Characters[idx].Order, cs.data.Characters[swapIdx].Order =
		cs.data.Characters[swapIdx].Order, cs.data.Characters[idx].Order
}

// ClearAllAssignments 모든 윈도우 할당 초기화
func (cs *CharacterStore) ClearAllAssignments() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i := range cs.data.Characters {
		cs.data.Characters[i].WindowHWND = 0
		cs.data.Characters[i].Assigned = false
	}
}

// GetOCRConfig OCR 설정 반환
func (cs *CharacterStore) GetOCRConfig() OCRRegionConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.data.OCRConfig
}

// SetOCRConfig OCR 설정 업데이트
func (cs *CharacterStore) SetOCRConfig(cfg OCRRegionConfig) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.data.OCRConfig = cfg
}
