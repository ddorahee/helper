package config

import (
	"encoding/json"
	"fmt"
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
	PeachType    string      `json:"peachType"`     // 복숭아 타입: "" / "silla" / "king" / "india"
	CompanionMode string     `json:"companionMode"` // 동시 메인화면: "" / "kanchen" / "daeya" — 설정 시 자동사냥 순환에서 빠지고 사냥시간(DurationMins) 동안 메인화면 자동화 병행 실행
	HuntAfterMins int        `json:"huntAfterMins"` // 동시실행 캐릭 전용: 다른 캐릭 자동사냥이 모두 끝난 뒤 이 시간(분)만큼 자동사냥 턴을 받음 (0 = 동시실행만)
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
	AlertConfirmX      int `json:"alertConfirmX"`      // 채굴 확인 버튼 X (ESC 후)
	AlertConfirmY      int `json:"alertConfirmY"`      // 채굴 확인 버튼 Y
	ReviveX            int `json:"reviveX"`            // 부활 버튼 X
	ReviveY            int `json:"reviveY"`            // 부활 버튼 Y
	SectButtonX        int `json:"sectButtonX"`        // 문파 버튼 X
	SectButtonY        int `json:"sectButtonY"`        // 문파 버튼 Y
	PeachReceiveX      int `json:"peachReceiveX"`      // 복숭아 받기 X
	PeachReceiveY      int `json:"peachReceiveY"`      // 복숭아 받기 Y
	ReceiveAcceptX     int `json:"receiveAcceptX"`     // 받기/수락 X
	ReceiveAcceptY     int `json:"receiveAcceptY"`     // 받기/수락 Y

	// 사냥 시작 후 창 최소화 (리소스 절감)
	MinimizeAfterStart bool `json:"minimizeAfterStart"`
}

// OCRRegionConfig OCR 이름 영역 좌표 설정
type OCRRegionConfig struct {
	NameRegionX      int  `json:"nameRegionX"`      // 크롭 시작 X (0 = 자동계산: 왼쪽 위)
	NameRegionY      int  `json:"nameRegionY"`      // 크롭 시작 Y
	NameRegionWidth  int  `json:"nameRegionWidth"`  // 크롭 너비
	NameRegionHeight int  `json:"nameRegionHeight"` // 크롭 높이
	Enabled          bool `json:"enabled"`          // OCR 자동 감지 사용 여부
}

// ItemPickupTargetItem 감지 대상 아이템 (영속화용)
type ItemPickupTargetItem struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// ItemPickupConfig 아이템 자동 습득 설정 (영속화용)
type ItemPickupConfig struct {
	Enabled      bool                   `json:"enabled"`
	Items        []ItemPickupTargetItem `json:"items"`
	ScanInterval int                    `json:"scanInterval"`
	TilePixelW   int                    `json:"tilePixelW"`
	TilePixelH   int                    `json:"tilePixelH"`
	OriginX      int                    `json:"originX"`
	OriginY      int                    `json:"originY"`
	TargetMap    string                 `json:"targetMap"`
	WrongMap     string                 `json:"wrongMap"`
	SkillKeys    []string               `json:"skillKeys"`
}

// CharacterData JSON 저장 구조
type CharacterData struct {
	Characters       []CharacterProfile            `json:"characters"`
	Coordinates      GameUICoordinates             `json:"coordinates"`
	OCRConfig        OCRRegionConfig               `json:"ocrConfig"`
	ItemPickupConfig ItemPickupConfig              `json:"itemPickupConfig,omitempty"`
	Presets          map[string][]CharacterProfile `json:"presets,omitempty"` // 캐릭터 구성 프리셋 (이름 → 캐릭터 목록 스냅샷)
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
				NameRegionX:      0, // 자동계산: 왼쪽 위
				NameRegionY:      5,
				NameRegionWidth:  200,
				NameRegionHeight: 30,
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

	if err := json.Unmarshal(content, &cs.data); err != nil {
		return err
	}

	// OCR 기본값 보정 (JSON에 ocrConfig 없으면 0으로 초기화되므로)
	if cs.data.OCRConfig.NameRegionWidth == 0 {
		cs.data.OCRConfig.NameRegionWidth = 200
	}
	if cs.data.OCRConfig.NameRegionHeight == 0 {
		cs.data.OCRConfig.NameRegionHeight = 30
		cs.data.OCRConfig.NameRegionY = 5
		cs.data.OCRConfig.Enabled = true
	}

	return nil
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

// SavePreset 현재 캐릭터 목록을 프리셋으로 저장 (같은 이름이면 덮어쓰기)
func (cs *CharacterStore) SavePreset(name string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.data.Presets == nil {
		cs.data.Presets = map[string][]CharacterProfile{}
	}
	snapshot := make([]CharacterProfile, len(cs.data.Characters))
	copy(snapshot, cs.data.Characters)
	cs.data.Presets[name] = snapshot
}

// ApplyPreset 프리셋의 캐릭터 목록으로 현재 목록을 교체
func (cs *CharacterStore) ApplyPreset(name string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	preset, ok := cs.data.Presets[name]
	if !ok {
		return fmt.Errorf("프리셋 '%s'을(를) 찾을 수 없습니다", name)
	}
	chars := make([]CharacterProfile, len(preset))
	copy(chars, preset)
	cs.data.Characters = chars
	return nil
}

// DeletePreset 프리셋 삭제
func (cs *CharacterStore) DeletePreset(name string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.data.Presets, name)
}

// GetPresets 프리셋 목록 반환 (이름 순 정렬, 캐릭터 스냅샷 포함)
func (cs *CharacterStore) GetPresets() map[string][]CharacterProfile {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	out := make(map[string][]CharacterProfile, len(cs.data.Presets))
	for k, v := range cs.data.Presets {
		chars := make([]CharacterProfile, len(v))
		copy(chars, v)
		out[k] = chars
	}
	return out
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

// Update 캐릭터 업데이트 (order는 기존 값 유지 — 수정 시 순서 변경 방지)
func (cs *CharacterStore) Update(profile CharacterProfile) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, c := range cs.data.Characters {
		if c.ID == profile.ID {
			// 기존 order 보존 (요청 측에서 잘못된 order를 보내도 순서가 유지됨)
			profile.Order = c.Order
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

// GetItemPickupConfig 아이템 습득 설정 반환
func (cs *CharacterStore) GetItemPickupConfig() ItemPickupConfig {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.data.ItemPickupConfig
}

// SetItemPickupConfig 아이템 습득 설정 업데이트
func (cs *CharacterStore) SetItemPickupConfig(cfg ItemPickupConfig) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.data.ItemPickupConfig = cfg
}
