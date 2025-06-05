import { clsx } from 'clsx'
import styles from './Button.module.css'

export default function Button({
    children,
    variant = 'primary',
    size = 'md',
    icon,
    loading = false,
    disabled = false,
    className,
    onClick,
    ...props
}) {
    const handleClick = (e) => {
        if (disabled || loading) return
        onClick?.(e)
    }

    return (
        <button
            className={clsx(
                styles.button,
                styles[variant],
                styles[size],
                {
                    [styles.loading]: loading,
                    [styles.disabled]: disabled || loading,
                    [styles.withIcon]: !!icon
                },
                className
            )}
            onClick={handleClick}
            disabled={disabled || loading}
            {...props}
        >
            {loading ? (
                <div className={styles.spinner} />
            ) : (
                <>
                    {icon && <span className={styles.icon}>{icon}</span>}
                    {children && <span className={styles.text}>{children}</span>}
                </>
            )}
        </button>
    )
}