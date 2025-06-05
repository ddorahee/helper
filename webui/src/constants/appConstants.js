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
    LOGS: 'logs',
    QUESTS: 'quests',
    SETTINGS: 'settings'
}

// 퀘스트 카테고리
export const QUEST_CATEGORIES = {
    GAME: 'game',
    DAILY: 'daily',
    SHOPPING: 'shopping',
    SPECIAL: 'special'
}

// 퀘스트 우선순위
export const QUEST_PRIORITIES = {
    LOW: 'low',
    MEDIUM: 'medium',
    HIGH: 'high'
}

// 카테고리 아이콘
export const CATEGORY_ICONS = {
    [QUEST_CATEGORIES.GAME]: '🎮',
    [QUEST_CATEGORIES.DAILY]: '📅',
    [QUEST_CATEGORIES.SHOPPING]: '🛒',
    [QUEST_CATEGORIES.SPECIAL]: '⭐'
}

// 카테고리 이름
export const CATEGORY_NAMES = {
    [QUEST_CATEGORIES.GAME]: '게임 퀘스트',
    [QUEST_CATEGORIES.DAILY]: '일상 업무',
    [QUEST_CATEGORIES.SHOPPING]: '쇼핑 목록',
    [QUEST_CATEGORIES.SPECIAL]: '특별 임무'
}

// 우선순위 라벨
export const PRIORITY_LABELS = {
    [QUEST_PRIORITIES.LOW]: '낮음',
    [QUEST_PRIORITIES.MEDIUM]: '보통',
    [QUEST_PRIORITIES.HIGH]: '높음'
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
    TELEGRAM_TEST: '/api/telegram/test'
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