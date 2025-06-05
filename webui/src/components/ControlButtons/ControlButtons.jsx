import { Play, Square, RotateCcw } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { appService } from '@services/appService'
import { useNotification } from '@hooks/useNotification'
import Button from '@components/Common/Button'
import styles from './ControlButtons.module.css'

export default function ControlButtons() {
    const { state, actions, getApiModeName, getHoursFromTimeOption, getModeName } = useApp()
    const { showNotification } = useNotification()

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

            // 서버에 시작 요청
            const success = await appService.startOperation(apiMode, hours, wasTimerPaused)

            if (success) {
                // 클라이언트 타이머 시작
                if (!wasTimerPaused) {
                    actions.setCountdownTime(hours * 60 * 60)
                }

                const message = wasTimerPaused
                    ? `${getModeName(state.currentMode)} 모드 작업을 재개합니다...`
                    : `${getModeName(state.currentMode)} 모드로 작업을 시작합니다...`

                actions.addLog(message)
                showNotification('작업이 시작되었습니다', 'success')
            } else {
                // 실패 시 상태 복원
                actions.setRunning(false)
                if (wasTimerPaused) actions.setPaused(true)
                actions.addLog('오류: 작업을 시작할 수 없습니다.')
                showNotification('작업 시작에 실패했습니다', 'error')
            }
        } catch (error) {
            actions.setRunning(false)
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
            const success = await appService.stopOperation()

            if (success) {
                actions.setRunning(false)
                actions.setPaused(true)
                actions.addLog('작업이 일시 중지되었습니다.')
                showNotification('작업이 중지되었습니다', 'info')
            } else {
                actions.addLog('오류: 작업을 중지할 수 없습니다.')
                showNotification('작업 중지에 실패했습니다', 'error')
            }
        } catch (error) {
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
            const success = await appService.resetSettings()

            if (success) {
                actions.resetAll()
                actions.addLog('모든 설정이 초기화되었습니다.')
                showNotification('설정이 초기화되었습니다', 'success')
            } else {
                actions.addLog('오류: 재설정 작업을 실행할 수 없습니다.')
                showNotification('재설정에 실패했습니다', 'error')
            }
        } catch (error) {
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