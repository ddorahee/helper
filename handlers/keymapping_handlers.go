// handlers/keymapping_handlers.go - 키 맵핑 초기화 기능 추가
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

// HandleMappings 키 맵핑 목록 관리 (GET, POST, PUT, DELETE)
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

// getMappings 모든 키 맵핑 조회
func (h *KeyMappingHandler) getMappings(w http.ResponseWriter, r *http.Request) {
	log.Printf("키 맵핑 목록 조회 요청")

	mappings := h.Manager.GetMappings()
	stats := h.Manager.GetMappingStats()

	log.Printf("반환할 맵핑 개수: %d", len(mappings))

	// 허용된 시작키만 포함된 맵핑만 반환
	filteredMappings := make(map[string]interface{})
	for key, mapping := range mappings {
		// delete 또는 end 키만 허용
		if strings.ToLower(key) == "delete" || strings.ToLower(key) == "end" {
			filteredMappings[key] = mapping
			log.Printf("유효한 맵핑: %s (시작키: %s)", mapping.Name, key)
		} else {
			log.Printf("허용되지 않은 시작키로 맵핑 제외: %s (시작키: %s)", mapping.Name, key)
		}
	}

	response := map[string]interface{}{
		"mappings": filteredMappings,
		"stats":    stats,
		"success":  true,
		"message":  fmt.Sprintf("총 %d개 맵핑 (허용된 키만)", len(filteredMappings)),
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

	// 시작키 검증 (delete 또는 end만 허용)
	startKeyLower := strings.ToLower(strings.TrimSpace(request.StartKey))
	if startKeyLower != "delete" && startKeyLower != "end" {
		log.Printf("허용되지 않은 시작키: %s", request.StartKey)
		http.Error(w, "시작키는 'delete' 또는 'end'만 사용할 수 있습니다", http.StatusBadRequest)
		return
	}

	// 키 시퀀스 파싱
	keys, err := h.Manager.ParseKeySequence(request.KeySequence)
	if err != nil {
		log.Printf("키 시퀀스 파싱 실패: %v", err)
		http.Error(w, fmt.Sprintf("키 시퀀스 파싱 실패: %v", err), http.StatusBadRequest)
		return
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

	// 키 시퀀스 파싱
	keys, err := h.Manager.ParseKeySequence(request.KeySequence)
	if err != nil {
		log.Printf("키 시퀀스 파싱 실패: %v", err)
		http.Error(w, fmt.Sprintf("키 시퀀스 파싱 실패: %v", err), http.StatusBadRequest)
		return
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

	response := map[string]interface{}{
		"success": true,
		"message": "키 맵핑이 삭제되었습니다",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleMappingToggle 키 맵핑 활성화/비활성화
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

	startKey := r.FormValue("start_key")
	if startKey == "" {
		http.Error(w, "start_key 파라미터가 필요합니다", http.StatusBadRequest)
		return
	}

	if err := h.Manager.ToggleMapping(startKey); err != nil {
		http.Error(w, fmt.Sprintf("키 맵핑 토글 실패: %v", err), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "키 맵핑 상태가 변경되었습니다",
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
		http.Error(w, "action 파라미터가 필요합니다 (start/stop)", http.StatusBadRequest)
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
				message = "키 맵핑 시스템이 시작되었습니다 (허용된 키: delete, end)"
				log.Printf("키 맵핑 시스템 시작됨")
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
		// 모든 키 맵핑 삭제 (새로 추가)
		log.Printf("모든 키 맵핑 삭제 요청")
		mappings := h.Manager.GetMappings()
		deletedCount := 0
		for startKey := range mappings {
			if err := h.Manager.RemoveMapping(startKey); err == nil {
				deletedCount++
			}
		}
		message = fmt.Sprintf("총 %d개 키 맵핑이 삭제되었습니다", deletedCount)
		log.Printf("키 맵핑 초기화 완료: %d개 삭제", deletedCount)
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

	log.Printf("반환할 키 카테고리 수: %d", len(availableKeys))
	for category, keys := range availableKeys {
		log.Printf("카테고리 '%s': %d개 키", category, len(keys))
	}

	response := map[string]interface{}{
		"keys":    availableKeys,
		"success": true,
		"message": "허용된 시작키: delete, end",
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
	}

	json.NewEncoder(w).Encode(response)
}
