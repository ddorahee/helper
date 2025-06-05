import { ExternalLink, Users } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { MODES } from '@constants/appConstants'
import styles from './ModeSelector.module.css'

const modeOptions = [
    {
        group: '대야',
        modes: [
            { value: MODES.DAEYA_ENTER, label: '입장', icon: ExternalLink },
            { value: MODES.DAEYA_PARTY, label: '파티', icon: Users }
        ]
    },
    {
        group: '칸첸',
        modes: [
            { value: MODES.KANCHEN_ENTER, label: '입장', icon: ExternalLink },
            { value: MODES.KANCHEN_PARTY, label: '파티', icon: Users }
        ]
    }
]

export default function ModeSelector() {
    const { state, actions, getModeName } = useApp()

    const handleModeChange = (mode) => {
        actions.setMode(mode)
        actions.addLog(`${getModeName(mode)} 모드 선택됨`)
    }

    return (
        <div className={styles.modesContainer}>
            {modeOptions.map(({ group, modes }) => (
                <div key={group} className={styles.modeGroup}>
                    <h3 className={styles.groupTitle}>{group}</h3>
                    <div className={styles.modeOptions}>
                        {modes.map(({ value, label, icon: Icon }) => (
                            <label key={value} className={styles.modeOption}>
                                <input
                                    type="radio"
                                    name="mode"
                                    value={value}
                                    checked={state.currentMode === value}
                                    onChange={() => handleModeChange(value)}
                                />
                                <div className={styles.modeCard}>
                                    <Icon className={styles.modeIcon} size={24} />
                                    <span>{label}</span>
                                </div>
                            </label>
                        ))}
                    </div>
                </div>
            ))}
        </div>
    )
}