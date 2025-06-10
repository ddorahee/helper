// webui/src/services/keyMappingService.js 수정 (validateKeySequence 부분)
import { KEY_MAPPING } from '@constants/appConstants'

class KeyMappingService {
    constructor() {
        this.baseURL = '/api/keymapping'
    }

    // ... 기존 메서드들 그대로 유지 ...

    // 키 시퀀스 유효성 검사 (딜레이 범위 수정)
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

            // 딜레이 범위 수정: 0ms~1000ms
            if (delay < KEY_MAPPING.MIN_DELAY || delay > KEY_MAPPING.MAX_DELAY) {
                return {
                    valid: false,
                    error: `키 ${i + 1}의 딜레이는 ${KEY_MAPPING.MIN_DELAY}~${KEY_MAPPING.MAX_DELAY}ms 사이여야 합니다`
                }
            }
        }

        return { valid: true }
    }

    // 키 시퀀스 포맷팅 (0ms 처리 개선)
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

    // 키 시퀀스 파싱 (기본값 0ms)
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