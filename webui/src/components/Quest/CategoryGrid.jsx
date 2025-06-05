import { QUEST_CATEGORIES, CATEGORY_ICONS, CATEGORY_NAMES } from '@constants/appConstants'
import styles from './CategoryGrid.module.css'

export default function CategoryGrid({ categoryStats, currentFilter, onFilterChange }) {
    const categories = [
        { key: 'all', name: '전체', icon: '📊' },
        ...Object.values(QUEST_CATEGORIES).map(key => ({
            key,
            name: CATEGORY_NAMES[key],
            icon: CATEGORY_ICONS[key]
        }))
    ]

    const getCategoryData = (key) => {
        if (key === 'all') {
            const totalStats = Object.values(categoryStats).reduce(
                (acc, stat) => ({
                    total: acc.total + stat.total,
                    completed: acc.completed + stat.completed
                }),
                { total: 0, completed: 0 }
            )

            return {
                total: totalStats.total,
                completed: totalStats.completed,
                progress: totalStats.total > 0 ? (totalStats.completed / totalStats.total) * 100 : 0
            }
        }

        return categoryStats[key] || { total: 0, completed: 0, progress: 0 }
    }

    return (
        <div className={styles.categoriesGrid}>
            {categories.map(({ key, name, icon }) => {
                const data = getCategoryData(key)
                const isActive = currentFilter === key

                return (
                    <div
                        key={key}
                        className={`${styles.categoryCard} ${isActive ? styles.active : ''}`}
                        onClick={() => onFilterChange(key)}
                    >
                        <div className={styles.categoryHeader}>
                            <div className={styles.categoryIcon}>{icon}</div>
                            <div className={styles.categoryCount}>{data.total}</div>
                        </div>

                        <div className={styles.categoryName}>{name}</div>

                        <div className={styles.categoryProgress}>
                            <div className={styles.progressBar}>
                                <div
                                    className={styles.progressFill}
                                    style={{ width: `${data.progress}%` }}
                                />
                            </div>
                            <span className={styles.progressText}>
                                {data.completed}/{data.total}
                            </span>
                        </div>
                    </div>
                )
            })}
        </div>
    )
}