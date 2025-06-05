import { useApp } from '@/contexts/AppContext'
import { TIME_OPTIONS } from '@constants/appConstants'
import styles from './TimeSelector.module.css'

const timeOptions = [
    { value: TIME_OPTIONS.ONE_HOUR, label: '1시간', hours: 1 },
    { value: TIME_OPTIONS.TWO_HOUR, label: '2시간', hours: 2 },
    { value: TIME_OPTIONS.THREE_HOUR, label: '3시간', hours: 3 },
    { value: TIME_OPTIONS.FOUR_HOUR, label: '4시간', hours: 4 }
]

export default function TimeSelector() {
    const { state, actions, getHoursFromTimeOption, formatTimeOption } = useApp()

    const handleTimeChange = (option) => {
        actions.setTimeOption(option)

        // 타이머가 실행 중이 아니라면 새 시간으로 표시 업데이트
        if (!state.isRunning && !state.isPaused) {
            const hours = getHoursFromTimeOption(option)
            actions.setCountdownTime(hours * 60 * 60)
        }

        actions.addLog(`${formatTimeOption(option)} 실행 설정됨`)
    }

    return (
        <div className={styles.timeOptions}>
            {timeOptions.map(({ value, label, hours }) => (
                <label key={value} className={styles.timeOption}>
                    <input
                        type="radio"
                        name="time"
                        value={value}
                        checked={state.currentTimeOption === value}
                        onChange={() => handleTimeChange(value)}
                    />
                    <div className={styles.timeCard}>
                        <span className={styles.timeValue}>{hours}</span>
                        <span className={styles.timeUnit}>시간</span>
                    </div>
                </label>
            ))}
        </div>
    )
}