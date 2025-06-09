import { useState, useEffect, useCallback } from 'react'
import { RefreshCw, Trash2, Filter } from 'lucide-react'
import { useApp } from '@/contexts/AppContext'
import { appService } from '@services/appService'
import Card from '@components/Common/Card'
import Button from '@components/Common/Button'
import Toggle from '@components/Common/Toggle'
import styles from './LogsPage.module.css'

export default function LogsPage() {
    const { state, actions } = useApp()
    const [logs, setLogs] = useState([]) // 빈 배열로 초기화
    const [loading, setLoading] = useState(false)
    const [autoRefresh, setAutoRefresh] = useState(true)
    const [showDebug, setShowDebug] = useState(false)
    const [filter, setFilter] = useState('')

    // 로그 불러오기 함수를 useCallback으로 메모이제이션
    const loadLogs = useCallback(async () => {
        if (loading) return // 중복 요청 방지

        setLoading(true)
        try {
            console.log('로그 데이터 요청 시작...')
            const data = await appService.getLogs()
            console.log('받은 로그 데이터:', data)

            if (data && data.logs && Array.isArray(data.logs)) {
                setLogs(data.logs)
                console.log('로그 설정 완료, 개수:', data.logs.length)
            } else {
                console.warn('로그 데이터 형식이 올바르지 않음:', data)
                setLogs([
                    '로그 데이터를 불러올 수 없습니다.',
                    '서버 응답 형식이 올바르지 않습니다.',
                    JSON.stringify(data)
                ])
            }
        } catch (error) {
            console.error('로그 로딩 오류:', error)
            actions.addLog('로그를 불러오는 중 오류가 발생했습니다.')
            setLogs([
                '로그를 불러오는 중 오류가 발생했습니다.',
                `오류 내용: ${error.message || error}`,
                '네트워크 연결을 확인하고 다시 시도해주세요.'
            ])
        } finally {
            setLoading(false)
        }
    }, [loading, actions])

    // 로그 지우기
    const clearLogs = useCallback(async () => {
        if (!confirm('모든 로그를 삭제하시겠습니까?')) return

        try {
            const success = await appService.clearLogs()
            if (success) {
                setLogs([])
                actions.addLog('로그가 삭제되었습니다.')
            } else {
                actions.addLog('로그 삭제에 실패했습니다.')
            }
        } catch (error) {
            console.error('로그 삭제 오류:', error)
            actions.addLog('로그 삭제 중 오류가 발생했습니다.')
        }
    }, [actions])

    // 초기 로드 및 자동 새로고침
    useEffect(() => {
        let mounted = true
        let interval = null

        // 초기 로드
        if (mounted) {
            loadLogs()
        }

        // 자동 새로고침 설정
        if (autoRefresh && mounted) {
            interval = setInterval(() => {
                if (mounted) {
                    loadLogs()
                }
            }, 10000) // 10초마다
        }

        return () => {
            mounted = false
            if (interval) {
                clearInterval(interval)
            }
        }
    }, [autoRefresh, loadLogs])

    // 디버그 로그 판단 함수
    const isDebugLog = useCallback((log) => {
        if (typeof log !== 'string') return false
        const lowerLog = log.toLowerCase()
        return lowerLog.includes('debug') ||
            lowerLog.includes('초기화') ||
            lowerLog.includes('설정') ||
            lowerLog.includes('디버그')
    }, [])

    // 로그 레벨 판단 함수
    const getLogLevel = useCallback((log) => {
        if (typeof log !== 'string') return 'info'
        const lowerLog = log.toLowerCase()
        if (lowerLog.includes('error') || lowerLog.includes('오류') || lowerLog.includes('실패')) {
            return 'error'
        } else if (lowerLog.includes('warn') || lowerLog.includes('경고')) {
            return 'warning'
        } else if (isDebugLog(log)) {
            return 'debug'
        }
        return 'info'
    }, [isDebugLog])

    // 로그 필터링
    const filteredLogs = useCallback(() => {
        if (!Array.isArray(logs)) return []

        return logs.filter(log => {
            if (typeof log !== 'string') return false
            if (!showDebug && isDebugLog(log)) return false
            if (filter && !log.toLowerCase().includes(filter.toLowerCase())) return false
            return true
        })
    }, [logs, showDebug, filter, isDebugLog])

    const displayLogs = filteredLogs()

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
                        ) : displayLogs.length === 0 ? (
                            <div className={styles.logPlaceholder}>
                                표시할 로그가 없습니다.
                                <br />
                                <small>프로그램 활동이 시작되면 여기에 로그가 표시됩니다.</small>
                            </div>
                        ) : (
                            displayLogs.map((log, index) => (
                                <div
                                    key={`log-${index}-${log?.substring?.(0, 20) || index}`}
                                    className={`${styles.logEntry} ${styles[getLogLevel(log)]}`}
                                >
                                    {log || '(빈 로그 항목)'}
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
                            onChange={(e) => setFilter(e.target.value || '')}
                            placeholder="필터링할 텍스트 입력"
                            className={styles.filterInput}
                        />
                    </div>
                </div>
            </Card>
        </div>
    )
}