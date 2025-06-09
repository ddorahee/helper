// 모드 상수
export const MODES = {
    NONE: 0,
    DAEYA_ENTER: 1,
    DAEYA_PARTY: 2,
    KANCHEN_ENTER: 3,
    KANCHEN_PARTY: 4
}

// 시간 옵션 상수
export const TIME_OPTIONS = {
    ONE_HOUR: 0,
    TWO_HOUR: 1,
    THREE_HOUR: 2,
    FOUR_HOUR: 3
}

// 섹션 상수
export const SECTIONS = {
    MAIN: 'main',
    KEYMAPPING: 'keymapping', // 키 맵핑 섹션 추가
    LOGS: 'logs',
    QUESTS: 'quests',
    SETTINGS: 'settings'
}

// API 엔드포인트
export const API_ENDPOINTS = {
    START: '/api/start',
    STOP: '/api/stop',
    RESET: '/api/reset',
    EXIT: '/api/exit',
    SETTINGS: '/api/settings',
    SETTINGS_LOAD: '/api/settings/load',
    STATUS: '/api/status',
    LOGS: '/api/logs',
    LOGS_CLEAR: '/api/logs/clear',
    TELEGRAM_CONFIG: '/api/telegram/config',
    TELEGRAM_TEST: '/api/telegram/test',

    // 키 맵핑 API 엔드포인트 추가
    KEYMAPPING_MAPPINGS: '/api/keymapping/mappings',
    KEYMAPPING_TOGGLE: '/api/keymapping/toggle',
    KEYMAPPING_CONTROL: '/api/keymapping/control',
    KEYMAPPING_KEYS: '/api/keymapping/keys',
    KEYMAPPING_STATUS: '/api/keymapping/status'
}

// 로그 레벨
export const LOG_LEVELS = {
    DEBUG: 'debug',
    INFO: 'info',
    WARNING: 'warning',
    ERROR: 'error'
}

// 알림 타입
export const NOTIFICATION_TYPES = {
    SUCCESS: 'success',
    ERROR: 'error',
    INFO: 'info',
    WARNING: 'warning'
}

// 키 맵핑 관련 상수
export const KEY_MAPPING = {
    MIN_DELAY: 100,     // 최소 딜레이 (ms)
    MAX_DELAY: 1000,    // 최대 딜레이 (ms)
    DEFAULT_DELAY: 200, // 기본 딜레이 (ms)

    // 키 카테고리
    CATEGORIES: {
        NUMBERS: '숫자 키',
        LETTERS: '알파벳 키',
        FUNCTION: '펑션 키',
        SPECIAL: '특수 키',
        ARROWS: '화살표 키',
        NUMPAD: '넘패드',
        MODIFIERS: '조합 키'
    }
}