package utils

import "strings"

// IsLocalhost는 요청이 localhost에서 온 것인지 확인합니다
func IsLocalhost(host string) bool {
	return strings.HasPrefix(host, "127.0.0.1:") || strings.HasPrefix(host, "localhost:")
}
