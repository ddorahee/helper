import { Routes, Route } from 'react-router-dom'
import { AppProvider } from '@/contexts/AppContext'
import Layout from '@components/Layout/Layout'
import MainPage from '@pages/MainPage'
import KeyMappingPage from '@pages/KeyMappingPage' // 키 맵핑 페이지 추가
import LogsPage from '@pages/LogsPage'
import SettingsPage from '@pages/SettingsPage'
import NotificationProvider from '@components/Notification/NotificationProvider'

function App() {
    return (
        <AppProvider>
            <NotificationProvider>
                <Layout>
                    <Routes>
                        <Route path="/" element={<MainPage />} />
                        <Route path="/keymapping" element={<KeyMappingPage />} />
                        <Route path="/logs" element={<LogsPage />} />
                        <Route path="/settings" element={<SettingsPage />} />
                    </Routes>
                </Layout>
            </NotificationProvider>
        </AppProvider>
    )
}

export default App