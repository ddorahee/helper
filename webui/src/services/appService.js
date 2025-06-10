// webui/src/services/appService.js 설정 저장 부분 수정
import { API_ENDPOINTS } from '@constants/appConstants'

class AppService {
    constructor() {
        this.isLoading = false
        this.lastRequest = null
    }

    async loadSettings() {
        try {
            const response = await fetch(API_ENDPOINTS.SETTINGS_LOAD)
            if (!response.ok) throw new Error('Failed to load settings')
            return await response.json()
        } catch (error) {
            console.error('Failed to load settings:', error)
            return {
                darkMode: true,
                autoStartup: false,
                telegramEnabled: false
            }
        }
    }

    // 개별 설정 저장 (수정됨)
    async saveSetting(type, value) {
        try {
            console.log('설정 저장 요청:', { type, value })

            const response = await fetch(API_ENDPOINTS.SETTINGS, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `type=${encodeURIComponent(type)}&value=${encodeURIComponent(value ? '1' : '0')}`
            })

            console.log('설정 저장 응답:', response.status, response.statusText)

            if (!response.ok) {
                const errorText = await response.text()
                console.error('설정 저장 실패:', errorText)
                throw new Error(`Failed to save setting: ${response.status}`)
            }

            const responseText = await response.text()
            console.log('설정 저장 성공:', responseText)
            return true
        } catch (error) {
            console.error('Failed to save setting:', error)
            return false
        }
    }

    // 모든 설정 한번에 저장 (새로 추가)
    async saveAllSettings(settings) {
        try {
            const promises = Object.entries(settings).map(([key, value]) => {
                // 설정 키 매핑
                const settingKeyMap = {
                    darkMode: 'dark_mode',
                    autoStartup: 'auto_startup',
                    telegramEnabled: 'telegram_enabled'
                }

                const apiKey = settingKeyMap[key] || key
                return this.saveSetting(apiKey, value)
            })

            const results = await Promise.all(promises)
            return results.every(result => result === true)
        } catch (error) {
            console.error('Failed to save all settings:', error)
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

    async startOperation(mode, autoStopHours, isResume = false, telegramEnabled = false) {
        try {
            console.log('작업 시작 요청:', { mode, autoStopHours, isResume, telegramEnabled })

            const body = `mode=${mode}&auto_stop=${autoStopHours}${isResume ? '&resume=true' : ''}${telegramEnabled ? '&telegram_enabled=true' : ''}`

            console.log('전송 데이터:', body)

            const response = await fetch(API_ENDPOINTS.START, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body
            })

            console.log('시작 응답:', response.status, response.statusText)

            if (!response.ok) {
                const errorText = await response.text()
                console.error('시작 요청 실패:', errorText)
                throw new Error(`Failed to start operation: ${response.status}`)
            }

            const responseText = await response.text()
            console.log('시작 성공:', responseText)
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
        // 중복 요청 방지
        if (this.isLoading) {
            console.log('이미 로그 요청 진행 중, 무시')
            return this.lastRequest
        }

        this.isLoading = true

        try {
            console.log('로그 API 요청 시작:', '/api/logs')

            const request = fetch('/api/logs', {
                method: 'GET',
                headers: {
                    'Accept': 'application/json',
                    'Content-Type': 'application/json',
                },
                cache: 'no-cache'
            })

            this.lastRequest = request

            const response = await request

            console.log('응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }

            const data = await response.json()
            console.log('파싱된 데이터:', data)

            // 데이터 구조 검증
            if (!data || typeof data !== 'object') {
                throw new Error('응답 데이터가 올바르지 않습니다')
            }

            // logs 배열이 없으면 빈 배열로 초기화
            if (!Array.isArray(data.logs)) {
                data.logs = []
            }

            return data

        } catch (error) {
            console.error('getLogs 오류:', error)

            return {
                logs: [
                    '로그를 불러오는 중 오류가 발생했습니다.',
                    `오류 내용: ${error.message}`,
                    '서버가 실행 중인지 확인해주세요.'
                ],
                success: false,
                error: error.message
            }
        } finally {
            this.isLoading = false
            this.lastRequest = null
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