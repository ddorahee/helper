import { clsx } from 'clsx'
import styles from './Toggle.module.css'

export default function Toggle({
    checked,
    onChange,
    label,
    disabled = false,
    className,
    ...props
}) {
    const handleChange = (e) => {
        if (disabled) return
        onChange?.(e.target.checked)
    }

    return (
        <label className={clsx(styles.toggleContainer, { [styles.disabled]: disabled }, className)}>
            <div className={styles.switch}>
                <input
                    type="checkbox"
                    checked={checked}
                    onChange={handleChange}
                    disabled={disabled}
                    className={styles.input}
                    {...props}
                />
                <span className={clsx(styles.slider, { [styles.checked]: checked })} />
            </div>
            {label && <span className={styles.label}>{label}</span>}
        </label>
    )
}