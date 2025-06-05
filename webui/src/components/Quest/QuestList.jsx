import { Trash2 } from 'lucide-react'
import { CATEGORY_ICONS, CATEGORY_NAMES, PRIORITY_LABELS } from '@constants/appConstants'
import styles from './QuestList.module.css'

export default function QuestList({ quests, currentFilter, onToggle, onDelete }) {
    const filteredQuests = currentFilter === 'all'
        ? quests
        : quests.filter(quest => quest.category === currentFilter)

    if (filteredQuests.length === 0) {
        return (
            <div className={styles.emptyState}>
                <p>표시할 퀘스트가 없습니다</p>
            </div>
        )
    }

    const renderDifficultyStars = (difficulty) => {
        return Array.from({ length: 5 }, (_, i) => (
            <div
                key={i}
                className={`${styles.difficultyStar} ${i < difficulty ? styles.filled : ''}`}
            />
        ))
    }

    return (
        <div className={styles.taskList}>
            {filteredQuests.map(quest => (
                <div
                    key={quest.id}
                    className={`${styles.taskItem} ${quest.completed ? styles.completed : ''}`}
                >
                    <div className={styles.taskHeader}>
                        <div
                            className={`${styles.taskCheckbox} ${quest.completed ? styles.checked : ''}`}
                            onClick={() => onToggle(quest.id)}
                        />
                        <div className={styles.taskTitle}>{quest.title}</div>
                        <div className={`${styles.taskPriority} ${styles[`priority${quest.priority.charAt(0).toUpperCase() + quest.priority.slice(1)}`]}`}>
                            {PRIORITY_LABELS[quest.priority]}
                        </div>
                    </div>

                    <div className={styles.taskMeta}>
                        <div className={styles.taskCategory}>
                            <span>{CATEGORY_ICONS[quest.category]}</span>
                            <span>{CATEGORY_NAMES[quest.category]}</span>
                        </div>

                        <div className={styles.taskDifficulty}>
                            {renderDifficultyStars(quest.difficulty)}
                        </div>

                        <div className={styles.taskActions}>
                            <button
                                className={`${styles.taskButton} ${styles.delete}`}
                                onClick={() => onDelete(quest.id)}
                                aria-label="퀘스트 삭제"
                            >
                                <Trash2 size={16} />
                            </button>
                        </div>
                    </div>
                </div>
            ))}
        </div>
    )
}