import { useState } from 'react'
import { X } from 'lucide-react'
import { QUEST_CATEGORIES, QUEST_PRIORITIES, CATEGORY_NAMES, PRIORITY_LABELS } from '@constants/appConstants'
import Button from '@components/Common/Button'
import styles from './QuestModal.module.css'

export default function QuestModal({ onClose, onSubmit }) {
    const [formData, setFormData] = useState({
        title: '',
        category: QUEST_CATEGORIES.GAME,
        priority: QUEST_PRIORITIES.MEDIUM,
        difficulty: 3
    })

    const handleSubmit = (e) => {
        e.preventDefault()

        if (!formData.title.trim()) {
            return
        }

        onSubmit({
            ...formData,
            title: formData.title.trim(),
            difficulty: parseInt(formData.difficulty)
        })
    }

    const handleChange = (field, value) => {
        setFormData(prev => ({
            ...prev,
            [field]: value
        }))
    }

    const handleOverlayClick = (e) => {
        if (e.target === e.currentTarget) {
            onClose()
        }
    }

    return (
        <div className={styles.questModal} onClick={handleOverlayClick}>
            <div className={styles.questModalContent}>
                <div className={styles.questModalHeader}>
                    <h2>새 퀘스트 추가</h2>
                    <button
                        className={styles.closeButton}
                        onClick={onClose}
                        aria-label="모달 닫기"
                    >
                        <X size={20} />
                    </button>
                </div>

                <form onSubmit={handleSubmit}>
                    <div className={styles.formGroup}>
                        <label className={styles.formLabel}>퀘스트 이름</label>
                        <input
                            type="text"
                            className={styles.formInput}
                            value={formData.title}
                            onChange={(e) => handleChange('title', e.target.value)}
                            placeholder="퀘스트 제목을 입력하세요"
                            required
                            autoFocus
                        />
                    </div>

                    <div className={styles.formGroup}>
                        <label className={styles.formLabel}>카테고리</label>
                        <select
                            className={styles.formSelect}
                            value={formData.category}
                            onChange={(e) => handleChange('category', e.target.value)}
                        >
                            {Object.entries(CATEGORY_NAMES).map(([key, name]) => (
                                <option key={key} value={key}>{name}</option>
                            ))}
                        </select>
                    </div>

                    <div className={styles.formGroup}>
                        <label className={styles.formLabel}>우선순위</label>
                        <select
                            className={styles.formSelect}
                            value={formData.priority}
                            onChange={(e) => handleChange('priority', e.target.value)}
                        >
                            {Object.entries(PRIORITY_LABELS).map(([key, label]) => (
                                <option key={key} value={key}>{label}</option>
                            ))}
                        </select>
                    </div>

                    <div className={styles.formGroup}>
                        <label className={styles.formLabel}>난이도</label>
                        <select
                            className={styles.formSelect}
                            value={formData.difficulty}
                            onChange={(e) => handleChange('difficulty', e.target.value)}
                        >
                            <option value="1">⭐ (매우 쉬움)</option>
                            <option value="2">⭐⭐ (쉬움)</option>
                            <option value="3">⭐⭐⭐ (보통)</option>
                            <option value="4">⭐⭐⭐⭐ (어려움)</option>
                            <option value="5">⭐⭐⭐⭐⭐ (매우 어려움)</option>
                        </select>
                    </div>

                    <div className={styles.questModalActions}>
                        <Button
                            type="button"
                            variant="secondary"
                            onClick={onClose}
                        >
                            취소
                        </Button>
                        <Button
                            type="submit"
                            variant="primary"
                            disabled={!formData.title.trim()}
                        >
                            퀘스트 추가
                        </Button>
                    </div>
                </form>
            </div>
        </div>
    )
}