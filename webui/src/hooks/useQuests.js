import { useState, useEffect } from 'react'
import { QUEST_CATEGORIES, QUEST_PRIORITIES } from '@constants/appConstants'

export function useQuests() {
    const [quests, setQuests] = useState([])
    const [currentFilter, setCurrentFilter] = useState('all')
    const [questIdCounter, setQuestIdCounter] = useState(1)

    // 로컬 스토리지에서 퀘스트 로드
    useEffect(() => {
        loadQuestsFromStorage()
    }, [])

    // 퀘스트가 변경될 때 로컬 스토리지에 저장
    useEffect(() => {
        if (quests.length > 0) {
            saveQuestsToStorage()
        }
    }, [quests])

    const loadQuestsFromStorage = () => {
        try {
            const savedQuests = localStorage.getItem('doumi-quests')
            const savedCounter = localStorage.getItem('doumi-quest-counter')

            if (savedQuests) {
                setQuests(JSON.parse(savedQuests))
            } else {
                // 기본 퀘스트들
                const defaultQuests = [
                    {
                        id: 1,
                        title: "매크로 자동화 스크립트 개선",
                        category: QUEST_CATEGORIES.GAME,
                        priority: QUEST_PRIORITIES.HIGH,
                        difficulty: 3,
                        completed: false,
                        createdAt: new Date().toISOString()
                    },
                    {
                        id: 2,
                        title: "장비 강화 재료 정리",
                        category: QUEST_CATEGORIES.GAME,
                        priority: QUEST_PRIORITIES.MEDIUM,
                        difficulty: 2,
                        completed: true,
                        createdAt: new Date().toISOString()
                    },
                    {
                        id: 3,
                        title: "길드 활동 참여",
                        category: QUEST_CATEGORIES.DAILY,
                        priority: QUEST_PRIORITIES.LOW,
                        difficulty: 1,
                        completed: false,
                        createdAt: new Date().toISOString()
                    }
                ]
                setQuests(defaultQuests)
                setQuestIdCounter(4)
            }

            if (savedCounter) {
                setQuestIdCounter(parseInt(savedCounter))
            }
        } catch (error) {
            console.error('Failed to load quests:', error)
        }
    }

    const saveQuestsToStorage = () => {
        try {
            localStorage.setItem('doumi-quests', JSON.stringify(quests))
            localStorage.setItem('doumi-quest-counter', questIdCounter.toString())
        } catch (error) {
            console.error('Failed to save quests:', error)
        }
    }

    const addQuest = (questData) => {
        const newQuest = {
            id: questIdCounter,
            ...questData,
            completed: false,
            createdAt: new Date().toISOString()
        }

        setQuests(prev => [...prev, newQuest])
        setQuestIdCounter(prev => prev + 1)
    }

    const toggleQuest = (questId) => {
        setQuests(prev => prev.map(quest =>
            quest.id === questId
                ? { ...quest, completed: !quest.completed }
                : quest
        ))
    }

    const deleteQuest = (questId) => {
        setQuests(prev => prev.filter(quest => quest.id !== questId))
    }

    const getQuestStats = () => {
        const total = quests.length
        const completed = quests.filter(q => q.completed).length
        const active = total - completed
        const completionRate = total > 0 ? Math.round((completed / total) * 100) : 0
        const experience = completed * 125 + active * 25

        return {
            total,
            completed,
            active,
            completionRate,
            experience
        }
    }

    const getCategoryStats = () => {
        const stats = {}

        Object.values(QUEST_CATEGORIES).forEach(category => {
            const categoryQuests = quests.filter(q => q.category === category)
            const completedQuests = categoryQuests.filter(q => q.completed)

            stats[category] = {
                total: categoryQuests.length,
                completed: completedQuests.length,
                progress: categoryQuests.length > 0
                    ? (completedQuests.length / categoryQuests.length) * 100
                    : 0
            }
        })

        return stats
    }

    const getFilteredQuests = () => {
        if (currentFilter === 'all') return quests
        return quests.filter(quest => quest.category === currentFilter)
    }

    return {
        quests: getFilteredQuests(),
        allQuests: quests,
        currentFilter,
        setCurrentFilter,
        addQuest,
        toggleQuest,
        deleteQuest,
        getQuestStats,
        getCategoryStats
    }
}