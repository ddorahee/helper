import { useApp } from '@/contexts/AppContext'
import Sidebar from './Sidebar'
import styles from './Layout.module.css'

export default function Layout({ children }) {
    const { state } = useApp()

    return (
        <div className={styles.appContainer} data-theme={state.settings.darkMode ? 'dark' : 'light'}>
            <Sidebar />
            <main className={styles.content}>
                {children}
            </main>
        </div>
    )
}