// handlers/keymapping_handlers.go - 중복키 허용 및 조합키 지원
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

// HandleMappings 키 맵핑 목록 관리 (GET, POST, PUT, DELETE) - 중복키 지원
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

// getMappings 모든 키 맵핑 조회 (중복키 지원 - 모든 맵핑 반환)
func (h *KeyMappingHandler) getMappings(w http.ResponseWriter, r *http.Request) {
	log.Printf("키 맵핑 목록 조회 요청")

	// 모든 맵핑 정보 가져오기
	allMappings := h.Manager.GetAllMappings()
	stats := h.Manager.GetMappingStats()

	// 모든 맵핑을 플랫 구조로 변환 (UI 호환성)
	flatMappings := make(map[string]interface{})
	duplicateInfo := make(map[string]interface{})

	for startKey, mappingList := range allMappings {
		// delete 또는 end 키만 허용
		if strings.ToLower(startKey) != "delete" && strings.ToLower(startKey) != "end" {
			continue
		}

		// 각 맵핑에 고유 키 생성 (startKey + index)
		for i, mapping := range mappingList {
			var mapKey string
			if len(mappingList) == 1 {
				// 중복이 없으면 기존 방식
				mapKey = startKey
			} else {
				// 중복이 있으면 인덱스 추가
				mapKey = fmt.Sprintf("%s_%d", startKey, i)
			}

			// 맵핑 정보에 원래 시작키와 인덱스 정보 추가
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
			log.Printf("맵핑 추가: %s (키: %s, 활성화: %t, 중복: %t)",
				mapping.Name, mapKey, mapping.Enabled, len(mappingList) > 1)
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
		"message":        fmt.Sprintf("총 %d개 맵핑 (중복키 지원)", len(flatMappings)),
		"features": map[string]bool{
			"combo_keys":     true,
			"duplicate_keys": true,
			"auto_disable":   true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON 인코딩 실패: %v", err)
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		return
	}
}

// createMapping 새로운 키 맵핑 생성 (중복키 허용)
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

	// 시작키 검증 (delete 또는 end만 허용)
	startKeyLower := strings.ToLower(strings.TrimSpace(request.StartKey))
	if startKeyLower != "delete" && startKeyLower != "end" {
		log.Printf("허용되지 않은 시작키: %s", request.StartKey)
		http.Error(w, "시작키는 'delete' 또는 'end'만 사용할 수 있습니다", http.StatusBadRequest)
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

	// 키 맵핑 추가 (중복키 허용)
	if err := h.Manager.AddMapping(request.Name, startKeyLower, keys); err != nil {
		log.Printf("키 맵핑 추가 실패: %v", err)
		http.Error(w, fmt.Sprintf("키 맵핑 추가 실패: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("키 맵핑 생성 성공: %s (시작키: %s)", request.Name, startKeyLower)

	// 중복키 정보 포함하여 응답
	duplicateWarning := ""
	allMappings := h.Manager.GetAllMappings()
	if mappings, exists := allMappings[startKeyLower]; exists && len(mappings) > 1 {
		duplicateWarning = fmt.Sprintf("시작키 '%s'에 %d개의 맵핑이 있습니다. 하나만 활성화됩니다.", startKeyLower, len(mappings))
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("키 맵핑 '%s'가 추가되었습니다", request.Name),
		"warning": duplicateWarning,
		"features": map[string]bool{
			"combo_keys_supported":   true,
			"duplicate_keys_allowed": true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updateMapping 키 맵핑 수정
func (h *KeyMappingHandler) updateMapping(w http.ResponseWriter, r *http.Request) {
	log.Printf("키 맵핑 수정 요청")

	var request struct {
		OldStartKey string `json:"old_start_key"`
		Name        string `json:"name"`
		StartKey    string `json:"start_key"`
		KeySequence string `json:"key_sequence"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("JSON 디코딩 실패: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("수정 요청 데이터: old_start_key=%s, name=%s, start_key=%s",
		request.OldStartKey, request.Name, request.StartKey)

	// 입력 검증
	if request.OldStartKey == "" || request.Name == "" || request.StartKey == "" || request.KeySequence == "" {
		log.Printf("필수 필드 누락")
		http.Error(w, "필수 필드가 누락되었습니다", http.StatusBadRequest)
		return
	}

	// 시작키 검증
	startKeyLower := strings.ToLower(strings.TrimSpace(request.StartKey))
	if startKeyLower != "delete" && startKeyLower != "end" {
		log.Printf("허용되지 않은 시작키: %s", request.StartKey)
		http.Error(w, "시작키는 'delete' 또는 'end'만 사용할 수 있습니다", http.StatusBadRequest)
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

	// 키 맵핑 수정
	if err := h.Manager.UpdateMapping(request.OldStartKey, request.Name, startKeyLower, keys); err != nil {
		log.Printf("키 맵핑 수정 실패: %v", err)
		http.Error(w, fmt.Sprintf("키 맵핑 수정 실패: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("키 맵핑 수정 성공: %s (시작키: %s)", request.Name, startKeyLower)

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("키 맵핑 '%s'가 수정되었습니다", request.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// deleteMapping 키 맵핑 삭제
func (h *KeyMappingHandler) deleteMapping(w http.ResponseWriter, r *http.Request) {
	startKey := r.URL.Query().Get("start_key")

	log.Printf("키 맵핑 삭제 요청: start_key=%s", startKey)

	if startKey == "" {
		log.Printf("start_key 파라미터 없음")
		http.Error(w, "start_key 파라미터가 필요합니다", http.StatusBadRequest)
		return
	}

	// 맵핑 존재 확인
	mapping, exists := h.Manager.GetMapping(startKey)
	if !exists {
		log.Printf("키 맵핑을 찾을 수 없음: %s", startKey)
		response := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("키 맵핑을 찾을 수 없습니다: %s", startKey),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("삭제할 맵핑: %s (이름: %s)", startKey, mapping.Name)

	if err := h.Manager.RemoveMapping(startKey); err != nil {
		log.Printf("키 맵핑 삭제 실패: %v", err)
		response := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("키 맵핑 삭제 실패: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("키 맵핑 삭제 성공: %s", startKey)

	// 남은 중복키 정보 확인
	allMappings := h.Manager.GetAllMappings()
	remainingCount := 0
	if mappings, exists := allMappings[startKey]; exists {
		remainingCount = len(mappings)
	}

	warningMessage := ""
	if remainingCount > 0 {
		warningMessage = fmt.Sprintf("시작키 '%s'에 %d개의 맵핑이 더 있습니다", startKey, remainingCount)
	}

	response := map[string]interface{}{
		"success": true,
		"message": "키 맵핑이 삭제되었습니다",
		"warning": warningMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleMappingToggle 키 맵핑 활성화/비활성화
// handlers/keymapping_handlers.go 토글 기능 수정 부분
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

	// ID 기반 토글과 시작키 기반 토글 모두 지원
	startKey := r.FormValue("start_key")
	mappingID := r.FormValue("id")

	log.Printf("토글 요청 받음: start_key=%s, id=%s", startKey, mappingID)

	var err error
	var toggledName string

	if mappingID != "" {
		// ID 기반 토글
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
	} else if startKey != "" {
		// 시작키 기반 토글 (기존 방식)
		log.Printf("시작키 기반 토글 실행: %s", startKey)
		err = h.Manager.ToggleMapping(startKey)

		// 토글된 맵핑 이름 찾기
		mapping, exists := h.Manager.GetMapping(startKey)
		if exists && mapping != nil {
			toggledName = mapping.Name
		}
	} else {
		log.Printf("토글 파라미터 없음")
		http.Error(w, "start_key 또는 id 파라미터가 필요합니다", http.StatusBadRequest)
		return
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

	// 토글 후 상태 확인
	var status string
	if mappingID != "" {
		// ID로 상태 확인
		allMappings := h.Manager.GetAllMappings()
		found := false
		for _, mappings := range allMappings {
			for _, mapping := range mappings {
				if mapping.ID == mappingID {
					if mapping.Enabled {
						status = "활성화"
					} else {
						status = "비활성화"
					}
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			status = "알 수 없음"
		}
	} else {
		// 시작키로 상태 확인
		mapping, exists := h.Manager.GetMapping(startKey)
		if exists && mapping != nil && mapping.Enabled {
			status = "활성화"
		} else {
			status = "비활성화"
		}
	}

	log.Printf("키 맵핑 토글 성공: %s -> %s", toggledName, status)

	response := map[string]interface{}{
		"success": true,
		"message": "키 맵핑 상태가 변경되었습니다",
		"status":  status,
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
				message = "키 맵핑 시스템이 시작되었습니다 (조합키 및 중복키 지원)"
				log.Printf("키 맵핑 시스템 시작됨 (고급 기능 활성화)")
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
			"combo_keys":     true,
			"duplicate_keys": true,
			"auto_disable":   true,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// HandleAvailableKeys 사용 가능한 키 목록 반환 (조합키 포함)
func (h *KeyMappingHandler) HandleAvailableKeys(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	log.Printf("사용 가능한 키 목록 요청")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	availableKeys := h.Manager.GetAvailableKeys()

	log.Printf("반환할 키 카테고리 수: %d", len(availableKeys))
	for category, keys := range availableKeys {
		log.Printf("카테고리 '%s': %d개 키", category, len(keys))
	}

	response := map[string]interface{}{
		"keys":    availableKeys,
		"success": true,
		"message": "허용된 시작키: delete, end | 조합키 지원: ctrl+키, alt+키, shift+키, win+키",
		"features": map[string]bool{
			"combo_keys_supported":   true,
			"duplicate_keys_allowed": true,
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON 인코딩 실패: %v", err)
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		return
	}

	log.Printf("사용 가능한 키 목록 반환 완료")
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
			"combo_keys":     true,
			"duplicate_keys": true,
			"auto_disable":   true,
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
