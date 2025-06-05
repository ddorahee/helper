import { useEffect } from 'react'
import { useApp } from '@/contexts/AppContext'
import Card from '@components/Common/Card'
import styles from './Timer.module.css'

export default function Timer() {
    const { state, actions, getHoursFromTimeOption } = useApp()

    // 카운트다운 로직
    useEffect(() => {
        let interval = null

        if (state.isRunning && !state.isPaused) {
            interval = setInterval(() => {
                actions.setCountdownTime(prev => {
                    if (prev <= 0) {
                        actions.setRunning(false)
                        actions.addLog('설정한 시간이 경과하여 자동으로 종료되었습니다.')
                        return 0
                    }
                    return prev - 1
                })
            }, 1000)
        }

        return () => {
            if (interval) clearInterval(interval)
        }
    }, [state.isRunning, state.isPaused])

    // 시간 포맷팅
    const formatTime = (seconds) => {
        const hours = Math.floor(seconds / 3600)
        const minutes = Math.floor((seconds % 3600) / 60)
        const secs = Math.floor(seconds % 60)

        return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
    }

    return (
        <Card title="남은 시간" className={styles.timerCard}>
            <div className={`${styles.timerDisplay} ${state.isRunning ? styles.running : ''}`}>
                {formatTime(state.countdownTime)}
            </div>
        </Card>
    )
}