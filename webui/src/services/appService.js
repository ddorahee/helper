import { API_ENDPOINTS } from '@constants/appConstants'

class AppService {
    async loadSettings() {
        try {
            const response = await fetch(API_ENDPOINTS.SETTINGS_LOAD)
            if (!response.ok) throw new Error('Failed to load settings')
            return await response.json()
        } catch (error) {
            console.error('Failed to load settings:', error)
            return {}
        }
    }

    async saveSetting(type, value) {
        try {
            const response = await fetch(API_ENDPOINTS.SETTINGS, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `type=${type}&value=${value}`
            })
            if (!response.ok) throw new Error('Failed to save setting')
            return true
        } catch (error) {
            console.error('Failed to save setting:', error)
            return false
        }
    }

    async getStatus() {
        try {
            const response = await fetch(API_ENDPOINTS.STATUS)
            if (!response.ok) throw new Error('Failed to get status')
            return await response.json()
        } catch (error) {
            console.error('Failed to get status:', error)
            return { running: false, mode: 1 }
        }
    }

    async startOperation(mode, autoStopHours, isResume = false) {
        try {
            const body = `mode=${mode}&auto_stop=${autoStopHours}${isResume ? '&resume=true' : ''}`
            const response = await fetch(API_ENDPOINTS.START, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body
            })
            if (!response.ok) throw new Error('Failed to start operation')
            return true
        } catch (error) {
            console.error('Failed to start operation:', error)
            return false
        }
    }

    async stopOperation() {
        try {
            const response = await fetch(API_ENDPOINTS.STOP, {
                method: 'POST'
            })
            if (!response.ok) throw new Error('Failed to stop operation')
            return true
        } catch (error) {
            console.error('Failed to stop operation:', error)
            return false
        }
    }

    async resetSettings() {
        try {
            const response = await fetch(API_ENDPOINTS.RESET, {
                method: 'POST'
            })
            if (!response.ok) throw new Error('Failed to reset settings')
            return true
        } catch (error) {
            console.error('Failed to reset settings:', error)
            return false
        }
    }

    async exit() {
        try {
            const response = await fetch(API_ENDPOINTS.EXIT, {
                method: 'POST'
            })
            if (!response.ok) throw new Error('Failed to exit')
            return true
        } catch (error) {
            console.error('Failed to exit:', error)
            return false
        }
    }

    async getLogs() {
        try {
            console.log('API 요청 시작:', API_ENDPOINTS.LOGS)

            const response = await fetch(API_ENDPOINTS.LOGS, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                },
                // 캐시 방지
                cache: 'no-cache'
            })

            console.log('응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }

            const contentType = response.headers.get('content-type')
            if (!contentType || !contentType.includes('application/json')) {
                const text = await response.text()
                console.error('응답이 JSON이 아님:', text)
                throw new Error('서버에서 잘못된 응답 형식을 반환했습니다.')
            }

            const data = await response.json()
            console.log('파싱된 데이터:', data)

            // 데이터 구조 검증
            if (!data || typeof data !== 'object') {
                throw new Error('응답 데이터가 올바르지 않습니다.')
            }

            // logs 배열이 없으면 빈 배열로 초기화
            if (!Array.isArray(data.logs)) {
                data.logs = []
            }

            return data

        } catch (error) {
            console.error('getLogs 상세 오류:', error)

            // 네트워크 오류인지 확인
            if (error.name === 'TypeError' && error.message.includes('fetch')) {
                return {
                    logs: [
                        '서버에 연결할 수 없습니다.',
                        '프로그램이 실행 중인지 확인해주세요.',
                        `오류: ${error.message}`
                    ],
                    success: false,
                    error: 'network_error'
                }
            }

            // 기타 오류
            return {
                logs: [
                    '로그를 불러오는 중 오류가 발생했습니다.',
                    `오류 내용: ${error.message}`,
                    '새로고침 버튼을 눌러 다시 시도해주세요.'
                ],
                success: false,
                error: error.message
            }
        }
    }
    async clearLogs() {
        try {
            const response = await fetch(API_ENDPOINTS.LOGS_CLEAR, {
                method: 'POST'
            })
            if (!response.ok) throw new Error('Failed to clear logs')
            return true
        } catch (error) {
            console.error('Failed to clear logs:', error)
            return false
        }
    }

    async sendLog(message) {
        try {
            await fetch('/api/log', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ message })
            })
        } catch (error) {
            // 로그 전송 실패는 무시
        }
    }
}

export const appService = new AppService()