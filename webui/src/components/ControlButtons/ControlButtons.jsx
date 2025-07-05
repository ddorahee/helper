// webui/src/components/ControlButtons/ControlButtons.jsx - 안전한 서버 타이머 연동
import { Play, Square, RotateCcw } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { appService } from '@services/appService'
import { useNotification } from '@hooks/useNotification'
import Button from '@components/Common/Button'
import styles from './ControlButtons.module.css'

export default function ControlButtons() {
    const { state, actions, getApiModeName, getHoursFromTimeOption, getModeName } = useApp()
    const { showNotification } = useNotification()

    // 서버 기반 타이머 시작 (안전한 버전)
    const startServerTimer = async (durationHours) => {
        try {
            console.log('서버 타이머 시작 시도:', durationHours, '시간')

            // window.startServerTimer가 있는지 확인
            if (typeof window.startServerTimer === 'function') {
                const durationSeconds = Math.floor(durationHours * 3600)
                console.log('서버 타이머 API 호출:', durationSeconds, '초')

                await window.startServerTimer(durationSeconds)
                console.log('서버 타이머 시작 성공')
                return true
            } else {
                console.warn('서버 타이머 API를 찾을 수 없음, 클라이언트 타이머 사용')
                // 클라이언트 fallback
                actions.setCountdownTime(durationHours * 3600)
                return true
            }
        } catch (error) {
            console.error('서버 타이머 시작 실패:', error)
            // 클라이언트 fallback - 에러가 나도 계속 진행
            actions.setCountdownTime(durationHours * 3600)
            return true
        }
    }

    // 서버 기반 타이머 중지 (안전한 버전)
    const stopServerTimer = async () => {
        try {
            if (typeof window.stopServerTimer === 'function') {
                await window.stopServerTimer()
                console.log('서버 타이머 일시정지 성공')
            } else {
                console.log('서버 타이머 API 없음, 클라이언트에서만 중지')
            }
        } catch (error) {
            console.error('서버 타이머 중지 실패:', error)
            // 에러가 나도 계속 진행
        }
    }

    // 서버 기반 타이머 리셋 (안전한 버전)
    const resetServerTimer = async () => {
        try {
            if (typeof window.resetServerTimer === 'function') {
                await window.resetServerTimer()
                console.log('서버 타이머 리셋 성공')
            } else {
                console.log('서버 타이머 API 없음, 클라이언트에서만 리셋')
            }

            // 클라이언트 상태도 리셋
            const hours = getHoursFromTimeOption(state.currentTimeOption)
            actions.setCountdownTime(hours * 3600)
        } catch (error) {
            console.error('서버 타이머 리셋 실패:', error)
            // 에러가 나도 클라이언트 상태는 리셋
            const hours = getHoursFromTimeOption(state.currentTimeOption)
            actions.setCountdownTime(hours * 3600)
        }
    }

    const handleStart = async () => {
        console.log('시작 버튼 클릭됨')

        try {
            // 1. 서버 상태 먼저 확인
            console.log('서버 상태 확인 중...')
            const serverStatus = await appService.getStatus()
            console.log('서버 상태:', serverStatus)

            if (serverStatus.running && !state.isPaused) {
                console.log('서버에서 이미 실행 중, 강제 중지 시도')
                await appService.stopOperation()
                // 잠시 대기
                await new Promise(resolve => setTimeout(resolve, 200))
            }

            // 2. 클라이언트 상태 확인
            if (state.isRunning && !state.isPaused) {
                console.log('클라이언트에서 이미 실행 중')
                actions.addLog('이미 작업이 실행 중입니다.')
                return
            }

            // 3. 기본 검증
            const apiMode = getApiModeName(state.currentMode)
            if (!apiMode) {
                console.error('유효하지 않은 모드:', state.currentMode)
                actions.addLog('오류: 유효하지 않은 모드입니다.')
                showNotification('유효하지 않은 모드입니다', 'error')
                return
            }

            const hours = getHoursFromTimeOption(state.currentTimeOption)
            const wasTimerPaused = state.isPaused

            console.log('시작 파라미터:', { apiMode, hours, wasTimerPaused })

            // 4. 상태 업데이트 (먼저 UI 상태부터)
            actions.setRunning(true)
            actions.setPaused(false)

            console.log('UI 상태 업데이트 완료')

            // 5. 서버 기반 타이머 시작 (새로 시작하는 경우에만)
            if (!wasTimerPaused) {
                console.log('새 타이머 시작')
                const timerStarted = await startServerTimer(hours)
                if (!timerStarted) {
                    throw new Error('서버 타이머 시작 실패')
                }
            } else {
                console.log('일시정지된 타이머 재개')
            }

            // 6. 서버에 작업 시작 요청 (안전한 버전)
            console.log('서버에 작업 시작 요청 전송')
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
                showNotification('작업이 시작되었습니다', 'success')
                console.log('작업 시작 성공')

                // 텔레그램 시작 알림 로그
                if (!wasTimerPaused && state.settings.telegramEnabled) {
                    actions.addLog('텔레그램 시작 알림이 전송됩니다.')
                }
            } else {
                throw new Error('서버에서 작업 시작을 거부했습니다')
            }
        } catch (error) {
            console.error('작업 시작 오류 상세:', error)
            console.error('에러 타입:', typeof error)
            console.error('에러 이름:', error.name)
            console.error('에러 메시지:', error.message)
            console.error('에러 스택:', error.stack)

            // 실패 시 상태 복원
            actions.setRunning(false)
            if (state.isPaused) {
                actions.setPaused(true)
            }

            // 에러 메시지 처리 (resetServerTimer 제거)
            const errorMessage = error.message || '알 수 없는 오류가 발생했습니다'
            actions.addLog(`작업 시작 실패: ${errorMessage}`)
            showNotification(`작업 시작에 실패했습니다: ${errorMessage}`, 'error')

            // 디버깅용 - resetServerTimer는 호출하지 않음
            console.log('에러 처리 완료, resetServerTimer 호출하지 않음')
        }
    }

    const handleStop = async () => {
        console.log('중지 버튼 클릭됨')

        if (!state.isRunning) {
            console.log('실행 중이 아님')
            actions.addLog('실행 중인 작업이 없습니다.')
            return
        }

        try {
            // 1. 서버 타이머 일시정지
            await stopServerTimer()

            // 2. 서버에 작업 중지 요청
            console.log('서버에 작업 중지 요청 전송')
            const success = await appService.stopOperation()

            if (success) {
                actions.setRunning(false)
                actions.setPaused(true)
                actions.addLog('작업이 일시 중지되었습니다.')
                showNotification('작업이 중지되었습니다', 'info')
                console.log('작업 중지 성공')
            } else {
                throw new Error('서버에서 작업 중지를 거부했습니다')
            }
        } catch (error) {
            console.error('작업 중지 오류:', error)

            // 에러가 나도 클라이언트 상태는 중지
            actions.setRunning(false)
            actions.setPaused(true)

            const errorMessage = error.message || '알 수 없는 오류가 발생했습니다'
            actions.addLog(`작업 중지 중 오류: ${errorMessage}`)
            showNotification(`작업 중지 중 오류가 발생했습니다: ${errorMessage}`, 'error')
        }
    }

    const handleReset = async () => {
        console.log('리셋 버튼 클릭됨')

        if (state.isRunning) {
            console.log('실행 중에는 리셋 불가')
            actions.addLog('작업 중에는 재설정할 수 없습니다.')
            return
        }

        try {
            // 1. 서버 타이머 리셋 (일단 제거해서 테스트)
            console.log('서버 타이머 리셋 건너뜀 (디버깅용)')
            // await resetServerTimer()

            // 2. 서버에 리셋 요청
            console.log('서버에 리셋 요청 전송')
            const success = await appService.resetSettings()

            if (success) {
                actions.resetAll()

                // 수동으로 타이머 시간 재설정
                const hours = getHoursFromTimeOption(state.currentTimeOption)
                actions.setCountdownTime(hours * 3600)

                actions.addLog('모든 설정이 초기화되었습니다.')
                showNotification('설정이 초기화되었습니다', 'success')
                console.log('리셋 성공')
            } else {
                throw new Error('서버에서 리셋을 거부했습니다')
            }
        } catch (error) {
            console.error('재설정 오류:', error)

            // 에러가 나도 클라이언트 상태는 리셋
            actions.resetAll()

            const errorMessage = error.message || '알 수 없는 오류가 발생했습니다'
            actions.addLog(`재설정 중 오류: ${errorMessage}`)
            showNotification(`재설정 중 오류가 발생했습니다: ${errorMessage}`, 'error')
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
