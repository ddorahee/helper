import { API_ENDPOINTS } from '@constants/appConstants'

class TelegramService {
    async saveConfig(token, chatId) {
        try {
            const response = await fetch(API_ENDPOINTS.TELEGRAM_CONFIG, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `token=${encodeURIComponent(token)}&chat_id=${encodeURIComponent(chatId)}`
            })

            return response.ok
        } catch (error) {
            console.error('Failed to save telegram config:', error)
            return false
        }
    }

    async testConnection() {
        try {
            const response = await fetch(API_ENDPOINTS.TELEGRAM_TEST, {
                method: 'POST'
            })

            return response.ok
        } catch (error) {
            console.error('Failed to test telegram connection:', error)
            return false
        }
    }

    async getStatus() {
        try {
            const response = await fetch(API_ENDPOINTS.TELEGRAM_CONFIG)
            if (!response.ok) throw new Error('Failed to get telegram status')
            return await response.json()
        } catch (error) {
            console.error('Failed to get telegram status:', error)
            return { enabled: false }
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
            return response.ok
        } catch (error) {
            console.error('Failed to save setting:', error)
            return false
        }
    }
}

export const telegramService = new TelegramService()