import { clsx } from 'clsx'
import styles from './Card.module.css'

export default function Card({ title, headerContent, children, className, ...props }) {
    return (
        <div className={clsx(styles.card, className)} {...props}>
            {(title || headerContent) && (
                <div className={styles.cardHeader}>
                    {title && <h2 className={styles.cardTitle}>{title}</h2>}
                    {headerContent && <div className={styles.cardHeaderContent}>{headerContent}</div>}
                </div>
            )}
            <div className={styles.cardContent}>
                {children}
            </div>
        </div>
    )
}