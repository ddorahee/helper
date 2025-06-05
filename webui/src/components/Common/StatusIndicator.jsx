import { clsx } from 'clsx'
import styles from './StatusIndicator.module.css'

export default function StatusIndicator({ isRunning, className }) {
    return (
        <div className={clsx(
            styles.statusIndicator,
            { [styles.running]: isRunning },
            className
        )} />
    )
}