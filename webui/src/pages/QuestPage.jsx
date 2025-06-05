import { useState, useEffect } from 'react'
import { Plus, User } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { useQuests } from '@hooks/useQuests'
import { QUEST_CATEGORIES, CATEGORY_ICONS, CATEGORY_NAMES } from '@constants/appConstants'
import Card from '@components/Common/Card'
import Button from '@components/Common/Button'
import QuestModal from '@components/Quest/QuestModal'
import QuestList from '@components/Quest/QuestList'
import QuestStats from '@components/Quest/QuestStats'
import CategoryGrid from '@components/Quest/CategoryGrid'
import styles from './QuestPage.module.css'

export default function QuestPage() {
    const { actions } = useApp()
    const {
        quests,
        currentFilter,
        setCurrentFilter,
        addQuest,
        toggleQuest,
        deleteQuest,
        getQuestStats,
        getCategoryStats
    } = useQuests()

    const [showModal, setShowModal] = useState(false)

    useEffect(() => {
        actions.addLog('퀘스트 시스템에 접속했습니다.')
    }, [])

    const handleAddQuest = (questData) => {
        addQuest(questData)
        setShowModal(false)
        actions.addLog(`새 퀘스트가 추가되었습니다: ${questData.title}`)
    }

    const handleToggleQuest = (questId) => {
        const quest = quests.find(q => q.id === questId)
        if (quest) {
            toggleQuest(questId)
            const statusText = quest.completed ? '진행 중' : '완료됨'
            actions.addLog(`퀘스트 상태 변경: "${quest.title}" - ${statusText}`)
        }
    }

    const handleDeleteQuest = (questId) => {
        const quest = quests.find(q => q.id === questId)
        if (quest && confirm(`"${quest.title}" 퀘스트를 삭제하시겠습니까?`)) {
            deleteQuest(questId)
            actions.addLog(`퀘스트가 삭제되었습니다: ${quest.title}`)
        }
    }

    const stats = getQuestStats()
    const categoryStats = getCategoryStats()

    return (
        <div className={styles.questPage}>
            {/* 퀘스트 헤더 */}
            <div className={styles.questHeader}>
                <div className={styles.questTitle}>
                    <h1>⚔️ 퀘스트 로그</h1>
                </div>
                <div className={styles.characterInfo}>
                    <div className={styles.characterAvatar}>
                        <User size={24} />
                    </div>
                    <div className={styles.characterStats}>
                        <div className={styles.characterName}>밍기</div>
                        <div className={styles.characterLevel}>레벨 42 • 아케나</div>
                    </div>
                </div>
            </div>

            {/* 카테고리 섹션 */}
            <Card title="🏆 카테고리" className={styles.categoriesCard}>
                <CategoryGrid
                    categoryStats={categoryStats}
                    currentFilter={currentFilter}
                    onFilterChange={setCurrentFilter}
                />
            </Card>

            {/* 퀘스트 섹션 */}
            <Card
                title="📋 현재 퀘스트"
                className={styles.questsCard}
                headerContent={
                    <Button
                        onClick={() => setShowModal(true)}
                        icon={<Plus size={16} />}
                        className={styles.addButton}
                    >
                        새 퀘스트
                    </Button>
                }
            >
                <QuestList
                    quests={quests}
                    currentFilter={currentFilter}
                    onToggle={handleToggleQuest}
                    onDelete={handleDeleteQuest}
                />
            </Card>

            {/* 통계 패널 */}
            <QuestStats stats={stats} />

            {/* 퀘스트 추가 모달 */}
            {showModal && (
                <QuestModal
                    onClose={() => setShowModal(false)}
                    onSubmit={handleAddQuest}
                />
            )}
        </div>
    )
}