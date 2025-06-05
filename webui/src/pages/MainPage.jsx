import { useApp } from '@/contexts/AppContext'
import Card from '@components/Common/Card'
import Timer from '@components/Timer/Timer'
import ModeSelector from '@components/ModeSelector/ModeSelector'
import TimeSelector from '@components/TimeSelector/TimeSelector'
import ControlButtons from '@components/ControlButtons/ControlButtons'
import MiniLog from '@components/MiniLog/MiniLog'
import styles from './MainPage.module.css'

export default function MainPage() {
    const { state } = useApp()

    return (
        <div className={styles.mainPage}>
            <Timer />

            <div className={styles.settingsGrid}>
                <Card title="모드 선택">
                    <ModeSelector />
                </Card>

                <Card title="자동 종료 시간">
                    <TimeSelector />
                </Card>
            </div>

            <ControlButtons />

            <MiniLog />
        </div>
    )
}