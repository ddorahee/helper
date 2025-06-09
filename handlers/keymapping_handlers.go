// handlers/keymapping_handlers.go
package handlers

import (
	"encoding/json"
	"fmt"
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
	mappings := h.Manager.GetMappings()
	stats := h.Manager.GetMappingStats()

	response := map[string]interface{}{
		"mappings": mappings,
		"stats":    stats,
		"success":  true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		return
	}
}

// createMapping 새로운 키 맵핑 생성
func (h *KeyMappingHandler) createMapping(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name        string `json:"name"`
		StartKey    string `json:"start_key"`
		KeySequence string `json:"key_sequence"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 입력 검증
	if request.Name == "" || request.StartKey == "" || request.KeySequence == "" {
		http.Error(w, "필수 필드가 누락되었습니다", http.StatusBadRequest)
		return
	}

	// 키 시퀀스 파싱
	keys, err := h.Manager.ParseKeySequence(request.KeySequence)
	if err != nil {
		http.Error(w, fmt.Sprintf("키 시퀀스 파싱 실패: %v", err), http.StatusBadRequest)
		return
	}

	// 키 맵핑 추가
	if err := h.Manager.AddMapping(request.Name, request.StartKey, keys); err != nil {
		http.Error(w, fmt.Sprintf("키 맵핑 추가 실패: %v", err), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("키 맵핑 '%s'가 추가되었습니다", request.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updateMapping 키 맵핑 수정
func (h *KeyMappingHandler) updateMapping(w http.ResponseWriter, r *http.Request) {
	var request struct {
		OldStartKey string `json:"old_start_key"`
		Name        string `json:"name"`
		StartKey    string `json:"start_key"`
		KeySequence string `json:"key_sequence"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 입력 검증
	if request.OldStartKey == "" || request.Name == "" || request.StartKey == "" || request.KeySequence == "" {
		http.Error(w, "필수 필드가 누락되었습니다", http.StatusBadRequest)
		return
	}

	// 키 시퀀스 파싱
	keys, err := h.Manager.ParseKeySequence(request.KeySequence)
	if err != nil {
		http.Error(w, fmt.Sprintf("키 시퀀스 파싱 실패: %v", err), http.StatusBadRequest)
		return
	}

	// 키 맵핑 수정
	if err := h.Manager.UpdateMapping(request.OldStartKey, request.Name, request.StartKey, keys); err != nil {
		http.Error(w, fmt.Sprintf("키 맵핑 수정 실패: %v", err), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("키 맵핑 '%s'가 수정되었습니다", request.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// deleteMapping 키 맵핑 삭제 (디버깅 강화)
func (h *KeyMappingHandler) deleteMapping(w http.ResponseWriter, r *http.Request) {
	startKey := r.URL.Query().Get("start_key")

	// 로그 추가
	fmt.Printf("DELETE 요청 받음 - start_key: '%s'\n", startKey)

	if startKey == "" {
		fmt.Println("DELETE 실패: start_key 파라미터 없음")
		http.Error(w, "start_key 파라미터가 필요합니다", http.StatusBadRequest)
		return
	}

	// 맵핑 존재 확인
	mapping, exists := h.Manager.GetMapping(startKey)
	if !exists {
		fmt.Printf("DELETE 실패: 키 맵핑을 찾을 수 없음 - '%s'\n", startKey)
		response := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("키 맵핑을 찾을 수 없습니다: %s", startKey),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
		return
	}

	fmt.Printf("삭제할 맵핑 찾음: %s (이름: %s)\n", startKey, mapping.Name)

	if err := h.Manager.RemoveMapping(startKey); err != nil {
		fmt.Printf("DELETE 실패: %v\n", err)
		response := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("키 맵핑 삭제 실패: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	fmt.Printf("DELETE 성공: %s\n", startKey)

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

	// CORS 헤더 추가
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

// HandleMappingControl 키 맵핑 시스템 시작/중지 (개선된 버전)
func (h *KeyMappingHandler) HandleMappingControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// CORS 헤더 추가
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// JSON이나 Form 데이터 모두 지원
	var action string

	// Content-Type 확인
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
			err = h.Manager.Start()
			if err == nil {
				message = "키 맵핑 시스템이 시작되었습니다"
			}
		}
	case "stop":
		if !h.Manager.IsRunning() {
			message = "키 맵핑 시스템이 이미 중지되어 있습니다"
		} else {
			h.Manager.Stop()
			message = "키 맵핑 시스템이 중지되었습니다"
		}
	default:
		http.Error(w, "잘못된 액션입니다 (start/stop)", http.StatusBadRequest)
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

	// CORS 헤더 추가
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	availableKeys := h.Manager.GetAvailableKeys()

	response := map[string]interface{}{
		"keys":    availableKeys,
		"success": true,
	}

	json.NewEncoder(w).Encode(response)
}

// HandleMappingStatus 키 맵핑 시스템 상태 반환
func (h *KeyMappingHandler) HandleMappingStatus(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// CORS 헤더 추가
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	stats := h.Manager.GetMappingStats()

	response := map[string]interface{}{
		"stats":   stats,
		"success": true,
	}

	json.NewEncoder(w).Encode(response)
}
