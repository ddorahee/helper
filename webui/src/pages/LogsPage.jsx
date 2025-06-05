import { useState, useEffect } from 'react'
import { RefreshCw, Trash2, Filter } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { appService } from '@services/appService'
import Card from '@components/Common/Card'
import Button from '@components/Common/Button'
import Toggle from '@components/Common/Toggle'
import styles from './LogsPage.module.css'

export default function LogsPage() {
    const { state, actions } = useApp()
    const [logs, setLogs] = useState([])
    const [loading, setLoading] = useState(false)
    const [autoRefresh, setAutoRefresh] = useState(true)
    const [showDebug, setShowDebug] = useState(false)
    const [filter, setFilter] = useState('')

    // 로그 불러오기
    const loadLogs = async () => {
        setLoading(true)
        try {
            const data = await appService.getLogs()
            setLogs(data.logs || [])
        } catch (error) {
            actions.addLog('로그를 불러오는 중 오류가 발생했습니다.')
        } finally {
            setLoading(false)
        }
    }

    // 로그 지우기
    const clearLogs = async () => {
        if (!confirm('모든 로그를 삭제하시겠습니까?')) return

        try {
            await appService.clearLogs()
            setLogs([])
            actions.addLog('로그가 삭제되었습니다.')
        } catch (error) {
            actions.addLog('로그 삭제 중 오류가 발생했습니다.')
        }
    }

    // 자동 새로고침
    useEffect(() => {
        loadLogs()

        let interval = null
        if (autoRefresh) {
            interval = setInterval(loadLogs, 10000) // 10초마다
        }

        return () => {
            if (interval) clearInterval(interval)
        }
    }, [autoRefresh])

    // 로그 필터링
    const filteredLogs = logs.filter(log => {
        if (!showDebug && isDebugLog(log)) return false
        if (filter && !log.toLowerCase().includes(filter.toLowerCase())) return false
        return true
    })

    // 디버그 로그 판단
    const isDebugLog = (log) => {
        const lowerLog = log.toLowerCase()
        return lowerLog.includes('debug') || lowerLog.includes('초기화') ||
            lowerLog.includes('설정') || lowerLog.includes('디버그')
    }

    // 로그 레벨 판단
    const getLogLevel = (log) => {
        const lowerLog = log.toLowerCase()
        if (lowerLog.includes('error') || lowerLog.includes('오류') || lowerLog.includes('실패')) {
            return 'error'
        } else if (lowerLog.includes('warn') || lowerLog.includes('경고')) {
            return 'warning'
        } else if (isDebugLog(log)) {
            return 'debug'
        }
        return 'info'
    }

    return (
        <div className={styles.logsPage}>
            <Card
                title="로그 기록"
                className={styles.logsCard}
                headerContent={
                    <div className={styles.logsActions}>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={loadLogs}
                            disabled={loading}
                            icon={<RefreshCw size={16} />}
                        >
                            새로고침
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={clearLogs}
                            icon={<Trash2 size={16} />}
                            className={styles.dangerButton}
                        >
                            지우기
                        </Button>
                    </div>
                }
            >
                <div className={styles.logsContent}>
                    <div className={styles.logsContainer}>
                        {loading ? (
                            <div className={styles.logPlaceholder}>로그를 불러오는 중...</div>
                        ) : filteredLogs.length === 0 ? (
                            <div className={styles.logPlaceholder}>표시할 로그가 없습니다.</div>
                        ) : (
                            filteredLogs.map((log, index) => (
                                <div
                                    key={`${log}-${index}`}
                                    className={`${styles.logEntry} ${styles[getLogLevel(log)]}`}
                                >
                                    {log}
                                </div>
                            ))
                        )}
                    </div>
                </div>
            </Card>

            <Card title="로그 설정" className={styles.settingsCard}>
                <div className={styles.settingsGrid}>
                    <div className={styles.settingItem}>
                        <Toggle
                            checked={autoRefresh}
                            onChange={setAutoRefresh}
                            label="자동 새로고침 (10초)"
                        />
                    </div>
                    <div className={styles.settingItem}>
                        <Toggle
                            checked={showDebug}
                            onChange={setShowDebug}
                            label="디버그 메시지 표시"
                        />
                    </div>
                    <div className={styles.filterGroup}>
                        <label className={styles.filterLabel}>
                            <Filter size={16} />
                            필터
                        </label>
                        <input
                            type="text"
                            value={filter}
                            onChange={(e) => setFilter(e.target.value)}
                            placeholder="필터링할 텍스트 입력"
                            className={styles.filterInput}
                        />
                    </div>
                </div>
            </Card>
        </div>
    )
}