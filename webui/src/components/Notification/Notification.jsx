import { useEffect, useState } from 'react'
import { X, CheckCircle, AlertCircle, Info, AlertTriangle } from 'lucide-react'
import { NOTIFICATION_TYPES } from '@constants/appConstants'
import styles from './Notification.module.css'

const iconMap = {
    [NOTIFICATION_TYPES.SUCCESS]: CheckCircle,
    [NOTIFICATION_TYPES.ERROR]: AlertCircle,
    [NOTIFICATION_TYPES.WARNING]: AlertTriangle,
    [NOTIFICATION_TYPES.INFO]: Info
}

export default function Notification({ id, message, type, duration, onClose }) {
    const [isVisible, setIsVisible] = useState(false)
    const [isLeaving, setIsLeaving] = useState(false)

    const Icon = iconMap[type] || Info

    useEffect(() => {
        // 입장 애니메이션
        setTimeout(() => setIsVisible(true), 100)

        // 자동 닫기
        const timer = setTimeout(() => {
            handleClose()
        }, duration - 300)

        return () => clearTimeout(timer)
    }, [duration])

    const handleClose = () => {
        setIsLeaving(true)
        setTimeout(() => {
            onClose()
        }, 300)
    }

    return (
        <div
            className={`
        ${styles.notification} 
        ${styles[type]} 
        ${isVisible ? styles.visible : ''} 
        ${isLeaving ? styles.leaving : ''}
      `}
        >
            <div className={styles.iconContainer}>
                <Icon size={20} />
            </div>

            <div className={styles.content}>
                <span className={styles.message}>{message}</span>
            </div>

            <button
                className={styles.closeButton}
                onClick={handleClose}
                aria-label="알림 닫기"
            >
                <X size={16} />
            </button>
        </div>
    )
}