import Card from '@components/Common/Card'
import styles from './QuestStats.module.css'

export default function QuestStats({ stats }) {
    const statItems = [
        { label: '총 퀘스트', value: stats.total, color: 'primary' },
        { label: '완료됨', value: stats.completed, color: 'success' },
        { label: '진행 중', value: stats.active, color: 'warning' },
        { label: '완료율', value: `${stats.completionRate}%`, color: 'info' },
        { label: '경험치', value: stats.experience.toLocaleString(), color: 'quest' }
    ]

    return (
        <Card className={styles.statsPanel}>
            <div className={styles.statsGrid}>
                {statItems.map(({ label, value, color }) => (
                    <div key={label} className={styles.statCard}>
                        <div className={`${styles.statValue} ${styles[color]}`}>
                            {value}
                        </div>
                        <div className={styles.statLabel}>
                            {label}
                        </div>
                    </div>
                ))}
            </div>
        </Card>
    )
}