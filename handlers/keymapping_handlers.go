// handlers/keymapping_handlers.go - Home 키 허용 추가
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"example.com/m/keymapping"
	"example.com/m/utils"
)

// KeyMappingHandler 키 맵핑 핸들러
type KeyMappingHandler struct {
	Manager *keymapping.KeyMappingManager
}

// NewKeyMappingHandler 새로운 키 맵핑 핸들러 생성
func NewKeyMappingHandler(manager *keymapping.KeyMappingManager) *KeyMappingHandler {
	return &KeyMappingHandler{
		Manager: manager,
	}
}

// HandleMappings 키 맵핑 목록 관리 (GET, POST, PUT, DELETE) - ID 기반 처리
func (h *KeyMappingHandler) HandleMappings(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// CORS 헤더 추가
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getMappings(w, r)
	case http.MethodPost:
		h.createMapping(w, r)
	case http.MethodPut:
		h.updateMapping(w, r)
	case http.MethodDelete:
		h.deleteMapping(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getMappings 모든 키 맵핑 조회 (ID 기반으로 수정)
func (h *KeyMappingHandler) getMappings(w http.ResponseWriter, r *http.Request) {
	log.Printf("키 맵핑 목록 조회 요청")

	// 모든 맵핑 정보 가져오기
	allMappings := h.Manager.GetAllMappings()
	stats := h.Manager.GetMappingStats()

	// ID를 키로 하는 플랫 구조로 변환
	flatMappings := make(map[string]interface{})
	duplicateInfo := make(map[string]interface{})

	for startKey, mappingList := range allMappings {
		// delete, end, home 키만 허용
		if strings.ToLower(startKey) != "delete" && strings.ToLower(startKey) != "end" && strings.ToLower(startKey) != "home" {
			continue
		}

		// 각 맵핑을 ID를 키로 저장
		for i, mapping := range mappingList {
			// ID를 키로 사용 (중복 문제 해결)
			mapKey := mapping.ID

			// 맵핑 정보에 중복 정보 추가
			mappingData := map[string]interface{}{
				"id":               mapping.ID,
				"name":             mapping.Name,
				"start_key":        mapping.StartKey,
				"keys":             mapping.Keys,
				"enabled":          mapping.Enabled,
				"created_at":       mapping.CreatedAt,
				"updated_at":       mapping.UpdatedAt,
				"is_duplicate":     len(mappingList) > 1,
				"duplicate_index":  i,
				"total_duplicates": len(mappingList),
			}

			flatMappings[mapKey] = mappingData
			log.Printf("맵핑 추가: %s (ID: %s, 키: %s, 활성화: %t, 중복: %t)",
				mapping.Name, mapping.ID, startKey, mapping.Enabled, len(mappingList) > 1)
		}

		// 중복키 정보
		if len(mappingList) > 1 {
			duplicateInfo[startKey] = map[string]interface{}{
				"count":    len(mappingList),
				"active":   getActiveMappingName(mappingList),
				"mappings": getMappingNames(mappingList),
			}
		}
	}

	log.Printf("반환할 총 맵핑 개수: %d", len(flatMappings))

	response := map[string]interface{}{
		"mappings":       flatMappings,
		"stats":          stats,
		"duplicate_info": duplicateInfo,
		"success":        true,
		"message":        fmt.Sprintf("총 %d개 맵핑 (ID 기반)", len(flatMappings)),
		"features": map[string]bool{
			"combo_keys":     true,
			"duplicate_keys": true,
			"id_based":       true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON 인코딩 실패: %v", err)
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		return
	}
}

// createMapping 새로운 키 맵핑 생성
func (h *KeyMappingHandler) createMapping(w http.ResponseWriter, r *http.Request) {
	log.Printf("키 맵핑 생성 요청")

	var request struct {
		Name        string `json:"name"`
		StartKey    string `json:"start_key"`
		KeySequence string `json:"key_sequence"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("JSON 디코딩 실패: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("생성 요청 데이터: name=%s, start_key=%s, key_sequence=%s",
		request.Name, request.StartKey, request.KeySequence)

	// 입력 검증
	if request.Name == "" || request.StartKey == "" || request.KeySequence == "" {
		log.Printf("필수 필드 누락")
		http.Error(w, "필수 필드가 누락되었습니다", http.StatusBadRequest)
		return
	}

	// 시작키 검증 (delete, end, home만 허용) - HOME 키 추가됨
	startKeyLower := strings.ToLower(strings.TrimSpace(request.StartKey))
	if startKeyLower != "delete" && startKeyLower != "end" && startKeyLower != "home" {
		log.Printf("허용되지 않은 시작키: %s", request.StartKey)
		http.Error(w, "시작키는 'delete', 'end', 'home'만 사용할 수 있습니다", http.StatusBadRequest)
		return
	}

	// 키 시퀀스 파싱 (조합키 지원)
	keys, err := h.Manager.ParseKeySequence(request.KeySequence)
	if err != nil {
		log.Printf("키 시퀀스 파싱 실패: %v", err)
		http.Error(w, fmt.Sprintf("키 시퀀스 파싱 실패: %v", err), http.StatusBadRequest)
		return
	}

	// 조합키 유효성 검사
	for i, key := range keys {
		if isComboKey(key.Key) {
			if err := validateComboKey(key.Key); err != nil {
				log.Printf("조합키 유효성 검사 실패 (키 %d): %v", i+1, err)
				http.Error(w, fmt.Sprintf("키 %d의 조합키 형식 오류: %v", i+1, err), http.StatusBadRequest)
				return
			}
		}
	}

	// 키 맵핑 추가
	if err := h.Manager.AddMapping(request.Name, startKeyLower, keys); err != nil {
		log.Printf("키 맵핑 추가 실패: %v", err)
		http.Error(w, fmt.Sprintf("키 맵핑 추가 실패: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("키 맵핑 생성 성공: %s (시작키: %s)", request.Name, startKeyLower)

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("키 맵핑 '%s'가 추가되었습니다", request.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updateMapping 키 맵핑 수정 - ID 기반으로 수정
func (h *KeyMappingHandler) updateMapping(w http.ResponseWriter, r *http.Request) {
	log.Printf("키 맵핑 수정 요청")

	var request struct {
		ID          string `json:"id"` // ID 기반으로 변경
		Name        string `json:"name"`
		StartKey    string `json:"start_key"`
		KeySequence string `json:"key_sequence"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("JSON 디코딩 실패: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("수정 요청 데이터: id=%s, name=%s, start_key=%s",
		request.ID, request.Name, request.StartKey)

	// 입력 검증
	if request.ID == "" || request.Name == "" || request.StartKey == "" || request.KeySequence == "" {
		log.Printf("필수 필드 누락")
		http.Error(w, "필수 필드가 누락되었습니다", http.StatusBadRequest)
		return
	}

	// 시작키 검증 (HOME 키 포함)
	startKeyLower := strings.ToLower(strings.TrimSpace(request.StartKey))
	if startKeyLower != "delete" && startKeyLower != "end" && startKeyLower != "home" {
		log.Printf("허용되지 않은 시작키: %s", request.StartKey)
		http.Error(w, "시작키는 'delete', 'end', 'home'만 사용할 수 있습니다", http.StatusBadRequest)
		return
	}

	// 키 시퀀스 파싱
	keys, err := h.Manager.ParseKeySequence(request.KeySequence)
	if err != nil {
		log.Printf("키 시퀀스 파싱 실패: %v", err)
		http.Error(w, fmt.Sprintf("키 시퀀스 파싱 실패: %v", err), http.StatusBadRequest)
		return
	}

	// 키 맵핑 수정 (ID 기반)
	if err := h.Manager.UpdateMappingByID(request.ID, request.Name, startKeyLower, keys); err != nil {
		log.Printf("키 맵핑 수정 실패: %v", err)
		http.Error(w, fmt.Sprintf("키 맵핑 수정 실패: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("키 맵핑 수정 성공: %s (ID: %s)", request.Name, request.ID)

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("키 맵핑 '%s'가 수정되었습니다", request.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// deleteMapping 키 맵핑 삭제 - ID 기반으로 수정
func (h *KeyMappingHandler) deleteMapping(w http.ResponseWriter, r *http.Request) {
	// ID 파라미터 우선 확인
	mappingID := r.URL.Query().Get("id")
	startKey := r.URL.Query().Get("start_key")

	log.Printf("키 맵핑 삭제 요청: id=%s, start_key=%s", mappingID, startKey)

	if mappingID == "" && startKey == "" {
		log.Printf("삭제 파라미터 없음")
		http.Error(w, "id 또는 start_key 파라미터가 필요합니다", http.StatusBadRequest)
		return
	}

	var err error
	var deletedName string

	if mappingID != "" {
		// ID 기반 삭제 (우선)
		log.Printf("ID 기반 삭제 실행: %s", mappingID)
		deletedName, err = h.Manager.RemoveMappingByID(mappingID)
	} else {
		// 시작키 기반 삭제 (하위 호환성)
		log.Printf("시작키 기반 삭제 실행: %s", startKey)
		mapping, exists := h.Manager.GetMapping(startKey)
		if exists && mapping != nil {
			deletedName = mapping.Name
		}
		err = h.Manager.RemoveMapping(startKey)
	}

	if err != nil {
		log.Printf("키 맵핑 삭제 실패: %v", err)
		response := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("키 맵핑 삭제 실패: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("키 맵핑 삭제 성공: %s", deletedName)

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("키 맵핑 '%s'가 삭제되었습니다", deletedName),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleMappingToggle 키 맵핑 활성화/비활성화 - ID 우선 처리로 수정
func (h *KeyMappingHandler) HandleMappingToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// ID를 우선으로 확인
	mappingID := r.FormValue("id")
	startKey := r.FormValue("start_key")

	log.Printf("토글 요청 받음: id=%s, start_key=%s", mappingID, startKey)

	if mappingID == "" && startKey == "" {
		log.Printf("토글 파라미터 없음")
		http.Error(w, "id 또는 start_key 파라미터가 필요합니다", http.StatusBadRequest)
		return
	}

	var err error
	var toggledName string

	if mappingID != "" {
		// ID 기반 토글 (우선)
		log.Printf("ID 기반 토글 실행: %s", mappingID)
		err = h.Manager.ToggleMappingByID(mappingID)

		// 토글된 맵핑 이름 찾기
		allMappings := h.Manager.GetAllMappings()
		for _, mappings := range allMappings {
			for _, mapping := range mappings {
				if mapping.ID == mappingID {
					toggledName = mapping.Name
					break
				}
			}
			if toggledName != "" {
				break
			}
		}
	} else {
		// 시작키 기반 토글 (하위 호환성)
		log.Printf("시작키 기반 토글 실행: %s", startKey)
		err = h.Manager.ToggleMapping(startKey)

		// 토글된 맵핑 이름 찾기
		mapping, exists := h.Manager.GetMapping(startKey)
		if exists && mapping != nil {
			toggledName = mapping.Name
		}
	}

	if err != nil {
		log.Printf("키 맵핑 토글 실패: %v", err)
		response := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("키 맵핑 토글 실패: %v", err),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("키 맵핑 토글 성공: %s", toggledName)

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("키 맵핑 '%s' 상태가 변경되었습니다", toggledName),
		"name":    toggledName,
	}

	json.NewEncoder(w).Encode(response)
}

// HandleMappingControl 키 맵핑 시스템 시작/중지
func (h *KeyMappingHandler) HandleMappingControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	var action string
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		var request struct {
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		action = request.Action
	} else {
		action = r.FormValue("action")
	}

	if action == "" {
		http.Error(w, "action 파라미터가 필요합니다 (start/stop/clear)", http.StatusBadRequest)
		return
	}

	var err error
	var message string

	switch action {
	case "start":
		if h.Manager.IsRunning() {
			message = "키 맵핑 시스템이 이미 실행 중입니다"
		} else {
			log.Printf("키 맵핑 시스템 시작 요청")
			err = h.Manager.Start()
			if err == nil {
				message = "키 맵핑 시스템이 시작되었습니다 (DELETE, END, HOME 키 지원)"
				log.Printf("키 맵핑 시스템 시작됨 (HOME 키 포함)")
			}
		}
	case "stop":
		if !h.Manager.IsRunning() {
			message = "키 맵핑 시스템이 이미 중지되어 있습니다"
		} else {
			log.Printf("키 맵핑 시스템 중지 요청")
			h.Manager.Stop()
			message = "키 맵핑 시스템이 중지되었습니다"
			log.Printf("키 맵핑 시스템 중지됨")
		}
	case "clear":
		// 모든 키 맵핑 삭제
		log.Printf("모든 키 맵핑 삭제 요청")
		allMappings := h.Manager.GetAllMappings()
		deletedCount := 0
		for startKey := range allMappings {
			if err := h.Manager.RemoveMapping(startKey); err == nil {
				deletedCount++
			}
		}
		message = fmt.Sprintf("총 %d개 키 시작키의 맵핑이 삭제되었습니다", deletedCount)
		log.Printf("키 맵핑 초기화 완료: %d개 시작키 삭제", deletedCount)
	default:
		http.Error(w, "잘못된 액션입니다 (start/stop/clear)", http.StatusBadRequest)
		return
	}

	if err != nil {
		response := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("작업 실패: %v", err),
			"running": h.Manager.IsRunning(),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": message,
		"running": h.Manager.IsRunning(),
		"features": map[string]bool{
			"id_based":       true,
			"combo_keys":     true,
			"duplicate_keys": true,
			"home_key":       true, // HOME 키 지원 추가
		},
	}

	json.NewEncoder(w).Encode(response)
}

// HandleAvailableKeys 사용 가능한 키 목록 반환
func (h *KeyMappingHandler) HandleAvailableKeys(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	log.Printf("사용 가능한 키 목록 요청")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	availableKeys := h.Manager.GetAvailableKeys()

	response := map[string]interface{}{
		"keys":    availableKeys,
		"success": true,
		"message": "허용된 시작키: delete, end, home | 조합키 지원: ctrl+키, alt+키, shift+키, win+키",
		"features": map[string]bool{
			"combo_keys_supported":   true,
			"duplicate_keys_allowed": true,
			"id_based_operations":    true,
			"home_key_supported":     true, // HOME 키 지원 명시
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON 인코딩 실패: %v", err)
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		return
	}

	log.Printf("사용 가능한 키 목록 반환 완료 (HOME 키 포함)")
}

// HandleMappingStatus 키 맵핑 시스템 상태 반환
func (h *KeyMappingHandler) HandleMappingStatus(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	stats := h.Manager.GetMappingStats()

	response := map[string]interface{}{
		"stats":   stats,
		"success": true,
		"features": map[string]bool{
			"id_based":       true,
			"combo_keys":     true,
			"duplicate_keys": true,
			"home_key":       true, // HOME 키 지원 추가
		},
	}

	json.NewEncoder(w).Encode(response)
}

// 헬퍼 함수들

// isComboKey 조합키인지 확인
func isComboKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "ctrl+") ||
		strings.Contains(key, "shift+") ||
		strings.Contains(key, "alt+") ||
		strings.Contains(key, "cmd+") ||
		strings.Contains(key, "win+")
}

// validateComboKey 조합키 유효성 검사
func validateComboKey(key string) error {
	key = strings.ToLower(key)

	modifiers := []string{"ctrl+", "shift+", "alt+", "cmd+", "win+"}
	hasModifier := false
	mainKey := key

	for _, modifier := range modifiers {
		if strings.Contains(mainKey, modifier) {
			hasModifier = true
			mainKey = strings.Replace(mainKey, modifier, "", 1)
		}
	}

	if !hasModifier {
		return fmt.Errorf("조합키에는 최소 하나의 수정키가 필요합니다")
	}

	if strings.TrimSpace(mainKey) == "" {
		return fmt.Errorf("조합키에는 메인키가 필요합니다")
	}

	return nil
}

// getActiveMappingName 활성화된 맵핑 이름 반환
func getActiveMappingName(mappings []*keymapping.KeyMapping) string {
	for _, mapping := range mappings {
		if mapping.Enabled {
			return mapping.Name
		}
	}
	return "없음"
}

// getMappingNames 모든 맵핑 이름 반환
func getMappingNames(mappings []*keymapping.KeyMapping) []string {
	names := make([]string, len(mappings))
	for i, mapping := range mappings {
		names[i] = mapping.Name
	}
	return names
}
