// webui/src/services/keyMappingService.js 단순화 버전
import { KEY_MAPPING } from '@constants/appConstants'

class KeyMappingService {
    constructor() {
        this.baseURL = '/api/keymapping'
    }

    // 모든 키 맵핑 조회
    async getMappings() {
        try {
            console.log('키 맵핑 목록 요청 시작:', `${this.baseURL}/mappings`)

            const response = await fetch(`${this.baseURL}/mappings`, {
                method: 'GET',
                headers: {
                    'Accept': 'application/json',
                    'Content-Type': 'application/json',
                },
                cache: 'no-cache'
            })

            console.log('키 맵핑 목록 응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }

            const data = await response.json()
            console.log('키 맵핑 목록 데이터:', data)

            return data
        } catch (error) {
            console.error('키 맵핑 목록 조회 실패:', error)
            throw error
        }
    }

    // 새 키 맵핑 생성
    async createMapping(mappingData) {
        try {
            console.log('키 맵핑 생성 요청:', mappingData)

            const response = await fetch(`${this.baseURL}/mappings`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(mappingData)
            })

            console.log('키 맵핑 생성 응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                const errorText = await response.text()
                console.error('키 맵핑 생성 실패:', errorText)
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            console.log('키 맵핑 생성 결과:', result)
            return result.success
        } catch (error) {
            console.error('키 맵핑 생성 실패:', error)
            throw error
        }
    }

    // 키 맵핑 수정
    async updateMapping(mappingData) {
        try {
            console.log('키 맵핑 수정 요청:', mappingData)

            const response = await fetch(`${this.baseURL}/mappings`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(mappingData)
            })

            console.log('키 맵핑 수정 응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                const errorText = await response.text()
                console.error('키 맵핑 수정 실패:', errorText)
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            console.log('키 맵핑 수정 결과:', result)
            return result.success
        } catch (error) {
            console.error('키 맵핑 수정 실패:', error)
            throw error
        }
    }

    // 키 맵핑 삭제 (시작키 기반)
    async deleteMapping(startKey) {
        console.log('키 맵핑 삭제 요청:', startKey)

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

    // 키 맵핑 활성화/비활성화 (시작키 기반)
    async toggleMapping(startKey) {
        try {
            console.log('키 맵핑 토글 요청:', startKey)

            if (!startKey) {
                throw new Error('start_key가 필요합니다')
            }

            const response = await fetch(`${this.baseURL}/toggle`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: `start_key=${encodeURIComponent(startKey)}`
            })

            console.log('키 맵핑 토글 응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                const errorText = await response.text()
                console.error('키 맵핑 토글 실패:', errorText)
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            console.log('키 맵핑 토글 결과:', result)
            return result.success
        } catch (error) {
            console.error('키 맵핑 토글 실패:', error)
            throw error
        }
    }

    // 키 맵핑 시스템 제어 (시작/중지)
    async controlSystem(action) {
        try {
            console.log('키 맵핑 시스템 제어 요청:', action)

            const response = await fetch(`${this.baseURL}/control`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ action })
            })

            console.log('키 맵핑 제어 응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                const errorText = await response.text()
                console.error('키 맵핑 제어 실패:', errorText)
                throw new Error(errorText || `HTTP ${response.status}`)
            }

            const result = await response.json()
            console.log('키 맵핑 제어 결과:', result)
            return result.success
        } catch (error) {
            console.error('키 맵핑 시스템 제어 실패:', error)
            throw error
        }
    }

    // 사용 가능한 키 목록 조회
    async getAvailableKeys() {
        try {
            console.log('사용 가능한 키 목록 요청 시작:', `${this.baseURL}/keys`)

            const response = await fetch(`${this.baseURL}/keys`)
            console.log('키 목록 응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                console.error('키 목록 요청 실패:', response.status, response.statusText)
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }

            const data = await response.json()
            console.log('키 목록 원본 데이터:', data)

            if (!data || !data.success) {
                console.error('키 목록 응답이 성공하지 않음:', data)
                throw new Error('키 목록 요청이 실패했습니다')
            }

            if (!data.keys || typeof data.keys !== 'object') {
                console.error('키 목록 데이터가 올바르지 않음:', data.keys)
                throw new Error('키 목록 데이터 형식이 올바르지 않습니다')
            }

            console.log('사용 가능한 키 목록 성공:', data.keys)

            return data
        } catch (error) {
            console.error('사용 가능한 키 목록 조회 실패:', error)

            // 에러 발생 시 기본 키 목록 반환
            const fallbackKeys = {
                "숫자 키": ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"],
                "알파벳 키": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"],
                "특수 키": ["space", "enter", "esc", "tab"],
                "조합키 예시": ["ctrl+c", "ctrl+v", "alt+tab"]
            }

            console.log('폴백 키 목록 사용:', fallbackKeys)

            return {
                success: true,
                keys: fallbackKeys
            }
        }
    }

    // 키 맵핑 시스템 상태 조회
    async getStatus() {
        try {
            console.log('키 맵핑 상태 요청 시작:', `${this.baseURL}/status`)

            const response = await fetch(`${this.baseURL}/status`)
            console.log('키 맵핑 상태 응답 상태:', response.status, response.statusText)

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`)
            }

            const data = await response.json()
            console.log('키 맵핑 상태 데이터:', data)

            return data
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
            const match = key.match(/^([a-zA-Z0-9_+]+)\((\d+)\)$/)
            if (!match) {
                return {
                    valid: false,
                    error: `키 ${i + 1}의 형식이 올바르지 않습니다. 예: x(200), ctrl+c(500)`
                }
            }

            const [, keyName, delayStr] = match
            const delay = parseInt(delayStr)

            if (!keyName) {
                return { valid: false, error: `키 ${i + 1}이 비어있습니다` }
            }

            // 딜레이 범위: 0ms~1000ms
            if (delay < KEY_MAPPING.MIN_DELAY || delay > KEY_MAPPING.MAX_DELAY) {
                return {
                    valid: false,
                    error: `키 ${i + 1}의 딜레이는 ${KEY_MAPPING.MIN_DELAY}~${KEY_MAPPING.MAX_DELAY}ms 사이여야 합니다`
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

        return keys.map(key => {
            if (key.delay === 0) {
                return `${key.key}(즉시)`
            }
            return `${key.key}(${key.delay}ms)`
        }).join(', ')
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
                const match = key.match(/^([a-zA-Z0-9_+]+)\((\d+)\)$/)
                if (match) {
                    const [, keyName, delayStr] = match
                    return {
                        key: keyName,
                        delay: parseInt(delayStr) || KEY_MAPPING.DEFAULT_DELAY
                    }
                }
                return {
                    key: key,
                    delay: KEY_MAPPING.DEFAULT_DELAY
                }
            })
    }
}

export const keyMappingService = new KeyMappingService()
