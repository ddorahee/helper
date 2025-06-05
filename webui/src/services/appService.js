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
            const response = await fetch(API_ENDPOINTS.LOGS)
            if (!response.ok) throw new Error('Failed to get logs')
            return await response.json()
        } catch (error) {
            console.error('Failed to get logs:', error)
            return { logs: [] }
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