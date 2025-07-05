import { useEffect, useState, useRef } from 'react'
import { useApp } from '@/contexts/AppContext'
import Card from '@components/Common/Card'
import styles from './Timer.module.css'

export default function Timer() {
    const { state, actions } = useApp()
    const [serverTime, setServerTime] = useState({
        running: false,
        paused: false,
        remainingSeconds: 0,
        elapsedSeconds: 0,
        totalSeconds: 0,
        progress: 0,
        remainingString: '00:00:00',
        elapsedString: '00:00:00'
    })

    // 서버 폴링을 위한 ref
    const pollingIntervalRef = useRef(null)
    const isActiveRef = useRef(true)

    // 컴포넌트 마운트/언마운트 시 활성 상태 관리
    useEffect(() => {
        isActiveRef.current = true

        return () => {
            isActiveRef.current = false
            if (pollingIntervalRef.current) {
                clearInterval(pollingIntervalRef.current)
                pollingIntervalRef.current = null
            }
        }
    }, [])

    // 서버에서 정확한 시간 정보 가져오기
    const fetchServerTime = async () => {
        try {
            if (window.getServerTime && typeof window.getServerTime === 'function') {
                const timeData = await window.getServerTime()
                if (isActiveRef.current) {
                    setServerTime(timeData)

                    // 클라이언트 상태와 동기화
                    if (timeData.running !== state.isRunning) {
                        actions.setRunning(timeData.running)
                    }
                    if (timeData.paused !== state.isPaused) {
                        actions.setPaused(timeData.paused)
                    }

                    // 남은 시간을 클라이언트 상태에도 반영
                    if (timeData.remainingSeconds !== state.countdownTime) {
                        actions.setCountdownTime(timeData.remainingSeconds)
                    }
                }
            }
        } catch (error) {
            console.warn('서버 시간 조회 실패, 클라이언트 타이머 사용:', error)
        }
    }

    // 정기적으로 서버 시간 폴링 (1초마다)
    useEffect(() => {
        // 초기 서버 시간 조회
        fetchServerTime()

        // 1초마다 서버 시간 조회 (창이 최소화되어도 정확함)
        pollingIntervalRef.current = setInterval(() => {
            if (isActiveRef.current) {
                fetchServerTime()
            }
        }, 1000)

        return () => {
            if (pollingIntervalRef.current) {
                clearInterval(pollingIntervalRef.current)
                pollingIntervalRef.current = null
            }
        }
    }, [])

    // 서버에서 오는 실시간 타이머 이벤트 리스너
    useEffect(() => {
        const handleTimerUpdate = (event) => {
            if (isActiveRef.current && event.detail) {
                const { remaining } = event.detail
                setServerTime(prev => ({
                    ...prev,
                    remainingSeconds: remaining,
                    remainingString: formatTime(remaining)
                }))

                // 클라이언트 상태도 업데이트
                actions.setCountdownTime(remaining)
            }
        }

        const handleTimerComplete = (event) => {
            if (isActiveRef.current) {
                console.log('서버에서 타이머 완료 이벤트 받음:', event.detail)
                actions.setRunning(false)
                actions.setPaused(false)
                actions.setCountdownTime(0)
                actions.addLog('설정한 시간이 완료되어 자동으로 종료되었습니다.')

                setServerTime(prev => ({
                    ...prev,
                    running: false,
                    remainingSeconds: 0,
                    remainingString: '00:00:00'
                }))
            }
        }

        // 이벤트 리스너 등록
        window.addEventListener('timerUpdate', handleTimerUpdate)
        window.addEventListener('timerComplete', handleTimerComplete)

        return () => {
            window.removeEventListener('timerUpdate', handleTimerUpdate)
            window.removeEventListener('timerComplete', handleTimerComplete)
        }
    }, [actions])

    // 클라이언트 fallback 타이머 (서버 연결이 안될 때만 사용)
    useEffect(() => {
        let fallbackInterval = null

        // 서버 타이머가 작동하지 않을 때만 클라이언트 타이머 사용
        if (state.isRunning && !state.isPaused && !serverTime.running) {
            console.log('서버 타이머 연결 실패, 클라이언트 fallback 타이머 사용')

            fallbackInterval = setInterval(() => {
                if (isActiveRef.current) {
                    actions.setCountdownTime(prev => {
                        if (prev <= 0) {
                            actions.setRunning(false)
                            actions.addLog('설정한 시간이 경과하여 자동으로 종료되었습니다.')
                            return 0
                        }
                        return prev - 1
                    })
                }
            }, 1000)
        }

        return () => {
            if (fallbackInterval) {
                clearInterval(fallbackInterval)
            }
        }
    }, [state.isRunning, state.isPaused, serverTime.running, actions])

    // 시간 포맷팅 함수
    const formatTime = (seconds) => {
        const hours = Math.floor(seconds / 3600)
        const minutes = Math.floor((seconds % 3600) / 60)
        const secs = Math.floor(seconds % 60)

        return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
    }

    // 표시할 시간 결정 (서버 시간 우선, fallback은 클라이언트 시간)
    const displayTime = serverTime.running ? serverTime.remainingString : formatTime(state.countdownTime)
    const isServerTimer = serverTime.running || (state.isRunning && serverTime.remainingSeconds > 0)

    return (
        <Card
            title={
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    남은 시간
                    {isServerTimer && (
                        <span style={{
                            fontSize: '0.7rem',
                            color: 'var(--success-color)',
                            background: 'rgba(16, 185, 129, 0.1)',
                            padding: '2px 6px',
                            borderRadius: '4px',
                            fontWeight: '500'
                        }}>
                            서버 기반
                        </span>
                    )}
                </div>
            }
            className={styles.timerCard}
        >
            <div className={`${styles.timerDisplay} ${(state.isRunning || serverTime.running) ? styles.running : ''}`}>
                {displayTime}
            </div>

            {/* 진행률 표시 (서버 기반인 경우) */}
            {isServerTimer && serverTime.totalSeconds > 0 && (
                <div className={styles.progressContainer}>
                    <div className={styles.progressBar}>
                        <div
                            className={styles.progressFill}
                            style={{
                                width: `${((serverTime.totalSeconds - serverTime.remainingSeconds) / serverTime.totalSeconds * 100)}%`
                            }}
                        />
                    </div>
                    <div className={styles.progressText}>
                        {Math.round((serverTime.totalSeconds - serverTime.remainingSeconds) / serverTime.totalSeconds * 100)}% 완료
                    </div>
                </div>
            )}

            {/* 디버그 정보 (개발 모드에서만) */}
            {process.env.NODE_ENV === 'development' && (
                <div className={styles.debugInfo}>
                    <small>
                        서버: {serverTime.running ? '실행중' : '중지'} |
                        클라이언트: {state.isRunning ? '실행중' : '중지'} |
                        {isServerTimer ? '서버 타이머' : '클라이언트 타이머'}
                    </small>
                </div>
            )}
        </Card>
    )
}
