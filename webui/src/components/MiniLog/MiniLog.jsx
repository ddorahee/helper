import { useApp } from '@/contexts/AppContext'
import Card from '@components/Common/Card'
import styles from './MiniLog.module.css'

export default function MiniLog() {
    const { state } = useApp()

    // 최신 로그 메시지 가져오기
    const latestLog = state.logs.length > 0
        ? state.logs[state.logs.length - 1].message
        : '프로그램이 시작되었습니다.'

    return (
        <Card className={styles.miniLogCard}>
            <div className={styles.miniLog}>
                {latestLog}
            </div>
        </Card>
    )
}