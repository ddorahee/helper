import { createContext, useState, useCallback } from 'react'
import Notification from './Notification'
import { NOTIFICATION_TYPES } from '@constants/appConstants'

export const NotificationContext = createContext()

export default function NotificationProvider({ children }) {
    const [notifications, setNotifications] = useState([])

    const showNotification = useCallback((message, type = NOTIFICATION_TYPES.INFO, duration = 3000) => {
        const id = Date.now()
        const notification = {
            id,
            message,
            type,
            duration
        }

        setNotifications(prev => [...prev, notification])

        // 자동 제거
        setTimeout(() => {
            setNotifications(prev => prev.filter(n => n.id !== id))
        }, duration)
    }, [])

    const removeNotification = useCallback((id) => {
        setNotifications(prev => prev.filter(n => n.id !== id))
    }, [])

    const value = {
        showNotification,
        removeNotification
    }

    return (
        <NotificationContext.Provider value={value}>
            {children}
            <div className="notification-container">
                {notifications.map(notification => (
                    <Notification
                        key={notification.id}
                        {...notification}
                        onClose={() => removeNotification(notification.id)}
                    />
                ))}
            </div>
        </NotificationContext.Provider>
    )
}