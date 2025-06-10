// utils/mode_utils.go 수정
package utils

// GetModeName은 모드 코드를 한글 이름으로 변환합니다
func GetModeName(mode string) string {
	switch mode {
	case "daeya-entrance":
		return "대야 (입장)"
	case "daeya-party":
		return "대야 (파티)"
	case "kanchen-entrance":
		return "칸첸 (입장)"
	case "kanchen-party":
		return "칸첸 (파티)"
	default:
		return "알 수 없음"
	}
}
