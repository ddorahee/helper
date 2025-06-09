import { useNavigate, useLocation } from 'react-router-dom'
import {
    Home,
    FileText,
    Settings,
    LogOut,
    Package
} from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { appService } from '@services/appService'
import StatusIndicator from '@components/Common/StatusIndicator'
import styles from './Sidebar.module.css'

const navigationItems = [
    { path: '/', icon: Home, label: '메인 화면' },
    { path: '/logs', icon: FileText, label: '로그' },
    { path: '/settings', icon: Settings, label: '설정' }
]

export default function Sidebar() {
    const navigate = useNavigate()
    const location = useLocation()
    const { state, actions } = useApp()

    const handleNavigation = (path) => {
        navigate(path)
    }

    const handleExit = async () => {
        try {
            actions.addLog('프로그램을 종료합니다...')
            await appService.exit()
        } catch (error) {
            actions.addLog('종료 중 오류가 발생했습니다.')
        }
    }

    return (
        <aside className={styles.sidebar}>
            <div className={styles.sidebarHeader}>
                <div className={styles.appLogo}>
                    <Package size={36} />
                </div>
                <h1>도우미</h1>
            </div>

            <div className={styles.sidebarStatus}>
                <StatusIndicator isRunning={state.isRunning} />
                <span>{state.isRunning ? '실행 중' : '준비됨'}</span>
            </div>

            <nav className={styles.sidebarNav}>
                {navigationItems.map(({ path, icon: Icon, label }) => (
                    <button
                        key={path}
                        onClick={() => handleNavigation(path)}
                        className={`${styles.navButton} ${location.pathname === path ? styles.active : ''}`}
                    >
                        <Icon size={18} />
                        <span>{label}</span>
                    </button>
                ))}
            </nav>

            <div className={styles.sidebarFooter}>
                <button className={styles.exitButton} onClick={handleExit}>
                    <LogOut size={16} />
                    <span>종료</span>
                </button>
                <div className={styles.versionInfo}>
                    v{state.version}
                </div>
            </div>
        </aside>
    )
}