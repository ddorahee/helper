import { createContext, useContext, useReducer, useEffect } from 'react'
import { appService } from '@services/appService'
import { MODES, TIME_OPTIONS } from '@constants/appConstants'

// 초기 상태
const initialState = {
    // 타이머 관련
    isRunning: false,
    isPaused: false,
    countdownTime: 3 * 60 * 60, // 3시간 (초)

    // 설정 관련
    currentMode: MODES.DAEYA_ENTER,
    currentTimeOption: TIME_OPTIONS.THREE_HOUR,

    // 앱 설정
    settings: {
        darkMode: true,
        soundEnabled: true,
        autoStartup: false,
        telegramEnabled: false
    },

    // 앱 정보
    version: '1.0.0',
    buildDate: 'unknown',

    // UI 상태
    currentSection: 'main',

    // 로그
    logs: [],
    logSettings: {
        autoRefresh: true,
        showDebug: false,
        filter: ''
    }
}

// 액션 타입
const actionTypes = {
    SET_RUNNING: 'SET_RUNNING',
    SET_PAUSED: 'SET_PAUSED',
    SET_COUNTDOWN_TIME: 'SET_COUNTDOWN_TIME',
    SET_MODE: 'SET_MODE',
    SET_TIME_OPTION: 'SET_TIME_OPTION',
    SET_SETTINGS: 'SET_SETTINGS',
    UPDATE_SETTING: 'UPDATE_SETTING',
    SET_VERSION: 'SET_VERSION',
    SET_CURRENT_SECTION: 'SET_CURRENT_SECTION',
    SET_LOGS: 'SET_LOGS',
    ADD_LOG: 'ADD_LOG',
    UPDATE_LOG_SETTINGS: 'UPDATE_LOG_SETTINGS',
    RESET_TIMER: 'RESET_TIMER',
    RESET_ALL: 'RESET_ALL'
}

// 리듀서
function appReducer(state, action) {
    switch (action.type) {
        case actionTypes.SET_RUNNING:
            return { ...state, isRunning: action.payload }

        case actionTypes.SET_PAUSED:
            return { ...state, isPaused: action.payload }

        case actionTypes.SET_COUNTDOWN_TIME:
            return { ...state, countdownTime: action.payload }

        case actionTypes.SET_MODE:
            return { ...state, currentMode: action.payload }

        case actionTypes.SET_TIME_OPTION:
            return { ...state, currentTimeOption: action.payload }

        case actionTypes.SET_SETTINGS:
            return { ...state, settings: { ...state.settings, ...action.payload } }

        case actionTypes.UPDATE_SETTING:
            return {
                ...state,
                settings: {
                    ...state.settings,
                    [action.key]: action.value
                }
            }

        case actionTypes.SET_VERSION:
            return {
                ...state,
                version: action.payload.version,
                buildDate: action.payload.buildDate
            }

        case actionTypes.SET_CURRENT_SECTION:
            return { ...state, currentSection: action.payload }

        case actionTypes.SET_LOGS:
            return { ...state, logs: action.payload }

        case actionTypes.ADD_LOG:
            return {
                ...state,
                logs: [...state.logs, {
                    id: Date.now(),
                    message: action.payload,
                    timestamp: new Date().toLocaleString()
                }]
            }

        case actionTypes.UPDATE_LOG_SETTINGS:
            return {
                ...state,
                logSettings: { ...state.logSettings, ...action.payload }
            }

        case actionTypes.RESET_TIMER:
            const hours = getHoursFromTimeOption(state.currentTimeOption)
            return {
                ...state,
                countdownTime: hours * 60 * 60,
                isRunning: false,
                isPaused: false
            }

        case actionTypes.RESET_ALL:
            return {
                ...state,
                currentMode: MODES.DAEYA_ENTER,
                currentTimeOption: TIME_OPTIONS.THREE_HOUR,
                countdownTime: 3 * 60 * 60,
                isRunning: false,
                isPaused: false
            }

        default:
            return state
    }
}

// 헬퍼 함수
function getHoursFromTimeOption(option) {
    switch (option) {
        case TIME_OPTIONS.ONE_HOUR:
            return 1 + (10 / 60)
        case TIME_OPTIONS.TWO_HOUR:
            return 2 + (10 / 60)
        case TIME_OPTIONS.THREE_HOUR:
            return 3 + (10 / 60)
        case TIME_OPTIONS.FOUR_HOUR:
            return 4 + (10 / 60)
        default:
            return 3 + (10 / 60)
    }
}

// 컨텍스트 생성
const AppContext = createContext()

// 프로바이더 컴포넌트
export function AppProvider({ children }) {
    const [state, dispatch] = useReducer(appReducer, initialState)

    // 액션 생성자들
    const actions = {
        setRunning: (running) => dispatch({ type: actionTypes.SET_RUNNING, payload: running }),
        setPaused: (paused) => dispatch({ type: actionTypes.SET_PAUSED, payload: paused }),
        setCountdownTime: (time) => dispatch({ type: actionTypes.SET_COUNTDOWN_TIME, payload: time }),
        setMode: (mode) => dispatch({ type: actionTypes.SET_MODE, payload: mode }),
        setTimeOption: (option) => dispatch({ type: actionTypes.SET_TIME_OPTION, payload: option }),
        setSettings: (settings) => dispatch({ type: actionTypes.SET_SETTINGS, payload: settings }),
        updateSetting: (key, value) => dispatch({ type: actionTypes.UPDATE_SETTING, key, value }),
        setVersion: (version, buildDate) => dispatch({
            type: actionTypes.SET_VERSION,
            payload: { version, buildDate }
        }),
        setCurrentSection: (section) => dispatch({ type: actionTypes.SET_CURRENT_SECTION, payload: section }),
        setLogs: (logs) => dispatch({ type: actionTypes.SET_LOGS, payload: logs }),
        addLog: (message) => dispatch({ type: actionTypes.ADD_LOG, payload: message }),
        updateLogSettings: (settings) => dispatch({ type: actionTypes.UPDATE_LOG_SETTINGS, payload: settings }),
        resetTimer: () => dispatch({ type: actionTypes.RESET_TIMER }),
        resetAll: () => dispatch({ type: actionTypes.RESET_ALL })
    }

    // 초기 데이터 로드
    useEffect(() => {
        async function loadInitialData() {
            try {
                // 저장된 설정 로드
                const settings = await appService.loadSettings()
                actions.setSettings(settings)

                // 상태 확인
                const status = await appService.getStatus()
                actions.setRunning(status.running)

                actions.addLog('프로그램이 시작되었습니다.')
            } catch (error) {
                actions.addLog('초기 데이터 로드 중 오류가 발생했습니다.')
            }
        }

        loadInitialData()
    }, [])

    // 상태 폴링 (2초마다)
    useEffect(() => {
        const interval = setInterval(async () => {
            try {
                const status = await appService.getStatus()
                if (status.running !== state.isRunning) {
                    actions.setRunning(status.running)
                }
            } catch (error) {
                // 오류 무시
            }
        }, 2000)

        return () => clearInterval(interval)
    }, [state.isRunning])

    const value = {
        state,
        actions,
        // 헬퍼 함수들
        getHoursFromTimeOption,
        getModeName: (mode) => {
            switch (mode) {
                case MODES.DAEYA_ENTER: return '대야 (입장)'
                case MODES.DAEYA_PARTY: return '대야 (파티)'
                case MODES.KANCHEN_ENTER: return '칸첸 (입장)'
                case MODES.KANCHEN_PARTY: return '칸첸 (파티)'
                default: return '알 수 없음'
            }
        },
        getApiModeName: (mode) => {
            switch (mode) {
                case MODES.DAEYA_ENTER: return 'daeya-entrance'
                case MODES.DAEYA_PARTY: return 'daeya-party'
                case MODES.KANCHEN_ENTER: return 'kanchen-entrance'
                case MODES.KANCHEN_PARTY: return 'kanchen-party'
                default: return ''
            }
        },
        formatTimeOption: (option) => {
            switch (option) {
                case TIME_OPTIONS.ONE_HOUR: return '1시간 10분'
                case TIME_OPTIONS.TWO_HOUR: return '2시간 10분'
                case TIME_OPTIONS.THREE_HOUR: return '3시간 10분'
                case TIME_OPTIONS.FOUR_HOUR: return '4시간 10분'
                default: return '3시간 10분'
            }
        }
    }

    return (
        <AppContext.Provider value={value}>
            {children}
        </AppContext.Provider>
    )
}

// 커스텀 훅
export function useApp() {
    const context = useContext(AppContext)
    if (!context) {
        throw new Error('useApp must be used within an AppProvider')
    }
    return context
}