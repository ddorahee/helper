// webui/src/services/keyMappingService.js
class KeyMappingService {
    constructor() {
        this.baseURL = '/api/keymapping'
    }

    // 모든 키 맵핑 조회
    async getMappings() {
        try {
            const response = await fetch(`${this.baseURL}/mappings`)
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }
            return await response.json()
        } catch (error) {
            console.error('키 맵핑 목록 조회 실패:', error)
            throw error
        }
    }

    // 새 키 맵핑 생성
    async createMapping(mappingData) {
        try {
            const response = await fetch(`${this.baseURL}/mappings`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(mappingData)
            })

            if (!response.ok) {
                const errorText = await response.text()
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            return result.success
        } catch (error) {
            console.error('키 맵핑 생성 실패:', error)
            throw error
        }
    }

    // 키 맵핑 수정
    async updateMapping(mappingData) {
        try {
            const response = await fetch(`${this.baseURL}/mappings`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(mappingData)
            })

            if (!response.ok) {
                const errorText = await response.text()
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            return result.success
        } catch (error) {
            console.error('키 맵핑 수정 실패:', error)
            throw error
        }
    }

    // 키 맵핑 삭제 (디버깅 강화)
    async deleteMapping(startKey) {
        console.log('deleteMapping 호출:', startKey)

        if (!startKey) {
            console.error('startKey가 없음')
            throw new Error('시작 키가 필요합니다')
        }

        try {
            const url = `${this.baseURL}/mappings?start_key=${encodeURIComponent(startKey)}`
            console.log('DELETE 요청 URL:', url)

            const response = await fetch(url, {
                method: 'DELETE',
                headers: {
                    'Content-Type': 'application/json',
                }
            })

            console.log('DELETE 응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                const errorText = await response.text()
                console.error('DELETE 응답 에러:', errorText)
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            console.log('DELETE 응답 데이터:', result)

            return result.success
        } catch (error) {
            console.error('키 맵핑 삭제 서비스 실패:', error)
            throw error
        }
    }

    // 키 맵핑 활성화/비활성화
    async toggleMapping(startKey) {
        try {
            const response = await fetch(`${this.baseURL}/toggle`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `start_key=${encodeURIComponent(startKey)}`
            })

            if (!response.ok) {
                const errorText = await response.text()
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            return result.success
        } catch (error) {
            console.error('키 맵핑 토글 실패:', error)
            throw error
        }
    }

    // 키 맵핑 시스템 제어 (시작/중지)
    async controlSystem(action) {
        try {
            const response = await fetch(`${this.baseURL}/control`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `action=${action}`
            })

            if (!response.ok) {
                const errorText = await response.text()
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            return result.success
        } catch (error) {
            console.error('키 맵핑 시스템 제어 실패:', error)
            throw error
        }
    }

    // 사용 가능한 키 목록 조회
    async getAvailableKeys() {
        try {
            const response = await fetch(`${this.baseURL}/keys`)
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }
            return await response.json()
        } catch (error) {
            console.error('사용 가능한 키 목록 조회 실패:', error)
            throw error
        }
    }

    // 키 맵핑 시스템 상태 조회
    async getStatus() {
        try {
            const response = await fetch(`${this.baseURL}/status`)
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }
            return await response.json()
        } catch (error) {
            console.error('키 맵핑 상태 조회 실패:', error)
            throw error
        }
    }

    // 키 시퀀스 유효성 검사
    validateKeySequence(keySequence) {
        if (!keySequence || typeof keySequence !== 'string') {
            return { valid: false, error: '키 시퀀스가 비어있습니다' }
        }

        const keys = keySequence.split(',').map(k => k.trim()).filter(k => k)

        if (keys.length === 0) {
            return { valid: false, error: '유효한 키가 없습니다' }
        }

        for (let i = 0; i < keys.length; i++) {
            const key = keys[i]

            // 키(딜레이) 형식 검사
            const match = key.match(/^([a-zA-Z0-9_]+)\((\d+)\)$/)
            if (!match) {
                return {
                    valid: false,
                    error: `키 ${i + 1}의 형식이 올바르지 않습니다. 예: 1(200)`
                }
            }

            const [, keyName, delayStr] = match
            const delay = parseInt(delayStr)

            if (!keyName) {
                return { valid: false, error: `키 ${i + 1}이 비어있습니다` }
            }

            if (delay < 100 || delay > 1000) {
                return {
                    valid: false,
                    error: `키 ${i + 1}의 딜레이는 100~1000ms 사이여야 합니다`
                }
            }
        }

        return { valid: true }
    }

    // 키 시퀀스 포맷팅
    formatKeySequence(keys) {
        if (!Array.isArray(keys) || keys.length === 0) {
            return ''
        }

        return keys.map(key => `${key.key}(${key.delay}ms)`).join(', ')
    }

    // 키 시퀀스 파싱
    parseKeySequence(keySequence) {
        if (!keySequence || typeof keySequence !== 'string') {
            return []
        }

        return keySequence.split(',')
            .map(k => k.trim())
            .filter(k => k)
            .map(key => {
                const match = key.match(/^([a-zA-Z0-9_]+)\((\d+)\)$/)
                if (match) {
                    const [, keyName, delayStr] = match
                    return {
                        key: keyName,
                        delay: parseInt(delayStr) || 200
                    }
                }
                return {
                    key: key,
                    delay: 200
                }
            })
    }
}

export const keyMappingService = new KeyMappingService()