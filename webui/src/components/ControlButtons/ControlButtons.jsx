// webui/src/components/ControlButtons/ControlButtons.jsx - 서버 타이머 연동
import { Play, Square, RotateCcw } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { appService } from '@services/appService'
import { useNotification } from '@hooks/useNotification'
import Button from '@components/Common/Button'
import styles from './ControlButtons.module.css'

export default function ControlButtons() {
    const { state, actions, getApiModeName, getHoursFromTimeOption, getModeName } = useApp()
    const { showNotification } = useNotification()

    // 서버 기반 타이머 시작
    const startServerTimer = async (durationHours) => {
        try {
            if (window.startServerTimer && typeof window.startServerTimer === 'function') {
                const durationSeconds = Math.floor(durationHours * 3600)
                await window.startServerTimer(durationSeconds)
                console.log('서버 타이머 시작:', durationSeconds, '초')
                return true
            } else {
                console.warn('서버 타이머 API 없음, 클라이언트 타이머 사용')
                // 클라이언트 fallback
                actions.setCountdownTime(durationHours * 3600)
                return true
            }
        } catch (error) {
            console.error('서버 타이머 시작 실패:', error)
            // 클라이언트 fallback
            actions.setCountdownTime(durationHours * 3600)
            return true
        }
    }

    // 서버 기반 타이머 중지
    const stopServerTimer = async () => {
        try {
            if (window.stopServerTimer && typeof window.stopServerTimer === 'function') {
                await window.stopServerTimer()
                console.log('서버 타이머 일시정지')
            }
        } catch (error) {
            console.error('서버 타이머 중지 실패:', error)
        }
    }

    // 서버 기반 타이머 리셋
    const resetServerTimer = async () => {
        try {
            if (window.resetServerTimer && typeof window.resetServerTimer === 'function') {
                await window.resetServerTimer()
                console.log('서버 타이머 리셋')

                // 클라이언트 상태도 리셋
                const hours = getHoursFromTimeOption(state.currentTimeOption)
                actions.setCountdownTime(hours * 3600)
            }
        } catch (error) {
            console.error('서버 타이머 리셋 실패:', error)
        }
    }

    const handleStart = async () => {
        if (state.isRunning) {
            actions.addLog('이미 작업이 실행 중입니다.')
            return
        }

        try {
            const apiMode = getApiModeName(state.currentMode)
            if (!apiMode) {
                actions.addLog('오류: 유효하지 않은 모드입니다.')
                return
            }

            const hours = getHoursFromTimeOption(state.currentTimeOption)
            const wasTimerPaused = state.isPaused

            // 상태 업데이트
            actions.setRunning(true)
            actions.setPaused(false)

            // 서버 기반 타이머 시작 (새로 시작하는 경우에만)
            if (!wasTimerPaused) {
                const timerStarted = await startServerTimer(hours)
                if (!timerStarted) {
                    throw new Error('서버 타이머 시작 실패')
                }
            }

            // 서버에 작업 시작 요청
            const success = await appService.startOperation(
                apiMode,
                hours,
                wasTimerPaused,
                state.settings.telegramEnabled
            )

            if (success) {
                const modeName = getModeName(state.currentMode)
                const message = wasTimerPaused
                    ? `${modeName} 모드 작업을 재개합니다...`
                    : `${modeName} 모드로 작업을 시작합니다... (서버 기반 타이머)`

                actions.addLog(message)
                showNotification('작업이 시작되었습니다 (정확한 서버 타이머)', 'success')

                // 텔레그램 시작 알림 로그
                if (!wasTimerPaused && state.settings.telegramEnabled) {
                    actions.addLog('텔레그램 시작 알림이 전송됩니다.')
                }
            } else {
                // 실패 시 상태 복원
                actions.setRunning(false)
                if (wasTimerPaused) actions.setPaused(true)
                await resetServerTimer() // 서버 타이머도 리셋
                actions.addLog('오류: 작업을 시작할 수 없습니다.')
                showNotification('작업 시작에 실패했습니다', 'error')
            }
        } catch (error) {
            console.error('작업 시작 오류:', error)
            actions.setRunning(false)
            await resetServerTimer() // 서버 타이머도 리셋
            actions.addLog('작업 시작 중 오류가 발생했습니다.')
            showNotification('작업 시작 중 오류가 발생했습니다', 'error')
        }
    }

    const handleStop = async () => {
        if (!state.isRunning) {
            actions.addLog('실행 중인 작업이 없습니다.')
            return
        }

        try {
            // 서버 타이머 일시정지
            await stopServerTimer()

            // 서버에 작업 중지 요청
            const success = await appService.stopOperation()

            if (success) {
                actions.setRunning(false)
                actions.setPaused(true)
                actions.addLog('작업이 일시 중지되었습니다. (서버 타이머 일시정지)')
                showNotification('작업이 중지되었습니다', 'info')
            } else {
                actions.addLog('오류: 작업을 중지할 수 없습니다.')
                showNotification('작업 중지에 실패했습니다', 'error')
            }
        } catch (error) {
            console.error('작업 중지 오류:', error)
            actions.addLog('작업 중지 중 오류가 발생했습니다.')
            showNotification('작업 중지 중 오류가 발생했습니다', 'error')
        }
    }

    const handleReset = async () => {
        if (state.isRunning) {
            actions.addLog('작업 중에는 재설정할 수 없습니다.')
            return
        }

        try {
            // 서버 타이머 리셋
            await resetServerTimer()

            // 서버에 리셋 요청
            const success = await appService.resetSettings()

            if (success) {
                actions.resetAll()
                actions.addLog('모든 설정이 초기화되었습니다. (서버 타이머 리셋)')
                showNotification('설정이 초기화되었습니다', 'success')
            } else {
                actions.addLog('오류: 재설정 작업을 실행할 수 없습니다.')
                showNotification('재설정에 실패했습니다', 'error')
            }
        } catch (error) {
            console.error('재설정 오류:', error)
            actions.addLog('재설정 중 오류가 발생했습니다.')
            showNotification('재설정 중 오류가 발생했습니다', 'error')
        }
    }

    return (
        <div className={styles.controlsContainer}>
            <Button
                variant={state.isRunning ? 'secondary' : 'success'}
                size="lg"
                onClick={handleStart}
                disabled={state.isRunning}
                icon={<Play size={20} />}
                className={styles.startButton}
            >
                {state.isPaused ? '재개' : '시작'}
            </Button>

            <Button
                variant="danger"
                size="lg"
                onClick={handleStop}
                disabled={!state.isRunning}
                icon={<Square size={20} />}
                className={styles.stopButton}
            >
                중지
            </Button>

            <Button
                variant="warning"
                size="lg"
                onClick={handleReset}
                disabled={state.isRunning}
                icon={<RotateCcw size={20} />}
                className={styles.resetButton}
            >
                재설정
            </Button>
        </div>
    )
}
